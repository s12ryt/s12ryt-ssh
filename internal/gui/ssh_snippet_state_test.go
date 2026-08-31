package gui

import (
	"context"
	"strings"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"

	"gioui.org/widget"
)

type snippetRemoteSession struct {
	fakeRemoteSession
	snippets []remote.SSHCommandSnippet
	secrets  map[string]string
}

type snippetCRUDRemoteSession struct {
	snippetRemoteSession
	createdInput remote.SSHCommandSnippetInput
	updatedID    string
	updatedInput remote.SSHCommandSnippetInput
	deletedID    string
	created      remote.SSHCommandSnippet
	updated      remote.SSHCommandSnippet
}

func (s *snippetCRUDRemoteSession) CreateSSHCommandSnippet(_ context.Context, input remote.SSHCommandSnippetInput) (remote.SSHCommandSnippet, error) {
	s.createdInput = input
	if s.created.ID == "" {
		s.created = remote.SSHCommandSnippet{ID: "snippet-created", Name: input.Name, Command: input.Command, Variables: append([]string(nil), input.Variables...), Enabled: input.Enabled == nil || *input.Enabled, Version: 1}
	}
	return s.created, nil
}

func (s *snippetCRUDRemoteSession) UpdateSSHCommandSnippet(_ context.Context, id string, input remote.SSHCommandSnippetInput) (remote.SSHCommandSnippet, error) {
	s.updatedID = id
	s.updatedInput = input
	s.updated = remote.SSHCommandSnippet{ID: id, Name: input.Name, Command: input.Command, Variables: append([]string(nil), input.Variables...), Enabled: input.Enabled == nil || *input.Enabled, Version: 2}
	return s.updated, nil
}

func (s *snippetCRUDRemoteSession) DeleteSSHCommandSnippet(_ context.Context, id string) error {
	s.deletedID = id
	return nil
}

func (s *snippetRemoteSession) SSHCommandSnippets(context.Context) ([]remote.SSHCommandSnippet, error) {
	return append([]remote.SSHCommandSnippet(nil), s.snippets...), nil
}

func (s *snippetRemoteSession) SSHCommandSnippetSecrets(context.Context, string) (map[string]string, error) {
	result := make(map[string]string, len(s.secrets))
	for name, value := range s.secrets {
		result[name] = value
	}
	return result, nil
}

func TestFilterSSHCommandSnippetsMatchesNameCommandAndVariables(t *testing.T) {
	snippets := []remote.SSHCommandSnippet{
		{ID: "one", Name: "Deploy production", Command: "kubectl rollout", Variables: []string{"SERVICE"}},
		{ID: "two", Name: "Tail logs", Command: "journalctl -fu api", Variables: []string{"LINES"}},
		{ID: "three", Name: "Database backup", Command: "pg_dump", Variables: []string{"DATABASE"}},
	}
	for _, testCase := range []struct {
		query string
		want  []string
	}{
		{query: "PRODUCTION", want: []string{"one"}},
		{query: "JOURNALCTL", want: []string{"two"}},
		{query: "database", want: []string{"three"}},
		{query: "service", want: []string{"one"}},
		{query: "", want: []string{"one", "two", "three"}},
	} {
		filtered := filterSSHCommandSnippets(snippets, testCase.query)
		if len(filtered) != len(testCase.want) {
			t.Fatalf("query %q returned %d snippets, want %d", testCase.query, len(filtered), len(testCase.want))
		}
		for index, snippet := range filtered {
			if snippet.ID != testCase.want[index] {
				t.Fatalf("query %q result[%d] = %q, want %q", testCase.query, index, snippet.ID, testCase.want[index])
			}
		}
	}
}

func TestExpandSSHCommandSnippetSubstitutesVariablesAndSecrets(t *testing.T) {
	snippet := remote.SSHCommandSnippet{
		Name:        "deploy",
		Command:     `deploy --service "${SERVICE}" --token "${TOKEN}"`,
		Variables:   []string{"SERVICE"},
		SecretNames: []string{"TOKEN"},
	}
	command, err := expandSSHCommandSnippet(snippet, map[string]string{"SERVICE": "web"}, map[string]string{"TOKEN": "secret"})
	if err != nil {
		t.Fatalf("expand snippet: %v", err)
	}
	if command != `deploy --service "web" --token "secret"` {
		t.Fatalf("expanded command = %q", command)
	}

	if _, err := expandSSHCommandSnippet(snippet, nil, map[string]string{"TOKEN": "secret"}); err == nil || !strings.Contains(err.Error(), "SERVICE") {
		t.Fatalf("missing variable error = %v", err)
	}
	unknown := snippet
	unknown.Command += " ${UNDECLARED}"
	if _, err := expandSSHCommandSnippet(unknown, map[string]string{"SERVICE": "web"}, map[string]string{"TOKEN": "secret"}); err == nil || !strings.Contains(err.Error(), "UNDECLARED") {
		t.Fatalf("unknown variable error = %v", err)
	}
}

func TestSSHCommandSnippetStoreRefreshCopiesMetadataAndClearsRemoved(t *testing.T) {
	store := newSSHCommandSnippetStore()
	store.replace([]remote.SSHCommandSnippet{
		{ID: "one", Name: "Deploy", Variables: []string{"SERVICE"}, SecretNames: []string{"TOKEN"}, Enabled: true, Version: 2},
		{ID: "two", Name: "Logs", Enabled: false, Version: 1},
	})

	snapshot := store.snapshot()
	if len(snapshot) != 2 || snapshot[0].Snippet.ID != "one" || snapshot[1].Snippet.ID != "two" {
		t.Fatalf("initial snippets = %+v", snapshot)
	}
	snapshot[0].Snippet.Variables[0] = "MUTATED"
	snapshot[0].Snippet.SecretNames[0] = "MUTATED"
	store.replace([]remote.SSHCommandSnippet{{ID: "one", Name: "Deploy updated", Variables: []string{"SERVICE"}, SecretNames: []string{"TOKEN"}, Enabled: true, Version: 3}})

	snapshot = store.snapshot()
	if len(snapshot) != 1 || snapshot[0].Snippet.Name != "Deploy updated" || snapshot[0].Snippet.Version != 3 {
		t.Fatalf("refreshed snippets = %+v", snapshot)
	}
	if snapshot[0].Snippet.Variables[0] != "SERVICE" || snapshot[0].Snippet.SecretNames[0] != "TOKEN" {
		t.Fatalf("snippet slices were not copied = %+v", snapshot[0])
	}
}

func TestRefreshSSHCommandSnippetsLoadsRemoteMetadataWithoutSecrets(t *testing.T) {
	ui := NewWindow(nil)
	session := &snippetRemoteSession{
		snippets: []remote.SSHCommandSnippet{{
			ID: "snippet-1", Name: "Deploy", Command: "deploy ${SERVICE}",
			Variables: []string{"SERVICE"}, SecretNames: []string{"TOKEN"}, Enabled: true, Version: 3,
		}},
		secrets: map[string]string{"TOKEN": "must-not-be-listed"},
	}
	ui.model.SetRemoteSession(session, true)
	if !ui.refreshSSHCommandSnippets() {
		t.Fatal("refresh snippets was not started")
	}
	if !waitForSnippetUIEvent(ui, 2*time.Second) {
		t.Fatal("snippet refresh did not complete")
	}
	entries := ui.sshSnippets.snapshot()
	if len(entries) != 1 || entries[0].Snippet.ID != "snippet-1" {
		t.Fatalf("snippet entries = %+v", entries)
	}
	if strings.Contains(entries[0].Snippet.Command, "must-not-be-listed") {
		t.Fatal("snippet metadata exposed a secret")
	}
}

func TestExecuteSSHCommandSnippetExpandsSecretsAndWritesOnlyActiveTerminal(t *testing.T) {
	ui := NewWindow(nil)
	session := &snippetRemoteSession{
		secrets: map[string]string{"TOKEN": "secret-token"},
	}
	ui.model.SetRemoteSession(session, true)
	ui.sshSnippets.replace([]remote.SSHCommandSnippet{{
		ID: "snippet-1", Name: "Deploy", Command: "deploy ${SERVICE} ${TOKEN}",
		Variables: []string{"SERVICE"}, SecretNames: []string{"TOKEN"}, Enabled: true,
	}})
	ui.sshSnippetVariableNames = []string{"SERVICE"}
	first := ui.sshTabs.open(testSSHHost("host-1", "web"))
	second := ui.sshTabs.open(testSSHHost("host-2", "db"))
	firstPTY := &testSSHWrites{}
	secondPTY := &testSSHWrites{}
	first.session = &sshTabSession{pty: firstPTY}
	second.session = &sshTabSession{pty: secondPTY}
	first.State = sshTabConnected
	second.State = sshTabConnected
	ui.sshTabs.activate(second.ID)
	ui.sshSnippetVariableEditors = []widget.Editor{{}}
	ui.sshSnippetVariableEditors[0].SetText("api")
	if !ui.executeSSHCommandSnippet("snippet-1") {
		t.Fatal("snippet execution was not started")
	}
	if !waitForSnippetUIEvent(ui, 2*time.Second) {
		t.Fatal("snippet execution did not complete")
	}
	if strings.Join(secondPTY.writes, "") != "deploy api secret-token\r" {
		t.Fatalf("active terminal writes = %q", secondPTY.writes)
	}
	if len(firstPTY.writes) != 0 {
		t.Fatalf("inactive terminal received writes = %q", firstPTY.writes)
	}
}

func waitForSnippetUIEvent(ui *Window, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ui.pump()
		if !ui.busy {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func TestOpenSSHCommandSnippetExecutionCreatesVariableInputs(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshSnippets.replace([]remote.SSHCommandSnippet{
		{ID: "snippet-1", Name: "Deploy", Command: "deploy ${SERVICE}", Variables: []string{"SERVICE"}, Enabled: true},
	})
	if !ui.openSSHCommandSnippetExecution("snippet-1") {
		t.Fatal("snippet execution form was not opened")
	}
	if !ui.sshSnippetExecutionOpen || ui.sshSnippetExecutionID != "snippet-1" {
		t.Fatalf("snippet execution state = open:%v id:%q", ui.sshSnippetExecutionOpen, ui.sshSnippetExecutionID)
	}
	if len(ui.sshSnippetVariableNames) != 1 || ui.sshSnippetVariableNames[0] != "SERVICE" || len(ui.sshSnippetVariableEditors) != 1 {
		t.Fatalf("snippet variable inputs = names:%v editors:%d", ui.sshSnippetVariableNames, len(ui.sshSnippetVariableEditors))
	}
	ui.closeSSHCommandSnippetExecution()
	if ui.sshSnippetExecutionOpen || ui.sshSnippetExecutionID != "" || len(ui.sshSnippetVariableEditors) != 0 {
		t.Fatalf("snippet execution state after close = open:%v id:%q editors:%d", ui.sshSnippetExecutionOpen, ui.sshSnippetExecutionID, len(ui.sshSnippetVariableEditors))
	}
}

func TestSSHCommandSnippetFormBuildsInputAndPreservesExistingSecrets(t *testing.T) {
	values := sshCommandSnippetFormValues{
		Name:             "Deploy",
		Command:          "deploy ${SERVICE} ${TOKEN}",
		VariablesText:    "SERVICE, ENVIRONMENT",
		SecretValuesText: "TOKEN=secret-token",
		Enabled:          false,
	}
	input, err := values.input()
	if err != nil {
		t.Fatalf("build snippet input: %v", err)
	}
	if input.Name != "Deploy" || input.Command != "deploy ${SERVICE} ${TOKEN}" {
		t.Fatalf("snippet input identity = %+v", input)
	}
	if strings.Join(input.Variables, ",") != "SERVICE,ENVIRONMENT" || input.Secrets["TOKEN"] != "secret-token" {
		t.Fatalf("snippet input fields = %+v", input)
	}
	if input.Enabled == nil || *input.Enabled {
		t.Fatalf("enabled pointer = %v", input.Enabled)
	}

	values.SecretValuesText = ""
	input, err = values.input()
	if err != nil {
		t.Fatalf("build preserve-secrets input: %v", err)
	}
	if input.Secrets != nil {
		t.Fatalf("empty secret editor must preserve existing secrets, got %#v", input.Secrets)
	}

	values.ClearSecrets = true
	input, err = values.input()
	if err != nil {
		t.Fatalf("build clear-secrets input: %v", err)
	}
	if input.Secrets == nil || len(input.Secrets) != 0 {
		t.Fatalf("clear secrets payload = %#v", input.Secrets)
	}
}

func TestSSHCommandSnippetFormRejectsMalformedSecretEntries(t *testing.T) {
	for _, text := range []string{"TOKEN", "TOKEN=one\nTOKEN=two", "=missing-name", "TOKEN="} {
		values := sshCommandSnippetFormValues{
			Name:             "Deploy",
			Command:          "deploy",
			SecretValuesText: text,
			Enabled:          true,
		}
		if _, err := values.input(); err == nil {
			t.Fatalf("secret text %q was accepted", text)
		}
	}
}

func TestOpenSSHCommandSnippetFormLoadsMetadataWithoutSecretValues(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshSnippets.replace([]remote.SSHCommandSnippet{{
		ID: "snippet-1", Name: "Deploy", Command: "deploy ${SERVICE} ${TOKEN}",
		Variables: []string{"SERVICE"}, SecretNames: []string{"TOKEN"}, Enabled: true,
	}})
	if !ui.openSSHCommandSnippetForm("snippet-1") {
		t.Fatal("snippet edit form was not opened")
	}
	if !ui.sshSnippetFormOpen || ui.sshSnippetFormID != "snippet-1" {
		t.Fatalf("snippet form state = open:%v id:%q", ui.sshSnippetFormOpen, ui.sshSnippetFormID)
	}
	if ui.sshSnippetName.Text() != "Deploy" || ui.sshSnippetCommand.Text() != "deploy ${SERVICE} ${TOKEN}" {
		t.Fatalf("snippet form metadata = name:%q command:%q", ui.sshSnippetName.Text(), ui.sshSnippetCommand.Text())
	}
	if ui.sshSnippetSecrets.Text() != "" {
		t.Fatal("existing secret values must not be loaded into the edit form")
	}
	ui.closeSSHCommandSnippetForm()
	if ui.sshSnippetFormOpen || ui.sshSnippetFormID != "" {
		t.Fatalf("snippet form state after close = open:%v id:%q", ui.sshSnippetFormOpen, ui.sshSnippetFormID)
	}
}

func TestSubmitSSHCommandSnippetFormUsesRemoteCreateAndUpdatePayloads(t *testing.T) {
	ui := NewWindow(nil)
	session := &snippetCRUDRemoteSession{}
	ui.model.SetRemoteSession(session, true)
	if !ui.openSSHCommandSnippetForm("") {
		t.Fatal("new snippet form was not opened")
	}
	ui.sshSnippetName.SetText("Deploy")
	ui.sshSnippetCommand.SetText("deploy ${SERVICE}")
	ui.sshSnippetVariables.SetText("SERVICE")
	ui.sshSnippetSecrets.SetText("TOKEN=secret")
	ui.sshSnippetEnabled.Value = false
	if !ui.submitSSHCommandSnippetForm() {
		t.Fatal("snippet create was not started")
	}
	if !waitForSnippetUIEvent(ui, 2*time.Second) {
		t.Fatal("snippet create did not complete")
	}
	if session.createdInput.Name != "Deploy" || session.createdInput.Secrets["TOKEN"] != "secret" {
		t.Fatalf("create payload = %+v", session.createdInput)
	}
	if session.createdInput.Enabled == nil || *session.createdInput.Enabled {
		t.Fatalf("create enabled payload = %v", session.createdInput.Enabled)
	}
	if ui.sshSnippetFormOpen {
		t.Fatal("successful create left the form open")
	}

	ui.sshSnippets.replace([]remote.SSHCommandSnippet{{
		ID: "snippet-1", Name: "Deploy", Command: "deploy ${SERVICE}", Variables: []string{"SERVICE"},
		SecretNames: []string{"TOKEN"}, Enabled: true, Version: 1,
	}})
	if !ui.openSSHCommandSnippetForm("snippet-1") {
		t.Fatal("edit snippet form was not opened")
	}
	ui.sshSnippetName.SetText("Deploy updated")
	ui.sshSnippetCommand.SetText("deploy ${SERVICE} --safe")
	ui.sshSnippetVariables.SetText("SERVICE")
	ui.sshSnippetSecrets.SetText("")
	ui.sshSnippetEnabled.Value = true
	if !ui.submitSSHCommandSnippetForm() {
		t.Fatal("snippet update was not started")
	}
	if !waitForSnippetUIEvent(ui, 2*time.Second) {
		t.Fatal("snippet update did not complete")
	}
	if session.updatedID != "snippet-1" || session.updatedInput.Command != "deploy ${SERVICE} --safe" {
		t.Fatalf("update payload = id:%q input:%+v", session.updatedID, session.updatedInput)
	}
	if session.updatedInput.Secrets != nil {
		t.Fatalf("blank secret editor must preserve secrets, got %#v", session.updatedInput.Secrets)
	}
}

func TestDeleteSSHCommandSnippetRequiresConfirmationBeforeRemoteCall(t *testing.T) {
	ui := NewWindow(nil)
	session := &snippetCRUDRemoteSession{}
	ui.model.SetRemoteSession(session, true)
	ui.sshSnippets.replace([]remote.SSHCommandSnippet{{ID: "snippet-1", Name: "Deploy", Command: "deploy", Enabled: true}})
	if !ui.deleteSSHCommandSnippet("snippet-1") {
		t.Fatal("snippet delete confirmation was not opened")
	}
	if !ui.confirm.active || session.deletedID != "" {
		t.Fatalf("delete state = confirm:%v deleted:%q", ui.confirm.active, session.deletedID)
	}
	ui.confirm.accept()
	if !waitForSnippetUIEvent(ui, 2*time.Second) {
		t.Fatal("snippet delete did not complete")
	}
	if session.deletedID != "snippet-1" {
		t.Fatalf("deleted snippet id = %q", session.deletedID)
	}
	if len(ui.sshSnippets.snapshot()) != 0 {
		t.Fatalf("snippet store after delete = %+v", ui.sshSnippets.snapshot())
	}
}

func TestSSHCommandSnippetManagementSourcesExposeCreateEditDelete(t *testing.T) {
	want := []string{"New snippet", "Edit snippet", "Delete snippet?"}
	got := sshCommandSnippetManagementSources()
	if len(got) != len(want) {
		t.Fatalf("management sources = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("management source[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestSyncSSHCommandSnippetButtonsFollowsVisibleSnippetIDs(t *testing.T) {
	ui := NewWindow(nil)
	ui.syncSSHCommandSnippetButtons([]sshCommandSnippetEntry{
		{Snippet: remote.SSHCommandSnippet{ID: "one"}},
		{Snippet: remote.SSHCommandSnippet{ID: "two"}},
	})
	if strings.Join(ui.sshSnippetVisibleIDs, ",") != "one,two" {
		t.Fatalf("visible snippet ids = %v", ui.sshSnippetVisibleIDs)
	}
	if len(ui.sshSnippetExecuteBtns) != 2 || len(ui.sshSnippetEditBtns) != 2 || len(ui.sshSnippetDeleteBtns) != 2 {
		t.Fatalf("snippet button counts = execute:%d edit:%d delete:%d", len(ui.sshSnippetExecuteBtns), len(ui.sshSnippetEditBtns), len(ui.sshSnippetDeleteBtns))
	}

	ui.syncSSHCommandSnippetButtons([]sshCommandSnippetEntry{
		{Snippet: remote.SSHCommandSnippet{ID: "two"}},
	})
	if strings.Join(ui.sshSnippetVisibleIDs, ",") != "two" {
		t.Fatalf("visible snippet ids after filtering = %v", ui.sshSnippetVisibleIDs)
	}
	if len(ui.sshSnippetExecuteBtns) != 1 || len(ui.sshSnippetEditBtns) != 1 || len(ui.sshSnippetDeleteBtns) != 1 {
		t.Fatalf("filtered snippet button counts = execute:%d edit:%d delete:%d", len(ui.sshSnippetExecuteBtns), len(ui.sshSnippetEditBtns), len(ui.sshSnippetDeleteBtns))
	}
}
