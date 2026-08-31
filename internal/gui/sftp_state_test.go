package gui

import (
	"reflect"
	"testing"
)

func TestSFTPBrowserNormalizesPathsAndSortsDirectoriesFirst(t *testing.T) {
	browser := newSFTPBrowserState("//srv///apps/../apps")
	browser.applyEntries([]sftpEntry{
		{Name: "zeta.log", Path: "/srv/apps/zeta.log", Size: 10},
		{Name: "beta", Path: "/srv/apps/beta", Directory: true},
		{Name: "Alpha", Path: "/srv/apps/Alpha", Directory: true},
		{Name: "link", Path: "/srv/apps/link", Symlink: true},
	})

	if browser.Path != "/srv/apps" {
		t.Fatalf("normalized path = %q, want /srv/apps", browser.Path)
	}
	names := make([]string, 0, len(browser.Entries))
	for _, entry := range browser.Entries {
		names = append(names, entry.Name)
	}
	if want := []string{"Alpha", "beta", "link", "zeta.log"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("entry order = %v, want %v", names, want)
	}
	if !browser.enter("beta") || browser.Path != "/srv/apps/beta" {
		t.Fatalf("enter directory = path %q", browser.Path)
	}
	if browser.enter("zeta.log") {
		t.Fatal("regular files must not be treated as directories")
	}
	if !browser.parent() || browser.Path != "/srv/apps" {
		t.Fatalf("parent path = %q", browser.Path)
	}
}

func TestSFTPChildPathRejectsTraversalAndEmptyNames(t *testing.T) {
	if got, ok := sftpChildPath("/srv", "notes.txt"); !ok || got != "/srv/notes.txt" {
		t.Fatalf("normal child path = %q, %v", got, ok)
	}
	for _, name := range []string{"", ".", "..", "../outside", "nested/file", "/absolute"} {
		if got, ok := sftpChildPath("/srv", name); ok {
			t.Fatalf("invalid child name %q produced %q", name, got)
		}
	}
	if got, ok := sftpChildPath("/", "notes.txt"); !ok || got != "/notes.txt" {
		t.Fatalf("root child path = %q, %v", got, ok)
	}
}

func TestSFTPActionSourcesExposeImplementedOperationsInOrder(t *testing.T) {
	want := []string{"Upload files", "Download selected", "New folder", "Rename item", "Delete selected", "File information", "Create symbolic link"}
	if got := sftpActionSources(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SFTP action sources = %v, want %v", got, want)
	}
}

func TestSFTPActionSelectionRulesAreExplicit(t *testing.T) {
	tests := []struct {
		action   string
		selected int
		valid    bool
	}{
		{action: "New folder", selected: 0, valid: true},
		{action: "New folder", selected: 2, valid: true},
		{action: "Upload files", selected: 0, valid: true},
		{action: "Download selected", selected: 0, valid: false},
		{action: "Download selected", selected: 2, valid: true},
		{action: "Rename item", selected: 0, valid: false},
		{action: "Rename item", selected: 1, valid: true},
		{action: "Rename item", selected: 2, valid: false},
		{action: "Delete selected", selected: 0, valid: false},
		{action: "Delete selected", selected: 2, valid: true},
		{action: "File information", selected: 0, valid: false},
		{action: "File information", selected: 1, valid: true},
		{action: "File information", selected: 2, valid: false},
		{action: "Create symbolic link", selected: 0, valid: true},
	}
	for _, test := range tests {
		if got := sftpActionSelectionValid(test.action, test.selected); got != test.valid {
			t.Errorf("selection rule %q with %d selected = %v, want %v", test.action, test.selected, got, test.valid)
		}
	}
}

func TestSFTPOperationDialogSpecsExposeRequiredInputs(t *testing.T) {
	tests := []struct {
		action string
		fields []string
		submit string
		valid  bool
	}{
		{action: "New folder", fields: []string{"Folder name"}, submit: "Create", valid: true},
		{action: "Rename item", fields: []string{"New name"}, submit: "Save name", valid: true},
		{action: "Create symbolic link", fields: []string{"Target path", "Link name"}, submit: "Create", valid: true},
		{action: "Delete selected", valid: false},
	}
	for _, test := range tests {
		spec, ok := sftpOperationDialogSpec(test.action)
		if ok != test.valid {
			t.Fatalf("dialog spec %q available = %v, want %v", test.action, ok, test.valid)
		}
		if !ok {
			continue
		}
		if !reflect.DeepEqual(spec.fieldSources, test.fields) || spec.submitSource != test.submit {
			t.Fatalf("dialog spec %q = %#v, want fields %v submit %q", test.action, spec, test.fields, test.submit)
		}
	}
}

func TestSFTPOperationInputValidationUsesActionSpecificSources(t *testing.T) {
	tests := []struct {
		action string
		first  string
		second string
		want   string
	}{
		{action: "New folder", want: "Folder name is required."},
		{action: "Rename item", want: "New name is required."},
		{action: "Create symbolic link", want: "Target path is required."},
		{action: "Create symbolic link", first: "/srv/target", want: "Link name is required."},
	}
	for _, test := range tests {
		t.Run(test.action+"/"+test.want, func(t *testing.T) {
			if got := sftpOperationInputError(test.action, test.first, test.second); got != test.want {
				t.Fatalf("operation input error = %q, want %q", got, test.want)
			}
		})
	}
	if got := sftpOperationInputError("New folder", "logs", ""); got != "" {
		t.Fatalf("valid folder input error = %q, want empty", got)
	}
	if got := sftpOperationInputError("Create symbolic link", "/srv/target", "link"); got != "" {
		t.Fatalf("valid symlink input error = %q, want empty", got)
	}
}

func TestSFTPInfoPanelPreservesEveryMetadataLine(t *testing.T) {
	info := "report.txt\nRemote path: /srv/report.txt\nType: file\nSize: 2048 B\nMode: -rw-r--r--\nModified: 2026-08-30T13:14:15Z"
	want := []string{
		"report.txt",
		"Remote path: /srv/report.txt",
		"Type: file",
		"Size: 2048 B",
		"Mode: -rw-r--r--",
		"Modified: 2026-08-30T13:14:15Z",
	}
	if got := sftpInfoLines(info); !reflect.DeepEqual(got, want) {
		t.Fatalf("info lines = %v, want %v", got, want)
	}
	if got := sftpInfoLines(" "); got != nil {
		t.Fatalf("empty info lines = %v, want nil", got)
	}
}

func TestOpenSFTPOperationCreatesModalForCurrentRemoteTab(t *testing.T) {
	ui := NewWindow(nil)
	remoteTab := ui.sshTabs.open(testSSHHost("host-1", "Remote"))
	remoteTab.State = sshTabConnected
	remoteTab.View = sshTabViewSFTP
	remoteTab.session = &sshTabSession{sftp: &testSFTPClient{}}
	remoteTab.sftpBrowser = newSFTPBrowserState("/")
	localTab := ui.sshTabs.openLocal("Local terminal")
	localTab.State = sshTabConnected

	if !ui.openSSHTabSFTPOperation(remoteTab.ID, "New folder") {
		t.Fatalf("connected remote SFTP tab should open an operation modal: local=%v state=%q view=%q session=%v sftp=%v browser=%v loading=%v busy=%v", remoteTab.Local, remoteTab.State, remoteTab.View, remoteTab.session != nil, remoteTab.session != nil && remoteTab.session.sftp != nil, remoteTab.sftpBrowser != nil, remoteTab.sftpLoading, ui.busy)
	}
	if !ui.sftpOperationOpen || ui.sftpOperationTabID != remoteTab.ID || ui.sftpOperationAction != "New folder" {
		t.Fatalf("operation modal = open %v, tab %q, action %q", ui.sftpOperationOpen, ui.sftpOperationTabID, ui.sftpOperationAction)
	}
	ui.closeSFTPOperation()
	if ui.sftpOperationOpen || ui.sftpOperationTabID != "" || ui.sftpOperationAction != "" || ui.sftpOperationFirst.Text() != "" {
		t.Fatalf("closed operation modal retained state: open %v, tab %q, action %q, first %q", ui.sftpOperationOpen, ui.sftpOperationTabID, ui.sftpOperationAction, ui.sftpOperationFirst.Text())
	}
	if ui.openSSHTabSFTPOperation(localTab.ID, "New folder") {
		t.Fatal("local tab must not open a remote SFTP operation modal")
	}
}

func TestSFTPBrowsersKeepIndependentSelections(t *testing.T) {
	first := newSFTPBrowserState("/")
	second := newSFTPBrowserState("/")
	entries := []sftpEntry{{Name: "one.txt", Path: "/one.txt"}, {Name: "two.txt", Path: "/two.txt"}}
	first.applyEntries(entries)
	second.applyEntries(entries)

	if !first.toggleSelection("/one.txt") || !first.toggleSelection("/two.txt") {
		t.Fatal("first browser should select both files")
	}
	if got := first.selectedPaths(); !reflect.DeepEqual(got, []string{"/one.txt", "/two.txt"}) {
		t.Fatalf("first selection = %v", got)
	}
	if got := second.selectedPaths(); len(got) != 0 {
		t.Fatalf("second browser inherited selection %v", got)
	}
	first.applyEntries(entries[:1])
	if got := first.selectedPaths(); !reflect.DeepEqual(got, []string{"/one.txt"}) {
		t.Fatalf("selection after refresh = %v", got)
	}
}

func TestTransferQueueHonorsConcurrencyPauseResumeAndRetry(t *testing.T) {
	queue := newTransferQueue(2)
	first := queue.enqueue(transferUpload, "host-1", "C:/one.bin", "/one.bin", 100)
	second := queue.enqueue(transferDownload, "host-1", "/two.bin", "C:/two.bin", 200)
	third := queue.enqueue(transferUpload, "host-2", "C:/three.bin", "/three.bin", 300)

	if first.Status != transferRunning || second.Status != transferRunning || third.Status != transferQueued {
		t.Fatalf("initial statuses = %s, %s, %s", first.Status, second.Status, third.Status)
	}
	if !queue.pause(first.ID) || first.Status != transferPaused || third.Status != transferRunning {
		t.Fatalf("pause statuses = first %s third %s", first.Status, third.Status)
	}
	if !queue.resume(first.ID) || first.Status != transferQueued {
		t.Fatalf("resume status = %s", first.Status)
	}
	if !queue.updateProgress(second.ID, 250) || second.Transferred != second.Size {
		t.Fatalf("clamped progress = %d/%d", second.Transferred, second.Size)
	}
	if !queue.complete(second.ID) || first.Status != transferRunning {
		t.Fatalf("complete second = second %s first %s", second.Status, first.Status)
	}
	if !queue.fail(first.ID, "network reset") || first.Status != transferFailed || first.Error != "network reset" {
		t.Fatalf("failed transfer = %+v", first)
	}
	if !queue.retry(first.ID) || first.Status != transferRunning || first.Error != "" || first.Attempts != 2 {
		t.Fatalf("retried transfer = %+v", first)
	}
}
