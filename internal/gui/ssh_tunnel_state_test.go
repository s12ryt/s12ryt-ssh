package gui

import (
	"context"
	"net"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
)

type testSSHTunnelRuntime struct {
	closed int
	up     int64
	down   int64
}

func (runtime *testSSHTunnelRuntime) Addr() net.Addr { return nil }

func (runtime *testSSHTunnelRuntime) Traffic() (int64, int64) {
	if runtime.up == 0 && runtime.down == 0 {
		return 12, 34
	}
	return runtime.up, runtime.down
}

func (runtime *testSSHTunnelRuntime) Close() error {
	runtime.closed++
	return nil
}

func testSSHTunnelRule(id, name string) remote.SSHTunnelRule {
	return remote.SSHTunnelRule{
		ID:         id,
		Name:       name,
		HostID:     "host-1",
		Type:       remote.SSHTunnelLocal,
		ListenHost: "127.0.0.1",
		ListenPort: 18080,
		TargetHost: "127.0.0.1",
		TargetPort: 8080,
		Enabled:    true,
	}
}

func TestSSHTunnelStoreRefreshPreservesRunningRuntimeAndClosesRemoved(t *testing.T) {
	store := newSSHTunnelStore()
	firstRuntime := &testSSHTunnelRuntime{}
	removedRuntime := &testSSHTunnelRuntime{}
	store.replace([]remote.SSHTunnelRule{
		testSSHTunnelRule("tunnel-1", "first"),
		testSSHTunnelRule("tunnel-2", "removed"),
	})
	if !store.attachRuntime("tunnel-1", firstRuntime) || !store.attachRuntime("tunnel-2", removedRuntime) {
		t.Fatal("failed to attach tunnel runtimes")
	}

	updated := testSSHTunnelRule("tunnel-1", "renamed")
	updated.Version = 2
	store.replace([]remote.SSHTunnelRule{updated})

	current, ok := store.get("tunnel-1")
	if !ok || current.Rule.Name != "renamed" || current.Runtime != firstRuntime || !current.Rule.Running {
		t.Fatalf("running tunnel was not preserved: %+v", current)
	}
	if removedRuntime.closed != 1 {
		t.Fatalf("removed runtime close count = %d, want 1", removedRuntime.closed)
	}
}

func TestSSHTunnelStoreStopAndCloseAllReleaseOnlyOwnedRuntimes(t *testing.T) {
	store := newSSHTunnelStore()
	firstRuntime := &testSSHTunnelRuntime{}
	secondRuntime := &testSSHTunnelRuntime{}
	store.replace([]remote.SSHTunnelRule{
		testSSHTunnelRule("tunnel-1", "first"),
		testSSHTunnelRule("tunnel-2", "second"),
	})
	store.attachRuntime("tunnel-1", firstRuntime)
	store.attachRuntime("tunnel-2", secondRuntime)

	if !store.stop("tunnel-1") {
		t.Fatal("stop did not find the first tunnel")
	}
	if firstRuntime.closed != 1 || secondRuntime.closed != 0 {
		t.Fatalf("stop close counts = %d/%d", firstRuntime.closed, secondRuntime.closed)
	}
	first, _ := store.get("tunnel-1")
	if first.Runtime != nil || first.Rule.Running {
		t.Fatalf("stopped tunnel still has runtime: %+v", first)
	}

	store.closeAll()
	if secondRuntime.closed != 1 || firstRuntime.closed != 1 {
		t.Fatalf("close all counts = %d/%d", firstRuntime.closed, secondRuntime.closed)
	}
	if len(store.snapshot()) != 0 {
		t.Fatal("close all left tunnel state behind")
	}
}

func TestSSHTunnelRuntimeSyncThrottlesUnchangedTrafficAndRetriesFailures(t *testing.T) {
	store := newSSHTunnelStore()
	runtime := &testSSHTunnelRuntime{up: 10, down: 20}
	store.replace([]remote.SSHTunnelRule{testSSHTunnelRule("tunnel-1", "first")})
	store.attachRuntime("tunnel-1", runtime)
	now := time.Unix(100, 0)

	update, ok := store.prepareRuntimeSync("tunnel-1", now, true, true)
	if !ok || !update.Running || update.TrafficUpBytes != 10 || update.TrafficDownBytes != 20 {
		t.Fatalf("initial runtime update = %+v, ok %v", update, ok)
	}
	if _, ok := store.prepareRuntimeSync("tunnel-1", now, true, true); ok {
		t.Fatal("runtime sync accepted a duplicate request while one is in flight")
	}
	store.completeRuntimeSync("tunnel-1", update, now, false)
	if _, ok := store.prepareRuntimeSync("tunnel-1", now.Add(time.Second), true, true); !ok {
		t.Fatal("failed runtime sync could not be retried")
	}
	store.completeRuntimeSync("tunnel-1", update, now.Add(time.Second), true)
	if _, ok := store.prepareRuntimeSync("tunnel-1", now.Add(2*time.Second), false, true); ok {
		t.Fatal("unchanged traffic was synchronized before the interval")
	}

	runtime.up = 30
	runtime.down = 40
	if _, ok := store.prepareRuntimeSync("tunnel-1", now.Add(4*time.Second), false, true); ok {
		t.Fatal("changed traffic was synchronized before the throttle interval")
	}
	update, ok = store.prepareRuntimeSync("tunnel-1", now.Add(6*time.Second), false, true)
	if !ok || update.TrafficUpBytes != 30 || update.TrafficDownBytes != 40 {
		t.Fatalf("throttled runtime update = %+v, ok %v", update, ok)
	}
}

func TestSSHTunnelStoreStopCapturesFinalTrafficBeforeClosingRuntime(t *testing.T) {
	store := newSSHTunnelStore()
	runtime := &testSSHTunnelRuntime{up: 55, down: 89}
	store.replace([]remote.SSHTunnelRule{testSSHTunnelRule("tunnel-1", "first")})
	store.attachRuntime("tunnel-1", runtime)

	update, ok := store.stopWithRuntimeUpdate("tunnel-1")
	if !ok || update.Running || update.TrafficUpBytes != 55 || update.TrafficDownBytes != 89 {
		t.Fatalf("final runtime update = %+v, ok %v", update, ok)
	}
	if runtime.closed != 1 {
		t.Fatalf("runtime close count = %d, want 1", runtime.closed)
	}
	entry, _ := store.get("tunnel-1")
	if entry.Runtime != nil || entry.Rule.Running || entry.Starting {
		t.Fatalf("stopped tunnel entry = %+v", entry)
	}
}

func TestSSHTunnelPresentationSourcesExposeDirectionAndRuntimeStates(t *testing.T) {
	if got := sshTunnelDirectionSource(remote.SSHTunnelLocal); got != "Local" {
		t.Fatalf("local direction source = %q", got)
	}
	if got := sshTunnelDirectionSource(remote.SSHTunnelRemote); got != "Remote" {
		t.Fatalf("remote direction source = %q", got)
	}
	if got := sshTunnelDirectionSource(remote.SSHTunnelDynamic); got != "Dynamic SOCKS" {
		t.Fatalf("dynamic direction source = %q", got)
	}
	if got := sshTunnelStatusSource(false, ""); got != "Stopped" {
		t.Fatalf("stopped status source = %q", got)
	}
	if got := sshTunnelStatusSource(true, ""); got != "Running" {
		t.Fatalf("running status source = %q", got)
	}
	if got := sshTunnelStatusSource(false, "bind failed"); got != "Failed" {
		t.Fatalf("failed status source = %q", got)
	}
}

func TestSSHTunnelStartingStateBlocksRepeatedStart(t *testing.T) {
	store := newSSHTunnelStore()
	store.replace([]remote.SSHTunnelRule{testSSHTunnelRule("tunnel-1", "web proxy")})
	if !store.setStarting("tunnel-1") {
		t.Fatal("starting state was not set")
	}
	entry, ok := store.get("tunnel-1")
	if !ok || !entry.Starting || sshTunnelActionSource(entry) != "Starting" || sshTunnelEntryStatusSource(entry) != "Starting" {
		t.Fatalf("starting entry = %+v", entry)
	}
	if store.setStarting("tunnel-1") {
		t.Fatal("starting state accepted a duplicate transition")
	}
}

func TestSSHTunnelActionSourceMatchesRuntimeState(t *testing.T) {
	stopped := sshTunnelEntry{Rule: testSSHTunnelRule("tunnel-1", "stopped")}
	if got := sshTunnelActionSource(stopped); got != "Start" {
		t.Fatalf("stopped action source = %q", got)
	}
	running := stopped
	running.Rule.Running = true
	running.Runtime = &testSSHTunnelRuntime{}
	if got := sshTunnelActionSource(running); got != "Stop" {
		t.Fatalf("running action source = %q", got)
	}
	failed := stopped
	failed.Error = "bind failed"
	if got := sshTunnelActionSource(failed); got != "Start" {
		t.Fatalf("failed action source = %q", got)
	}
}

type tunnelRemoteSession struct {
	*fakeRemoteSession
	rules []remote.SSHTunnelRule
}

func (session *tunnelRemoteSession) SSHTunnels(context.Context) ([]remote.SSHTunnelRule, error) {
	return append([]remote.SSHTunnelRule(nil), session.rules...), nil
}

type tunnelRuntimeSession struct {
	*tunnelRemoteSession
	credentials    remote.SSHHostCredentials
	runtimeUpdates chan remote.SSHTunnelRuntimeUpdate
}

func (session *tunnelRuntimeSession) SSHHostCredentials(context.Context, string) (remote.SSHHostCredentials, error) {
	return session.credentials, nil
}

func (session *tunnelRuntimeSession) UpdateSSHTunnelRuntime(_ context.Context, _ string, update remote.SSHTunnelRuntimeUpdate) (remote.SSHTunnelRule, error) {
	if session.runtimeUpdates != nil {
		session.runtimeUpdates <- update
	}
	return remote.SSHTunnelRule{Running: update.Running, TrafficUpBytes: update.TrafficUpBytes, TrafficDownBytes: update.TrafficDownBytes}, nil
}

type tunnelCRUDSession struct {
	*tunnelRemoteSession
	created chan remote.SSHTunnelInput
	updated chan remote.SSHTunnelInput
	deleted chan string
}

func (session *tunnelCRUDSession) CreateSSHTunnel(_ context.Context, input remote.SSHTunnelInput) (remote.SSHTunnelRule, error) {
	session.created <- input
	return remote.SSHTunnelRule{
		ID:         "tunnel-created",
		Name:       input.Name,
		HostID:     input.HostID,
		Type:       input.Type,
		ListenHost: input.ListenHost,
		ListenPort: input.ListenPort,
		TargetHost: input.TargetHost,
		TargetPort: input.TargetPort,
		Enabled:    input.Enabled,
		AutoStart:  input.AutoStart,
		Version:    1,
	}, nil
}

func (session *tunnelCRUDSession) UpdateSSHTunnel(_ context.Context, id string, input remote.SSHTunnelInput) (remote.SSHTunnelRule, error) {
	session.updated <- input
	return remote.SSHTunnelRule{ID: id, Name: input.Name, HostID: input.HostID, Type: input.Type, ListenHost: input.ListenHost, ListenPort: input.ListenPort, TargetHost: input.TargetHost, TargetPort: input.TargetPort, Enabled: input.Enabled, AutoStart: input.AutoStart, Version: 2}, nil
}

func (session *tunnelCRUDSession) DeleteSSHTunnel(_ context.Context, id string) error {
	session.deleted <- id
	return nil
}

func TestRefreshSSHTunnelsLoadsRulesThroughOptionalRemoteSession(t *testing.T) {
	session := &tunnelRemoteSession{
		fakeRemoteSession: &fakeRemoteSession{},
		rules: []remote.SSHTunnelRule{
			testSSHTunnelRule("tunnel-1", "web proxy"),
		},
	}
	ui := NewWindowWithPreferences(nil, "")
	ui.model.SetRemoteSession(session, true)

	if !ui.refreshSSHTunnels() {
		t.Fatal("refresh did not start")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ui.pump()
		entries := ui.sshTunnels.snapshot()
		if len(entries) == 1 && entries[0].Rule.Name == "web proxy" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("tunnel rules were not loaded: %+v", ui.sshTunnels.snapshot())
}

func TestStartSSHTunnelUsesVersionedCredentialsAndAttachesRuntime(t *testing.T) {
	rule := testSSHTunnelRule("tunnel-1", "web proxy")
	rule.ListenPort = 0
	session := &tunnelRuntimeSession{
		tunnelRemoteSession: &tunnelRemoteSession{fakeRemoteSession: &fakeRemoteSession{}},
		credentials: remote.SSHHostCredentials{
			ID:       "host-1",
			Version:  7,
			Host:     "web.example.com",
			Port:     22,
			Username: "deploy",
		},
	}
	transport := &testForwardTransport{testSSHTransport: &testSSHTransport{}}
	ui := NewWindowWithPreferences(nil, "")
	ui.model.SetRemoteSession(session, true)
	ui.sshTunnels.replace([]remote.SSHTunnelRule{rule})
	ui.sshTransportFactory = func(credentials remote.SSHHostCredentials) (sshTransport, error) {
		if credentials.ID != "host-1" || credentials.Version != 7 {
			t.Fatalf("factory credentials = %+v", credentials)
		}
		return transport, nil
	}

	if !ui.startSSHTunnel(rule.ID) {
		t.Fatal("tunnel start did not begin")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ui.pump()
		entry, ok := ui.sshTunnels.get(rule.ID)
		if ok && entry.Runtime != nil {
			if !entry.Rule.Running || entry.Runtime.Addr() == nil {
				t.Fatalf("running entry = %+v", entry)
			}
			ui.sshTunnels.closeAll()
			if transport.closed != 1 {
				t.Fatalf("transport close count = %d, want 1", transport.closed)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("tunnel runtime was not attached: %+v", ui.sshTunnels.snapshot())
}

func TestStartAndStopSSHTunnelSynchronizesRemoteRuntimeState(t *testing.T) {
	rule := testSSHTunnelRule("tunnel-1", "web proxy")
	rule.ListenPort = 0
	session := &tunnelRuntimeSession{
		tunnelRemoteSession: &tunnelRemoteSession{fakeRemoteSession: &fakeRemoteSession{}},
		credentials:         remote.SSHHostCredentials{ID: "host-1", Version: 7, Host: "web.example.com", Port: 22, Username: "deploy"},
		runtimeUpdates:      make(chan remote.SSHTunnelRuntimeUpdate, 4),
	}
	transport := &testForwardTransport{testSSHTransport: &testSSHTransport{}}
	ui := NewWindowWithPreferences(nil, "")
	ui.model.SetRemoteSession(session, true)
	ui.sshTunnels.replace([]remote.SSHTunnelRule{rule})
	ui.sshTransportFactory = func(remote.SSHHostCredentials) (sshTransport, error) { return transport, nil }

	if !ui.startSSHTunnel(rule.ID) {
		t.Fatal("tunnel start did not begin")
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		ui.pump()
		select {
		case update := <-session.runtimeUpdates:
			if !update.Running {
				t.Fatalf("start runtime update = %+v", update)
			}
			if !ui.stopSSHTunnel(rule.ID) {
				t.Fatal("tunnel stop did not begin")
			}
			select {
			case stopped := <-session.runtimeUpdates:
				if stopped.Running {
					t.Fatalf("stop runtime update = %+v", stopped)
				}
				return
			case <-time.After(time.Second):
				t.Fatal("stopped runtime was not synchronized")
			}
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("running runtime was not synchronized")
}

func TestRefreshSSHTunnelsStartsEnabledAutoStartRules(t *testing.T) {
	rule := testSSHTunnelRule("tunnel-1", "auto proxy")
	rule.ListenPort = 0
	rule.AutoStart = true
	session := &tunnelRuntimeSession{
		tunnelRemoteSession: &tunnelRemoteSession{
			fakeRemoteSession: &fakeRemoteSession{},
			rules:             []remote.SSHTunnelRule{rule},
		},
		credentials: remote.SSHHostCredentials{ID: "host-1", Version: 7, Host: "web.example.com", Port: 22, Username: "deploy"},
	}
	transport := &testForwardTransport{testSSHTransport: &testSSHTransport{}}
	ui := NewWindowWithPreferences(nil, "")
	ui.model.SetRemoteSession(session, true)
	ui.sshTransportFactory = func(remote.SSHHostCredentials) (sshTransport, error) { return transport, nil }

	if !ui.refreshSSHTunnels() {
		t.Fatal("refresh did not start")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ui.pump()
		entry, ok := ui.sshTunnels.get(rule.ID)
		if ok && entry.Runtime != nil {
			if !entry.Rule.Running {
				t.Fatal("auto-started tunnel is not running")
			}
			ui.sshTunnels.closeAll()
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("auto-start tunnel was not started: %+v", ui.sshTunnels.snapshot())
}

func TestSSHTunnelFormOpensWithDefaultsAndExistingRuleValues(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshHosts = []remote.SSHHost{{ID: "host-1", Name: "Remote"}}
	ui.sshTunnels.replace([]remote.SSHTunnelRule{testSSHTunnelRule("tunnel-1", "web proxy")})

	if !ui.openSSHTunnelForm("") {
		t.Fatal("new tunnel form did not open")
	}
	if ui.sshTunnelFormID != "" || ui.sshTunnelForm.HostID != "host-1" || ui.sshTunnelForm.Type != remote.SSHTunnelLocal || !ui.sshTunnelForm.Enabled {
		t.Fatalf("new tunnel defaults = %+v", ui.sshTunnelForm)
	}
	ui.closeSSHTunnelForm()

	if !ui.openSSHTunnelForm("tunnel-1") {
		t.Fatal("edit tunnel form did not open")
	}
	if ui.sshTunnelFormID != "tunnel-1" || ui.sshTunnelForm.Name != "web proxy" || ui.sshTunnelForm.ListenPort != 18080 {
		t.Fatalf("existing tunnel values = %+v", ui.sshTunnelForm)
	}
}

func TestSubmitSSHTunnelFormCreatesRuleAndUpdatesStore(t *testing.T) {
	session := &tunnelCRUDSession{
		tunnelRemoteSession: &tunnelRemoteSession{fakeRemoteSession: &fakeRemoteSession{}},
		created:             make(chan remote.SSHTunnelInput, 1),
		updated:             make(chan remote.SSHTunnelInput, 1),
		deleted:             make(chan string, 1),
	}
	ui := NewWindow(nil)
	ui.model.SetRemoteSession(session, true)
	ui.sshHosts = []remote.SSHHost{{ID: "host-1", Name: "Remote"}}
	if !ui.openSSHTunnelForm("") {
		t.Fatal("new tunnel form did not open")
	}
	ui.sshTunnelName.SetText("web proxy")
	ui.sshTunnelHost.SetText("host-1")
	ui.sshTunnelListenHost.SetText("127.0.0.1")
	ui.sshTunnelListenPort.SetText("18080")
	ui.sshTunnelTargetHost.SetText("127.0.0.1")
	ui.sshTunnelTargetPort.SetText("8080")

	if !ui.submitSSHTunnelForm() {
		t.Fatal("tunnel form submit did not start")
	}
	select {
	case input := <-session.created:
		if input.Name != "web proxy" || input.HostID != "host-1" || input.ListenPort != 18080 || input.TargetPort != 8080 {
			t.Fatalf("created tunnel input = %+v", input)
		}
	case <-time.After(time.Second):
		t.Fatal("create request was not dispatched")
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		ui.pump()
		if _, ok := ui.sshTunnels.get("tunnel-created"); ok {
			if ui.sshTunnelFormOpen {
				t.Fatal("successful submit left the modal open")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("created tunnel was not stored: %+v", ui.sshTunnels.snapshot())
}

func TestDeleteSSHTunnelRequiresConfirmationBeforeRemoteMutation(t *testing.T) {
	session := &tunnelCRUDSession{
		tunnelRemoteSession: &tunnelRemoteSession{fakeRemoteSession: &fakeRemoteSession{}},
		created:             make(chan remote.SSHTunnelInput, 1),
		updated:             make(chan remote.SSHTunnelInput, 1),
		deleted:             make(chan string, 1),
	}
	ui := NewWindow(nil)
	ui.model.SetRemoteSession(session, true)
	ui.sshTunnels.replace([]remote.SSHTunnelRule{testSSHTunnelRule("tunnel-1", "web proxy")})
	runtime := &testSSHTunnelRuntime{}
	ui.sshTunnels.attachRuntime("tunnel-1", runtime)

	if !ui.deleteSSHTunnel("tunnel-1") {
		t.Fatal("delete did not request confirmation")
	}
	if !ui.confirm.active {
		t.Fatal("delete confirmation did not open")
	}
	select {
	case id := <-session.deleted:
		t.Fatalf("delete API called before confirmation for %q", id)
	default:
	}

	ui.confirm.accept()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ui.pump()
		select {
		case id := <-session.deleted:
			if id != "tunnel-1" {
				t.Fatalf("deleted tunnel id = %q", id)
			}
			if runtime.closed != 1 || len(ui.sshTunnels.snapshot()) != 0 {
				t.Fatalf("delete cleanup = runtime %d, rules %+v", runtime.closed, ui.sshTunnels.snapshot())
			}
			return
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("delete API was not called after confirmation")
}

func TestSSHTunnelInputValidationRequiresTypeSpecificEndpoints(t *testing.T) {
	base := remote.SSHTunnelInput{
		Name:       "web proxy",
		HostID:     "host-1",
		Type:       remote.SSHTunnelLocal,
		ListenHost: "127.0.0.1",
		ListenPort: 18080,
		TargetHost: "127.0.0.1",
		TargetPort: 8080,
		Enabled:    true,
	}
	tests := []struct {
		name  string
		input remote.SSHTunnelInput
		want  string
	}{
		{name: "name", input: func() remote.SSHTunnelInput { input := base; input.Name = ""; return input }(), want: "Tunnel name is required."},
		{name: "host", input: func() remote.SSHTunnelInput { input := base; input.HostID = ""; return input }(), want: "Tunnel host is required."},
		{name: "listen port", input: func() remote.SSHTunnelInput { input := base; input.ListenPort = 0; return input }(), want: "Listen port must be between 1 and 65535."},
		{name: "target port", input: func() remote.SSHTunnelInput { input := base; input.TargetPort = 0; return input }(), want: "Target port must be between 1 and 65535."},
		{name: "dynamic target optional", input: func() remote.SSHTunnelInput {
			input := base
			input.Type = remote.SSHTunnelDynamic
			input.TargetHost = ""
			input.TargetPort = 0
			return input
		}(), want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validateSSHTunnelInput(test.input); got != test.want {
				t.Fatalf("validation error = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSSHTunnelFormValuesRoundTripExistingRule(t *testing.T) {
	rule := testSSHTunnelRule("tunnel-1", "web proxy")
	rule.Type = remote.SSHTunnelRemote
	rule.ListenHost = "0.0.0.0"
	rule.ListenPort = 19090
	rule.TargetHost = "db.internal"
	rule.TargetPort = 5432
	rule.Enabled = false
	rule.AutoStart = true

	values := sshTunnelFormFromRule(rule)
	if values.ID != rule.ID || values.Name != rule.Name || values.HostID != rule.HostID || values.Type != rule.Type {
		t.Fatalf("form identity = %+v, rule = %+v", values, rule)
	}
	if values.ListenHost != rule.ListenHost || values.ListenPort != rule.ListenPort || values.TargetHost != rule.TargetHost || values.TargetPort != rule.TargetPort {
		t.Fatalf("form endpoints = %+v, rule = %+v", values, rule)
	}
	if values.Enabled != rule.Enabled || values.AutoStart != rule.AutoStart {
		t.Fatalf("form flags = %+v, rule = %+v", values, rule)
	}
	input := values.input()
	if input.Name != rule.Name || input.Type != rule.Type || input.ListenPort != rule.ListenPort || input.TargetPort != rule.TargetPort || input.Enabled != rule.Enabled || input.AutoStart != rule.AutoStart {
		t.Fatalf("round-trip input = %+v, rule = %+v", input, rule)
	}
}
