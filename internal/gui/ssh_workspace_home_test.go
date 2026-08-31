package gui

import (
	"strings"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
)

func TestSSHWorkspaceNavigationMatchesReferenceModules(t *testing.T) {
	items := sshWorkspaceNavigation()
	want := []sshWorkspaceModule{
		sshWorkspaceHosts,
		sshWorkspaceTunnels,
		sshWorkspaceSnippets,
		sshWorkspaceKeys,
		sshWorkspaceFingerprints,
		sshWorkspaceHistory,
	}
	if len(items) != len(want) {
		t.Fatalf("navigation count = %d, want %d", len(items), len(want))
	}
	for i, item := range items {
		if item.ID != want[i] {
			t.Fatalf("navigation[%d] = %q, want %q", i, item.ID, want[i])
		}
		if item.LabelSource == "" {
			t.Fatalf("navigation[%d] has no translation source", i)
		}
	}
}

func TestFilterSSHHostsMatchesNameAddressGroupAndTags(t *testing.T) {
	hosts := []remote.SSHHost{
		{ID: "one", Name: "Production web", Host: "web.example.com", GroupPath: "vps/prod", Tags: []string{"critical", "nginx"}},
		{ID: "two", Name: "Staging", Host: "staging.example.com", GroupPath: "vps/staging", Tags: []string{"preview"}},
		{ID: "three", Name: "Local", Host: "127.0.0.1", GroupPath: "local", Tags: []string{"dev"}},
	}
	for _, testCase := range []struct {
		query string
		want  []string
	}{
		{query: "PRODUCTION", want: []string{"one"}},
		{query: "staging.example", want: []string{"two"}},
		{query: "vps/prod", want: []string{"one"}},
		{query: "NGINX", want: []string{"one"}},
		{query: "", want: []string{"one", "two", "three"}},
	} {
		filtered := filterSSHHosts(hosts, testCase.query)
		if len(filtered) != len(testCase.want) {
			t.Fatalf("query %q returned %d hosts, want %d", testCase.query, len(filtered), len(testCase.want))
		}
		for i, host := range filtered {
			if host.ID != testCase.want[i] {
				t.Fatalf("query %q result[%d] = %q, want %q", testCase.query, i, host.ID, testCase.want[i])
			}
		}
	}
}

func TestGroupSSHHostsBuildsNestedGroupSummaries(t *testing.T) {
	hosts := []remote.SSHHost{
		{ID: "one", Name: "web", GroupPath: "vps/prod"},
		{ID: "two", Name: "db", GroupPath: "vps/prod"},
		{ID: "three", Name: "local", GroupPath: "local"},
		{ID: "four", Name: "ungrouped"},
	}
	groups := groupSSHHosts(hosts)
	if len(groups) != 3 {
		t.Fatalf("groups = %+v, want 3 groups", groups)
	}
	if groups[0].Path != "local" || groups[0].Count != 1 {
		t.Fatalf("first group = %+v", groups[0])
	}
	if groups[1].Path != "vps/prod" || groups[1].Count != 2 {
		t.Fatalf("nested group = %+v", groups[1])
	}
	if groups[2].Path != "" || groups[2].Count != 1 {
		t.Fatalf("ungrouped group = %+v", groups[2])
	}
}

func TestRecentSSHHostsSortsByUpdatedAtAndHonorsLimit(t *testing.T) {
	hosts := []remote.SSHHost{
		{ID: "old", UpdatedAt: 10},
		{ID: "new", UpdatedAt: 30},
		{ID: "middle", UpdatedAt: 20},
	}

	recent := recentSSHHosts(hosts, 2)
	if len(recent) != 2 || recent[0].ID != "new" || recent[1].ID != "middle" {
		t.Fatalf("recent hosts = %+v, want new then middle", recent)
	}
	if hosts[0].ID != "old" || hosts[1].ID != "new" {
		t.Fatal("recent host sorting must not mutate the input slice")
	}
}

func TestSSHWorkspaceModuleTitleUsesNavigationSource(t *testing.T) {
	for _, item := range sshWorkspaceNavigation() {
		if got := sshWorkspaceModuleTitle(item.ID); got != item.LabelSource {
			t.Fatalf("module %q title = %q, want %q", item.ID, got, item.LabelSource)
		}
	}
}

func TestLocalTerminalCommandUsesAvailableWindowsShell(t *testing.T) {
	name, args := localShellCommand()
	if name == "" {
		t.Fatal("local terminal shell command is empty")
	}
	if len(args) == 0 {
		t.Fatal("local terminal shell arguments are empty")
	}
}

func TestLocalShellSessionAcceptsCommandsAndReturnsOutput(t *testing.T) {
	terminal, err := startLocalShell("cmd.exe", []string{"/d"})
	if err != nil {
		t.Fatalf("start local shell: %v", err)
	}
	defer terminal.Close()

	const marker = "s12ryt-local-terminal"
	if _, err := terminal.Write([]byte("echo " + marker + "\r\n")); err != nil {
		t.Fatalf("write local shell command: %v", err)
	}

	output := make(chan string, 1)
	go func() {
		var text strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := terminal.Read(buf)
			if n > 0 {
				text.Write(buf[:n])
				if strings.Contains(text.String(), marker) {
					output <- text.String()
					return
				}
			}
			if err != nil {
				output <- text.String()
				return
			}
		}
	}()
	select {
	case text := <-output:
		if !strings.Contains(text, marker) {
			t.Fatalf("local shell output = %q, want marker %q", text, marker)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("local shell did not return command output")
	}
}

func TestSSHWorkspaceModuleSelectionAcceptsOnlyNavigationModules(t *testing.T) {
	ui := NewWindow(nil)
	for _, item := range sshWorkspaceNavigation() {
		if !ui.setSSHWorkspaceModule(item.ID) {
			t.Fatalf("navigation module %q was rejected", item.ID)
		}
		if ui.workspaceModule != item.ID {
			t.Fatalf("workspace module = %q, want %q", ui.workspaceModule, item.ID)
		}
	}
	previous := ui.workspaceModule
	if ui.setSSHWorkspaceModule(sshWorkspaceModule(255)) {
		t.Fatal("unknown workspace module was accepted")
	}
	if ui.workspaceModule != previous {
		t.Fatalf("unknown module changed selection to %q", ui.workspaceModule)
	}
}

func TestSSHWorkspaceCanOpenLocalTerminalTab(t *testing.T) {
	ui := NewWindow(nil)
	tab := ui.openLocalTerminalTab()
	if tab == nil {
		t.Fatal("local terminal tab was not created")
	}
	if !tab.Local || tab.HostName != "Local terminal" || tab.State != sshTabConnecting {
		t.Fatalf("local terminal tab = %+v", tab)
	}
	if ui.sshTabs.activeID != tab.ID || len(ui.sshTabs.tabs) != 1 {
		t.Fatalf("local tab selection = active %q tabs %d", ui.sshTabs.activeID, len(ui.sshTabs.tabs))
	}
}

func TestAttachSSHTabReleasesTerminalWhenTabWasClosed(t *testing.T) {
	ui := NewWindow(nil)
	terminal := &testSSHCloser{}

	ui.attachSSHTab("closed-tab", nil, terminal)

	if !terminal.closed {
		t.Fatal("terminal must be released when its tab no longer exists")
	}
}
