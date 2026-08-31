package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/widget"

	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

func (ui *Window) openPooledSFTP(credentials remote.SSHHostCredentials) (sshclient.SFTPClient, error) {
	if ui == nil {
		return nil, errors.New("ssh connection pool: window is not available")
	}
	if ui.sshPool == nil {
		ui.sshPool = newSSHConnectionPool()
	}
	factory := ui.sshTransportFactory
	if factory == nil {
		factory = newSSHClientTransport
	}
	return openPooledSFTP(ui.sshPool, credentials, factory)
}

func (ui *Window) openSSHTabSFTP(tabID string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.session == nil || ui.model.RemoteSession == nil {
		return false
	}
	terminalSession := tab.session
	tab.View = sshTabViewSFTP
	tab.sftpLoading = true
	tab.sftpError = ""
	tab.sftpInfo = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		credentials, err := ui.model.RemoteSession.SSHHostCredentials(ctx, tab.HostID)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTabSFTP(tabID, terminalSession, err) })
			return
		}
		client, err := ui.openPooledSFTP(credentials)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTabSFTP(tabID, terminalSession, err) })
			return
		}
		entries, err := client.ReadDir(ctx, "/")
		if err != nil {
			_ = client.Close()
			ui.queueSSHTabApply(func() { ui.failSSHTabSFTP(tabID, terminalSession, err) })
			return
		}
		ui.queueSSHTabApply(
			func() { ui.attachSSHTabSFTP(tabID, terminalSession, client, "/", entries) },
			func() { _ = client.Close() },
		)
	}()
	return true
}

func (ui *Window) failSSHTabSFTP(tabID string, terminalSession *sshTabSession, err error) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.session != terminalSession {
		return false
	}
	tab.sftpLoading = false
	if err != nil {
		tab.sftpError = err.Error()
	}
	return true
}

func (ui *Window) refreshSSHTabSFTP(tabID string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.session == nil || tab.session.sftp == nil || tab.sftpBrowser == nil {
		return false
	}
	terminalSession := tab.session
	client := terminalSession.sftp
	remotePath := tab.sftpBrowser.Path
	tab.sftpLoading = true
	tab.sftpError = ""
	tab.sftpInfo = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		entries, err := client.ReadDir(ctx, remotePath)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTabSFTP(tabID, terminalSession, err) })
			return
		}
		ui.queueSSHTabApply(func() {
			ui.applySSHTabSFTPEntries(tabID, terminalSession, remotePath, entries)
		})
	}()
	return true
}

func (ui *Window) enterSSHTabSFTPDirectory(tabID, name string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.sftpBrowser == nil {
		return false
	}
	previousPath := tab.sftpBrowser.Path
	if !tab.sftpBrowser.enter(name) {
		return false
	}
	if ui.refreshSSHTabSFTP(tabID) {
		return true
	}
	tab.sftpBrowser.Path = previousPath
	return false
}

func (ui *Window) parentSSHTabSFTP(tabID string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.sftpBrowser == nil {
		return false
	}
	previousPath := tab.sftpBrowser.Path
	if !tab.sftpBrowser.parent() {
		return false
	}
	if ui.refreshSSHTabSFTP(tabID) {
		return true
	}
	tab.sftpBrowser.Path = previousPath
	return false
}

func sftpOperationInputError(action, first, second string) string {
	switch action {
	case "New folder":
		if strings.TrimSpace(first) == "" {
			return "Folder name is required."
		}
	case "Rename item":
		if strings.TrimSpace(first) == "" {
			return "New name is required."
		}
	case "Create symbolic link":
		if strings.TrimSpace(first) == "" {
			return "Target path is required."
		}
		if strings.TrimSpace(second) == "" {
			return "Link name is required."
		}
	}
	return ""
}

func (ui *Window) openSSHTabSFTPOperation(tabID, action string) bool {
	if ui == nil || ui.sftpOperationOpen || ui.busy {
		return false
	}
	if _, ok := sftpOperationDialogSpec(action); !ok {
		return false
	}
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.View != sshTabViewSFTP || tab.session == nil || tab.session.sftp == nil || tab.sftpBrowser == nil || tab.sftpLoading {
		return false
	}
	ui.sftpOperationOpen = true
	ui.sftpOperationAction = action
	ui.sftpOperationTabID = tabID
	ui.sftpOperationFirst.SetText("")
	ui.sftpOperationSecond.SetText("")
	ui.model.Error = ""
	return true
}

func (ui *Window) closeSFTPOperation() {
	if ui == nil {
		return
	}
	ui.sftpOperationOpen = false
	ui.sftpOperationAction = ""
	ui.sftpOperationTabID = ""
	ui.sftpOperationFirst.SetText("")
	ui.sftpOperationSecond.SetText("")
	ui.sftpOperationClose = widget.Clickable{}
	ui.sftpOperationCancel = widget.Clickable{}
	ui.sftpOperationSave = widget.Clickable{}
	ui.sftpOperationScrim = widget.Clickable{}
}

func (ui *Window) createSSHTabSFTPFolder(tabID, name string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.session == nil || tab.session.sftp == nil || tab.sftpBrowser == nil {
		return false
	}
	remotePath, ok := sftpChildPath(tab.sftpBrowser.Path, name)
	if !ok || tab.sftpLoading {
		return false
	}
	return ui.runSSHTabSFTPMutation(tabID, tab.session, remotePath, func(client sshclient.SFTPClient, path string) error {
		return client.Mkdir(path)
	})
}

func (ui *Window) renameSSHTabSFTPItem(tabID, oldName, newName string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.session == nil || tab.session.sftp == nil || tab.sftpBrowser == nil || tab.sftpLoading {
		return false
	}
	oldPath, oldOK := sftpChildPath(tab.sftpBrowser.Path, oldName)
	newPath, newOK := sftpChildPath(tab.sftpBrowser.Path, newName)
	if !oldOK || !newOK || oldPath == newPath {
		return false
	}
	for _, entry := range tab.sftpBrowser.Entries {
		if entry.Path == oldPath {
			return ui.runSSHTabSFTPOperation(tabID, tab.session, tab.sftpBrowser.Path, func(client sshclient.SFTPClient) error {
				return client.Rename(oldPath, newPath)
			})
		}
	}
	return false
}

func (ui *Window) deleteSelectedSSHTabSFTPItems(tabID string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.session == nil || tab.session.sftp == nil || tab.sftpBrowser == nil || tab.sftpLoading {
		return false
	}
	selected := tab.sftpBrowser.selectedPaths()
	if len(selected) == 0 {
		return false
	}
	entries := make(map[string]sftpEntry, len(tab.sftpBrowser.Entries))
	for _, entry := range tab.sftpBrowser.Entries {
		entries[entry.Path] = entry
	}
	ui.requestConfirm(ui.text("Delete remote items?"), ui.text("This will permanently delete the selected remote items."), func() {
		ui.runSSHTabSFTPOperation(tabID, tab.session, tab.sftpBrowser.Path, func(client sshclient.SFTPClient) error {
			for _, remotePath := range selected {
				entry, ok := entries[remotePath]
				if !ok {
					continue
				}
				if entry.Directory {
					if err := client.RemoveDirectory(remotePath); err != nil {
						return err
					}
					continue
				}
				if err := client.Remove(remotePath); err != nil {
					return err
				}
			}
			return nil
		})
	})
	return true
}

func (ui *Window) showSSHTabSFTPItemInfo(tabID, remotePath string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.session == nil || tab.session.sftp == nil || tab.sftpLoading {
		return false
	}
	remotePath = cleanRemotePath(remotePath)
	terminalSession := tab.session
	client := terminalSession.sftp
	tab.sftpLoading = true
	tab.sftpError = ""
	tab.sftpInfo = ""
	go func() {
		entry, err := client.Lstat(remotePath)
		ui.queueSSHTabApply(func() {
			current := ui.sshTabs.get(tabID)
			if current == nil || current.session != terminalSession {
				return
			}
			current.sftpLoading = false
			if err != nil {
				current.sftpError = err.Error()
				return
			}
			current.sftpInfo = formatSFTPEntryInfo(entry, ui.text)
		})
	}()
	return true
}

func formatSFTPEntryInfo(entry sshclient.SFTPEntry, translate func(string) string) string {
	if translate == nil {
		translate = func(source string) string { return source }
	}
	kind := translate("file")
	if entry.Directory {
		kind = translate("directory")
	} else if entry.Symlink {
		kind = translate("symbolic link")
	}
	return fmt.Sprintf("%s\n%s: %s\n%s: %s\n%s: %d B\n%s: %s\n%s: %s", entry.Name, translate("Remote path"), cleanRemotePath(entry.Path), translate("Type"), kind, translate("Size"), entry.Size, translate("Mode"), entry.Mode, translate("Modified"), entry.Modified.Format(time.RFC3339))
}

func (ui *Window) createSSHTabSFTPSymlink(tabID, targetPath, linkName string) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.State != sshTabConnected || tab.session == nil || tab.session.sftp == nil || tab.sftpBrowser == nil || tab.sftpLoading {
		return false
	}
	linkPath, ok := sftpChildPath(tab.sftpBrowser.Path, linkName)
	if !ok || strings.TrimSpace(targetPath) == "" {
		return false
	}
	targetPath = cleanRemotePath(targetPath)
	return ui.runSSHTabSFTPOperation(tabID, tab.session, tab.sftpBrowser.Path, func(client sshclient.SFTPClient) error {
		return client.Symlink(targetPath, linkPath)
	})
}

func (ui *Window) runSSHTabSFTPMutation(
	tabID string,
	terminalSession *sshTabSession,
	remotePath string,
	mutate func(sshclient.SFTPClient, string) error,
) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || terminalSession == nil || terminalSession.sftp == nil || mutate == nil {
		return false
	}
	return ui.runSSHTabSFTPOperation(tabID, terminalSession, tab.sftpBrowser.Path, func(client sshclient.SFTPClient) error {
		return mutate(client, remotePath)
	})
}

func (ui *Window) runSSHTabSFTPOperation(
	tabID string,
	terminalSession *sshTabSession,
	browserPath string,
	operation func(sshclient.SFTPClient) error,
) bool {
	if terminalSession == nil || terminalSession.sftp == nil || operation == nil {
		return false
	}
	client := terminalSession.sftp
	browserPath = cleanRemotePath(browserPath)
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.session != terminalSession || tab.sftpBrowser == nil || tab.sftpLoading {
		return false
	}
	tab.sftpLoading = true
	tab.sftpError = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := operation(client); err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTabSFTP(tabID, terminalSession, err) })
			return
		}
		entries, err := client.ReadDir(ctx, browserPath)
		if err != nil {
			ui.queueSSHTabApply(func() { ui.failSSHTabSFTP(tabID, terminalSession, err) })
			return
		}
		ui.queueSSHTabApply(func() {
			ui.applySSHTabSFTPEntries(tabID, terminalSession, browserPath, entries)
		})
	}()
	return true
}

func (ui *Window) attachSSHTabSFTP(
	tabID string,
	terminalSession *sshTabSession,
	client sshclient.SFTPClient,
	remotePath string,
	entries []sshclient.SFTPEntry,
) bool {
	if client == nil {
		return false
	}
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.Local || tab.session != terminalSession {
		_ = client.Close()
		return false
	}
	if terminalSession.sftp != nil && terminalSession.sftp != client {
		_ = terminalSession.sftp.Close()
	}
	terminalSession.sftp = client
	return ui.applySSHTabSFTPEntries(tabID, terminalSession, remotePath, entries)
}

func (ui *Window) applySSHTabSFTPEntries(
	tabID string,
	terminalSession *sshTabSession,
	remotePath string,
	entries []sshclient.SFTPEntry,
) bool {
	tab := ui.sshTabs.get(tabID)
	if tab == nil || tab.session != terminalSession {
		return false
	}
	if tab.sftpBrowser == nil {
		tab.sftpBrowser = newSFTPBrowserState(remotePath)
	} else {
		tab.sftpBrowser.Path = cleanRemotePath(remotePath)
	}
	tab.sftpBrowser.applyEntries(sftpEntriesFromTransport(entries))
	tab.syncSFTPEntryWidgets()
	tab.sftpLoading = false
	tab.sftpError = ""
	tab.View = sshTabViewSFTP
	return true
}

func (ui *Window) handleSSHTabSFTP(gtx layout.Context, tab *sshTab) bool {
	if tab == nil || tab.Local || tab.session == nil || tab.session.sftp == nil {
		return false
	}
	if ui.handleSFTPDrop(gtx, tab) {
		return true
	}
	ui.ensureSFTPActionButtons(tab)
	for index, source := range sftpActionSources() {
		if index >= len(tab.sftpActionButtons) || !tab.sftpActionButtons[index].Clicked(gtx) {
			continue
		}
		selected := len(tab.sftpBrowser.selectedPaths())
		if !sftpActionSelectionValid(source, selected) {
			if source == "Rename item" || source == "File information" {
				ui.model.Error = ui.text("Select exactly one item.")
			} else {
				ui.model.Error = ui.text("Select at least one item.")
			}
			return true
		}
		switch source {
		case "Upload files":
			ui.requestSFTPUploadFiles(tab.ID)
		case "Download selected":
			ui.requestSFTPDownloadFiles(tab.ID)
		case "New folder", "Create symbolic link":
			ui.openSSHTabSFTPOperation(tab.ID, source)
		case "Delete selected":
			ui.deleteSelectedSSHTabSFTPItems(tab.ID)
		case "File information":
			paths := tab.sftpBrowser.selectedPaths()
			if len(paths) == 1 {
				ui.showSSHTabSFTPItemInfo(tab.ID, paths[0])
			}
		case "Rename item":
			if entry, ok := selectedSFTPEntry(tab); ok {
				if ui.openSSHTabSFTPOperation(tab.ID, source) {
					ui.sftpOperationFirst.SetText("")
					ui.sftpOperationAction = source
					_ = entry
				}
			}
		}
		return true
	}
	if tab.sftpParentButton.Clicked(gtx) {
		ui.parentSSHTabSFTP(tab.ID)
		return true
	}
	if tab.sftpRefreshButton.Clicked(gtx) {
		ui.refreshSSHTabSFTP(tab.ID)
		return true
	}
	if tab.sftpBrowser == nil || tab.sftpLoading {
		return false
	}
	tab.syncSFTPEntryWidgets()
	for index := range tab.sftpSelectionWidgets {
		if !tab.sftpSelectionWidgets[index].Update(gtx) || index >= len(tab.sftpEntryPaths) {
			continue
		}
		remotePath := tab.sftpEntryPaths[index]
		if tab.sftpSelectionWidgets[index].Value {
			tab.sftpBrowser.selections[remotePath] = true
		} else {
			delete(tab.sftpBrowser.selections, remotePath)
		}
		return true
	}
	for index := range tab.sftpOpenButtons {
		if !tab.sftpOpenButtons[index].Clicked(gtx) || index >= len(tab.sftpBrowser.Entries) {
			continue
		}
		entry := tab.sftpBrowser.Entries[index]
		if entry.Directory {
			ui.enterSSHTabSFTPDirectory(tab.ID, entry.Name)
		}
		return true
	}
	return false
}

func (ui *Window) handleSFTPDrop(gtx layout.Context, tab *sshTab) bool {
	if ui == nil || tab == nil || tab.Local || tab.session == nil || tab.session.sftp == nil {
		return false
	}
	for _, mimeType := range []string{"text/uri-list", "text/plain"} {
		for {
			raw, ok := gtx.Event(transfer.TargetFilter{Target: &tab.sftpDropTag, Type: mimeType})
			if !ok {
				break
			}
			dataEvent, ok := raw.(transfer.DataEvent)
			if !ok || dataEvent.Open == nil {
				if ui.model != nil {
					ui.model.Error = ui.text("Dropped files are invalid.")
				}
				return true
			}
			ui.handleSFTPDropData(tab.ID, dataEvent.Open())
			return true
		}
	}
	return false
}

func (ui *Window) ensureSFTPActionButtons(tab *sshTab) {
	if tab == nil {
		return
	}
	want := len(sftpActionSources())
	if len(tab.sftpActionButtons) != want {
		tab.sftpActionButtons = make([]widget.Clickable, want)
	}
}

func selectedSFTPEntry(tab *sshTab) (sftpEntry, bool) {
	if tab == nil || tab.sftpBrowser == nil {
		return sftpEntry{}, false
	}
	paths := tab.sftpBrowser.selectedPaths()
	if len(paths) != 1 {
		return sftpEntry{}, false
	}
	for _, entry := range tab.sftpBrowser.Entries {
		if entry.Path == paths[0] {
			return entry, true
		}
	}
	return sftpEntry{}, false
}

func (ui *Window) handleSFTPOperation(gtx layout.Context) {
	if ui.drainEditors(gtx, &ui.sftpOperationFirst, &ui.sftpOperationSecond) {
		ui.submitSFTPOperation()
		return
	}
	if ui.sftpOperationClose.Clicked(gtx) || ui.sftpOperationCancel.Clicked(gtx) || ui.sftpOperationScrim.Clicked(gtx) || ui.escapePressed(gtx) {
		ui.closeSFTPOperation()
		return
	}
	if ui.sftpOperationSave.Clicked(gtx) {
		ui.submitSFTPOperation()
	}
}

func (ui *Window) submitSFTPOperation() bool {
	if !ui.sftpOperationOpen {
		return false
	}
	first := ui.sftpOperationFirst.Text()
	second := ui.sftpOperationSecond.Text()
	if source := sftpOperationInputError(ui.sftpOperationAction, first, second); source != "" {
		ui.model.Error = ui.text(source)
		return false
	}
	tab := ui.sshTabs.get(ui.sftpOperationTabID)
	if tab == nil || tab.sftpBrowser == nil {
		return false
	}
	started := false
	switch ui.sftpOperationAction {
	case "New folder":
		started = ui.createSSHTabSFTPFolder(tab.ID, first)
	case "Rename item":
		if entry, ok := selectedSFTPEntry(tab); ok {
			started = ui.renameSSHTabSFTPItem(tab.ID, entry.Name, first)
		}
	case "Create symbolic link":
		started = ui.createSSHTabSFTPSymlink(tab.ID, first, second)
	}
	if started {
		ui.model.Error = ""
		ui.closeSFTPOperation()
	}
	return started
}
