package remote

import (
	"context"
	"strings"
	"testing"
)

func TestSessionSSHWorkspaceImportExportLifecycle(t *testing.T) {
	fixture := newSSHFixture(t)
	session := fixture.loginSession(t)
	ctx := context.Background()

	encoded, err := session.ExportSSHWorkspace(ctx, SSHWorkspaceExportRequest{
		IncludeSecrets: true,
		Password:       "correct horse battery staple",
	})
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "encrypted-workspace-package" {
		t.Fatalf("encoded package = %q", encoded)
	}
	if body := fixture.lastBody(); !strings.Contains(body, `"includeSecrets":true`) || strings.Contains(body, "resolutions") {
		t.Fatalf("export body = %s", body)
	}

	preview, err := session.PreviewSSHWorkspaceImport(ctx, encoded, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.IncludesSecrets || preview.Counts.Hosts != 1 || len(preview.Conflicts) != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Conflicts[0].Kind != SSHWorkspaceImportHost || preview.Conflicts[0].Name != "web" || !preview.Conflicts[0].Conflict {
		t.Fatalf("conflict = %+v", preview.Conflicts[0])
	}

	result, err := session.ApplySSHWorkspaceImport(ctx, SSHWorkspaceImportRequest{
		Package:  encoded,
		Password: "correct horse battery staple",
		Resolutions: []SSHWorkspaceImportResolution{
			{Kind: SSHWorkspaceImportHost, Name: "web", Action: SSHWorkspaceImportCopy},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IncludesSecrets || result.Counts.Copied != 1 || len(result.Items) != 1 {
		t.Fatalf("result = %+v", result)
	}
	item := result.Items[0]
	if item.Kind != SSHWorkspaceImportHost || item.Action != SSHWorkspaceImportCopy || item.TargetName != "web (2)" {
		t.Fatalf("plan item = %+v", item)
	}
	if body := fixture.lastBody(); !strings.Contains(body, `"resolutions"`) || strings.Contains(body, "workspace-import-password") {
		t.Fatalf("apply body = %s", body)
	}

	wantCalls := []string{
		"POST /api/v1/ssh/workspace/export",
		"POST /api/v1/ssh/workspace/import/preview",
		"POST /api/v1/ssh/workspace/import/apply",
	}
	calls := fixture.calls()
	if len(calls) < len(wantCalls) {
		t.Fatalf("calls = %+v", calls)
	}
	got := calls[len(calls)-len(wantCalls):]
	for index := range wantCalls {
		if got[index] != wantCalls[index] {
			t.Fatalf("calls = %+v", got)
		}
	}
}
