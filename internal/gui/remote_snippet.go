package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"s12ryt-ssh/internal/remote"

	"gioui.org/widget"
)

// remoteSnippetSession is optional so older authenticated sessions remain
// usable while command snippets are introduced.
type remoteSnippetSession interface {
	SSHCommandSnippets(context.Context) ([]remote.SSHCommandSnippet, error)
	SSHCommandSnippetSecrets(context.Context, string) (map[string]string, error)
}

type remoteSnippetCRUDSession interface {
	remoteSnippetSession
	CreateSSHCommandSnippet(context.Context, remote.SSHCommandSnippetInput) (remote.SSHCommandSnippet, error)
	UpdateSSHCommandSnippet(context.Context, string, remote.SSHCommandSnippetInput) (remote.SSHCommandSnippet, error)
	DeleteSSHCommandSnippet(context.Context, string) error
}

func (ui *Window) openSSHCommandSnippetExecution(id string) bool {
	if ui == nil || ui.sshSnippetExecutionOpen || ui.sshSnippets == nil || ui.busy {
		return false
	}
	var snippet remote.SSHCommandSnippet
	found := false
	for _, entry := range ui.sshSnippets.snapshot() {
		if entry.Snippet.ID == id {
			snippet = entry.Snippet
			found = true
			break
		}
	}
	if !found || !snippet.Enabled {
		return false
	}
	if len(snippet.Variables) == 0 {
		return ui.executeSSHCommandSnippet(id)
	}
	ui.sshSnippetExecutionOpen = true
	ui.sshSnippetExecutionID = id
	ui.sshSnippetVariableNames = append([]string(nil), snippet.Variables...)
	ui.sshSnippetVariableEditors = make([]widget.Editor, len(snippet.Variables))
	for index := range ui.sshSnippetVariableEditors {
		ui.sshSnippetVariableEditors[index].SingleLine = true
		ui.sshSnippetVariableEditors[index].Submit = true
	}
	ui.model.Error = ""
	return true
}

func (ui *Window) closeSSHCommandSnippetExecution() {
	if ui == nil {
		return
	}
	ui.sshSnippetExecutionOpen = false
	ui.sshSnippetExecutionID = ""
	ui.sshSnippetVariableNames = nil
	ui.sshSnippetVariableEditors = nil
	ui.sshSnippetExecutionClose = widget.Clickable{}
	ui.sshSnippetExecutionCancel = widget.Clickable{}
	ui.sshSnippetExecutionRun = widget.Clickable{}
	ui.sshSnippetExecutionScrim = widget.Clickable{}
}

func (ui *Window) openSSHCommandSnippetForm(id string) bool {
	if ui == nil || ui.sshSnippetFormOpen || ui.busy || ui.sshSnippets == nil {
		return false
	}
	values := sshCommandSnippetFormValues{Enabled: true}
	if id != "" {
		var found bool
		for _, entry := range ui.sshSnippets.snapshot() {
			if entry.Snippet.ID != id {
				continue
			}
			values = sshCommandSnippetFormFromSnippet(entry.Snippet)
			found = true
			break
		}
		if !found {
			return false
		}
	}
	ui.sshSnippetFormOpen = true
	ui.sshSnippetFormID = id
	ui.sshSnippetForm = values
	ui.setSSHCommandSnippetFormEditors(values)
	ui.model.Error = ""
	return true
}

func sshCommandSnippetFormFromSnippet(snippet remote.SSHCommandSnippet) sshCommandSnippetFormValues {
	return sshCommandSnippetFormValues{
		ID:            snippet.ID,
		Name:          snippet.Name,
		Command:       snippet.Command,
		VariablesText: strings.Join(snippet.Variables, ", "),
		Enabled:       snippet.Enabled,
	}
}

func (ui *Window) setSSHCommandSnippetFormEditors(values sshCommandSnippetFormValues) {
	ui.sshSnippetName.SetText(values.Name)
	ui.sshSnippetCommand.SetText(values.Command)
	ui.sshSnippetVariables.SetText(values.VariablesText)
	ui.sshSnippetSecrets.SetText(values.SecretValuesText)
	ui.sshSnippetClearSecrets.Value = values.ClearSecrets
	ui.sshSnippetEnabled.Value = values.Enabled
	ui.sshSnippetSavedSecretNames = ""
	if ui.sshSnippetFormID != "" && ui.sshSnippets != nil {
		for _, entry := range ui.sshSnippets.snapshot() {
			if entry.Snippet.ID == ui.sshSnippetFormID {
				ui.sshSnippetSavedSecretNames = strings.Join(entry.Snippet.SecretNames, ", ")
				break
			}
		}
	}
}

func (ui *Window) closeSSHCommandSnippetForm() {
	if ui == nil {
		return
	}
	ui.sshSnippetFormOpen = false
	ui.sshSnippetFormID = ""
	ui.sshSnippetForm = sshCommandSnippetFormValues{}
	ui.sshSnippetName.SetText("")
	ui.sshSnippetCommand.SetText("")
	ui.sshSnippetVariables.SetText("")
	ui.sshSnippetSecrets.SetText("")
	ui.sshSnippetSavedSecretNames = ""
	ui.sshSnippetClearSecrets.Value = false
	ui.sshSnippetEnabled.Value = false
	ui.sshSnippetFormClose = widget.Clickable{}
	ui.sshSnippetFormCancel = widget.Clickable{}
	ui.sshSnippetFormSave = widget.Clickable{}
	ui.sshSnippetFormDelete = widget.Clickable{}
	ui.sshSnippetFormScrim = widget.Clickable{}
}

func (ui *Window) currentSSHCommandSnippetForm() sshCommandSnippetFormValues {
	values := ui.sshSnippetForm
	values.ID = ui.sshSnippetFormID
	values.Name = ui.sshSnippetName.Text()
	values.Command = ui.sshSnippetCommand.Text()
	values.VariablesText = ui.sshSnippetVariables.Text()
	values.SecretValuesText = ui.sshSnippetSecrets.Text()
	values.ClearSecrets = ui.sshSnippetClearSecrets.Value
	values.Enabled = ui.sshSnippetEnabled.Value
	return values
}

func snippetFormErrorSource(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	switch {
	case strings.HasPrefix(message, "Snippet name is required."):
		return "Snippet name is required."
	case strings.HasPrefix(message, "Command is required."):
		return "Command is required."
	case strings.HasPrefix(message, "Variable name"):
		return "Variable name is invalid."
	case strings.HasPrefix(message, "Duplicate variable"):
		return "Duplicate variable name."
	case strings.HasPrefix(message, "Secret entry"):
		return "Secret entry must use NAME=value."
	case strings.HasPrefix(message, "Duplicate secret"):
		return "Duplicate secret name."
	default:
		return message
	}
}

func (ui *Window) submitSSHCommandSnippetForm() bool {
	if ui == nil || !ui.sshSnippetFormOpen || ui.busy {
		return false
	}
	values := ui.currentSSHCommandSnippetForm()
	input, err := values.input()
	if err != nil {
		ui.model.Error = ui.text(snippetFormErrorSource(err))
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteSnippetCRUDSession)
	if !ok {
		ui.model.Error = "SSH command snippet service is unavailable"
		return false
	}
	id := ui.sshSnippetFormID
	ui.closeSSHCommandSnippetForm()
	ui.async(ui.text("Saving SSH snippet..."), func(ctx context.Context) (func(), error) {
		var (
			snippet remote.SSHCommandSnippet
			err     error
		)
		if id == "" {
			snippet, err = session.CreateSSHCommandSnippet(ctx, input)
		} else {
			snippet, err = session.UpdateSSHCommandSnippet(ctx, id, input)
		}
		if err != nil {
			return nil, err
		}
		return func() {
			if ui.sshSnippets == nil {
				ui.sshSnippets = newSSHCommandSnippetStore()
			}
			ui.sshSnippets.upsert(snippet)
		}, nil
	})
	return true
}

func (ui *Window) deleteSSHCommandSnippet(id string) bool {
	if ui == nil || ui.sshSnippets == nil || ui.busy {
		return false
	}
	if _, found := ui.snippetByID(id); !found {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteSnippetCRUDSession)
	if !ok {
		ui.model.Error = "SSH command snippet service is unavailable"
		return false
	}
	ui.requestConfirm(ui.text("Delete snippet?"), ui.text("This snippet will be permanently deleted."), func() {
		ui.async(ui.text("Deleting SSH snippet..."), func(ctx context.Context) (func(), error) {
			if err := session.DeleteSSHCommandSnippet(ctx, id); err != nil {
				return nil, err
			}
			return func() { ui.sshSnippets.remove(id) }, nil
		})
	})
	return true
}

func (ui *Window) snippetByID(id string) (remote.SSHCommandSnippet, bool) {
	if ui == nil || ui.sshSnippets == nil {
		return remote.SSHCommandSnippet{}, false
	}
	for _, entry := range ui.sshSnippets.snapshot() {
		if entry.Snippet.ID == id {
			return entry.Snippet, true
		}
	}
	return remote.SSHCommandSnippet{}, false
}

func (ui *Window) refreshSSHCommandSnippets() bool {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteSnippetSession)
	if !ok {
		ui.model.Error = "SSH command snippet service is unavailable"
		return false
	}
	if ui.sshSnippets == nil {
		ui.sshSnippets = newSSHCommandSnippetStore()
	}
	if ui.busy {
		return false
	}
	ui.async(ui.text("Working..."), func(ctx context.Context) (func(), error) {
		snippets, err := session.SSHCommandSnippets(ctx)
		if err != nil {
			return nil, err
		}
		return func() { ui.sshSnippets.replace(snippets) }, nil
	})
	return true
}

func (ui *Window) executeSSHCommandSnippet(id string) bool {
	if ui == nil || ui.sshSnippets == nil {
		return false
	}
	var snippet remote.SSHCommandSnippet
	found := false
	for _, entry := range ui.sshSnippets.snapshot() {
		if entry.Snippet.ID == id {
			snippet = entry.Snippet
			found = true
			break
		}
	}
	if !found || !snippet.Enabled {
		if !found {
			ui.model.Error = fmt.Sprintf("SSH command snippet %q was not found", id)
		} else {
			ui.model.Error = ui.text("Snippet is disabled.")
		}
		return false
	}
	tab := ui.sshTabs.active()
	if tab == nil || tab.State != sshTabConnected || tab.session == nil || tab.session.pty == nil {
		ui.model.Error = ui.text("No terminal tab is connected.")
		return false
	}
	tabID := tab.ID
	tabSession := tab.session
	variables := ui.currentSSHSnippetVariables(snippet)
	remoteSession, hasRemoteSession := ui.model.RemoteSession.(remoteSnippetSession)
	if len(snippet.SecretNames) > 0 && !hasRemoteSession {
		ui.model.Error = "SSH command snippet service is unavailable"
		return false
	}
	if len(snippet.SecretNames) == 0 {
		command, err := expandSSHCommandSnippet(snippet, variables, nil)
		if err != nil {
			ui.model.Error = err.Error()
			return false
		}
		return ui.writeSSHCommandSnippet(tabID, tabSession, command)
	}
	if ui.busy {
		return false
	}
	ui.async(ui.text("Working..."), func(ctx context.Context) (func(), error) {
		secrets, err := remoteSession.SSHCommandSnippetSecrets(ctx, id)
		if err != nil {
			return nil, err
		}
		command, err := expandSSHCommandSnippet(snippet, variables, secrets)
		if err != nil {
			return nil, err
		}
		return func() {
			if err := ui.writeSSHCommandSnippetBytes(tabID, tabSession, command); err != nil {
				ui.model.Error = err.Error()
			}
		}, nil
	})
	return true
}

func (ui *Window) currentSSHSnippetVariables(snippet remote.SSHCommandSnippet) map[string]string {
	names := snippet.Variables
	if len(ui.sshSnippetVariableNames) == len(ui.sshSnippetVariableEditors) && len(names) == len(ui.sshSnippetVariableNames) {
		names = ui.sshSnippetVariableNames
	}
	values := make(map[string]string, len(names))
	for index, name := range names {
		if index < len(ui.sshSnippetVariableEditors) {
			values[name] = ui.sshSnippetVariableEditors[index].Text()
		}
	}
	return values
}

func (ui *Window) writeSSHCommandSnippet(tabID string, session *sshTabSession, command string) bool {
	if err := ui.writeSSHCommandSnippetBytes(tabID, session, command); err != nil {
		ui.model.Error = err.Error()
		return false
	}
	return true
}

func (ui *Window) writeSSHCommandSnippetBytes(tabID string, session *sshTabSession, command string) error {
	data, _ := prepareTerminalPaste(command + "\n")
	if len(data) == 0 {
		return errors.New("SSH command snippet is empty")
	}
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.State != sshTabConnected || tab.session != session || session == nil || session.pty == nil {
		return errors.New("terminal tab is no longer connected")
	}
	n, err := session.pty.Write(data)
	if err != nil {
		ui.failSSHTab(tabID, err)
		return err
	}
	if n != len(data) {
		err = fmt.Errorf("write command snippet: %w", io.ErrShortWrite)
		ui.failSSHTab(tabID, err)
		return err
	}
	return nil
}
