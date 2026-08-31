package gui

import (
	"os"
	"runtime"
	"sort"
	"strings"

	"gioui.org/widget"

	"s12ryt-ssh/internal/remote"
)

type sshWorkspaceModule uint8

const (
	sshWorkspaceHosts sshWorkspaceModule = iota
	sshWorkspaceTunnels
	sshWorkspaceSnippets
	sshWorkspaceKeys
	sshWorkspaceFingerprints
	sshWorkspaceHistory
)

type sshWorkspaceNavItem struct {
	ID          sshWorkspaceModule
	LabelSource string
}

type sshHostGroupSummary struct {
	Path  string
	Name  string
	Count int
}

func sshWorkspaceNavigation() []sshWorkspaceNavItem {
	return []sshWorkspaceNavItem{
		{ID: sshWorkspaceHosts, LabelSource: "Hosts"},
		{ID: sshWorkspaceTunnels, LabelSource: "Port forwarding"},
		{ID: sshWorkspaceSnippets, LabelSource: "Command snippets"},
		{ID: sshWorkspaceKeys, LabelSource: "Key management"},
		{ID: sshWorkspaceFingerprints, LabelSource: "Host fingerprints"},
		{ID: sshWorkspaceHistory, LabelSource: "Session history"},
	}
}

func sshWorkspaceModuleTitle(module sshWorkspaceModule) string {
	for _, item := range sshWorkspaceNavigation() {
		if item.ID == module {
			return item.LabelSource
		}
	}
	return ""
}

func (ui *Window) setSSHWorkspaceModule(module sshWorkspaceModule) bool {
	if ui == nil {
		return false
	}
	for _, item := range sshWorkspaceNavigation() {
		if item.ID != module {
			continue
		}
		ui.workspaceModule = module
		ui.model.Error = ""
		return true
	}
	return false
}

func (ui *Window) openLocalTerminalTab() *sshTab {
	if ui == nil {
		return nil
	}
	return ui.sshTabs.openLocal("Local terminal")
}

func localShellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		if command := strings.TrimSpace(os.Getenv("ComSpec")); command != "" {
			return command, []string{"/d"}
		}
		return "cmd.exe", []string{"/d"}
	}
	if command := strings.TrimSpace(os.Getenv("SHELL")); command != "" {
		return command, []string{"-i"}
	}
	return "/bin/sh", []string{"-i"}
}

func filterSSHHosts(hosts []remote.SSHHost, query string) []remote.SSHHost {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return append([]remote.SSHHost(nil), hosts...)
	}
	filtered := make([]remote.SSHHost, 0, len(hosts))
	for _, host := range hosts {
		values := []string{host.Name, host.Host, host.GroupPath, string(host.AuthMethod)}
		values = append(values, host.Tags...)
		matched := false
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), needle) {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, host)
		}
	}
	return filtered
}

func groupSSHHosts(hosts []remote.SSHHost) []sshHostGroupSummary {
	counts := make(map[string]int)
	for _, host := range hosts {
		counts[strings.TrimSpace(host.GroupPath)]++
	}
	groups := make([]sshHostGroupSummary, 0, len(counts))
	for path, count := range counts {
		name := path
		if path == "" {
			name = "Ungrouped"
		} else if index := strings.LastIndex(path, "/"); index >= 0 {
			name = path[index+1:]
		}
		groups = append(groups, sshHostGroupSummary{Path: path, Name: name, Count: count})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Path == "" {
			return false
		}
		if groups[j].Path == "" {
			return true
		}
		return groups[i].Path < groups[j].Path
	})
	return groups
}

func (ui *Window) rebuildSSHHostFilter() {
	if ui == nil {
		return
	}
	ui.sshHostQuery = strings.TrimSpace(ui.workspaceSearch.Text())
	filtered := filterSSHHosts(ui.sshHosts, ui.sshHostQuery)
	indices := make([]int, 0, len(filtered))
	for _, filteredHost := range filtered {
		for index, host := range ui.sshHosts {
			if host.ID == filteredHost.ID {
				indices = append(indices, index)
				break
			}
		}
	}
	ui.sshHostIndices = indices
	ui.sshHostButtons = make([]widget.Clickable, len(indices))
	ui.sshHostEditButtons = make([]widget.Clickable, len(indices))
	recent := recentSSHHosts(filtered, 4)
	ui.sshRecentHostIndices = make([]int, 0, len(recent))
	for _, recentHost := range recent {
		for index, host := range ui.sshHosts {
			if host.ID == recentHost.ID {
				ui.sshRecentHostIndices = append(ui.sshRecentHostIndices, index)
				break
			}
		}
	}
	ui.sshRecentButtons = make([]widget.Clickable, len(ui.sshRecentHostIndices))
}

func (ui *Window) rebuildSSHHostFilterIfNeeded() {
	if ui == nil {
		return
	}
	query := strings.TrimSpace(ui.workspaceSearch.Text())
	if query != ui.sshHostQuery || len(ui.sshHostIndices) > len(ui.sshHosts) {
		ui.rebuildSSHHostFilter()
	}
}
