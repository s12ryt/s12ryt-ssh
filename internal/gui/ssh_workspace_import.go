package gui

import (
	"strings"

	"s12ryt-ssh/internal/remote"
)

// sshWorkspaceImportState keeps only the opaque package and its preview. The
// decrypted workspace payload never enters the desktop UI state.
type sshWorkspaceImportState struct {
	Package         string
	Password        string
	IncludesSecrets bool
	Counts          remote.SSHWorkspaceResourceCounts
	Conflicts       []remote.SSHWorkspaceImportConflict
	decisions       map[string]remote.SSHWorkspaceImportDecision
}

type sshWorkspaceExportState struct {
	IncludeSecrets bool
	Password       string
}

func validateSSHWorkspaceExportState(state sshWorkspaceExportState) string {
	if state.IncludeSecrets && strings.TrimSpace(state.Password) == "" {
		return "Export password is required when secrets are included."
	}
	return ""
}

func newSSHWorkspaceImportState(preview remote.SSHWorkspaceImportPreview) *sshWorkspaceImportState {
	conflicts := append([]remote.SSHWorkspaceImportConflict(nil), preview.Conflicts...)
	return &sshWorkspaceImportState{
		IncludesSecrets: preview.IncludesSecrets,
		Counts:          preview.Counts,
		Conflicts:       conflicts,
		decisions:       make(map[string]remote.SSHWorkspaceImportDecision),
	}
}

func validateSSHWorkspaceImportState(state *sshWorkspaceImportState) string {
	if state == nil || strings.TrimSpace(state.Package) == "" {
		return "Import package is required."
	}
	if state.IncludesSecrets && strings.TrimSpace(state.Password) == "" {
		return "Import password is required for encrypted packages."
	}
	if state == nil {
		return "Import package is required."
	}
	for _, conflict := range state.Conflicts {
		if !conflict.Conflict {
			continue
		}
		if _, ok := state.decisions[sshWorkspaceImportDecisionKey(conflict.Kind, conflict.Name)]; !ok {
			return "Import conflict decisions are incomplete."
		}
	}
	return ""
}

func (state *sshWorkspaceImportState) setResolution(
	kind remote.SSHWorkspaceImportKind,
	name string,
	action remote.SSHWorkspaceImportDecision,
) bool {
	if state == nil || state.decisions == nil {
		return false
	}
	if action != remote.SSHWorkspaceImportOverwrite && action != remote.SSHWorkspaceImportSkip && action != remote.SSHWorkspaceImportCopy {
		return false
	}
	for _, conflict := range state.Conflicts {
		if conflict.Conflict && conflict.Kind == kind && strings.EqualFold(strings.TrimSpace(conflict.Name), strings.TrimSpace(name)) {
			state.decisions[sshWorkspaceImportDecisionKey(kind, conflict.Name)] = action
			return true
		}
	}
	return false
}

func (state *sshWorkspaceImportState) resolutions() []remote.SSHWorkspaceImportResolution {
	if state == nil {
		return nil
	}
	resolutions := make([]remote.SSHWorkspaceImportResolution, 0, len(state.Conflicts))
	for _, conflict := range state.Conflicts {
		if !conflict.Conflict {
			continue
		}
		action, ok := state.decisions[sshWorkspaceImportDecisionKey(conflict.Kind, conflict.Name)]
		if !ok {
			continue
		}
		resolutions = append(resolutions, remote.SSHWorkspaceImportResolution{
			Kind:   conflict.Kind,
			Name:   conflict.Name,
			Action: action,
		})
	}
	return resolutions
}

func sshWorkspaceImportDecisionKey(kind remote.SSHWorkspaceImportKind, name string) string {
	return string(kind) + "\x00" + strings.ToLower(strings.TrimSpace(name))
}
