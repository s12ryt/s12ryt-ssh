package gui

import (
	"context"
	"testing"
	"time"

	"s12ryt-ssh/internal/remote"
)

type keyIdentityRemoteSession struct {
	fakeRemoteSession
	keys []remote.SSHKeyIdentity
}

type keyIdentityCRUDRemoteSession struct {
	keyIdentityRemoteSession
	createdInput remote.SSHKeyIdentityInput
	updatedID    string
	updatedInput remote.SSHKeyIdentityInput
	deletedID    string
	created      remote.SSHKeyIdentity
	updated      remote.SSHKeyIdentity
}

func (s *keyIdentityRemoteSession) SSHKeyIdentities(context.Context) ([]remote.SSHKeyIdentity, error) {
	return append([]remote.SSHKeyIdentity(nil), s.keys...), nil
}

func (s *keyIdentityCRUDRemoteSession) CreateSSHKeyIdentity(_ context.Context, input remote.SSHKeyIdentityInput) (remote.SSHKeyIdentity, error) {
	s.createdInput = input
	s.created = remote.SSHKeyIdentity{ID: "key-created", Name: input.Name, PublicKey: input.PublicKey, Fingerprint: input.Fingerprint, Enabled: input.Enabled == nil || *input.Enabled, Version: 1}
	return s.created, nil
}

func (s *keyIdentityCRUDRemoteSession) UpdateSSHKeyIdentity(_ context.Context, id string, input remote.SSHKeyIdentityInput) (remote.SSHKeyIdentity, error) {
	s.updatedID = id
	s.updatedInput = input
	s.updated = remote.SSHKeyIdentity{ID: id, Name: input.Name, PublicKey: input.PublicKey, Fingerprint: input.Fingerprint, Enabled: input.Enabled == nil || *input.Enabled, Version: 2}
	return s.updated, nil
}

func (s *keyIdentityCRUDRemoteSession) DeleteSSHKeyIdentity(_ context.Context, id string) error {
	s.deletedID = id
	return nil
}

func testSSHKeyIdentity(id, name string) remote.SSHKeyIdentity {
	return remote.SSHKeyIdentity{
		ID: id, Name: name, PublicKey: "ssh-ed25519 public-" + id,
		Fingerprint: "SHA256:" + id, HasPassphrase: true, Enabled: true, Version: 1,
	}
}

func TestFilterSSHKeyIdentitiesMatchesMetadataFields(t *testing.T) {
	keys := []remote.SSHKeyIdentity{
		testSSHKeyIdentity("key-1", "Production deploy"),
		{ID: "key-2", Name: "Staging", PublicKey: "ssh-rsa staging", Fingerprint: "SHA256:staging"},
	}

	for _, query := range []string{"production", "SSH-RSA", "STAGING", "sha256:key-1"} {
		filtered := filterSSHKeyIdentities(keys, query)
		if len(filtered) != 1 {
			t.Fatalf("query %q returned %+v", query, filtered)
		}
	}
	if filtered := filterSSHKeyIdentities(keys, ""); len(filtered) != 2 {
		t.Fatalf("empty query returned %+v", filtered)
	}
}

func TestSSHKeyIdentityStoreRefreshCopiesMetadataAndClearsRemoved(t *testing.T) {
	store := newSSHKeyIdentityStore()
	keys := []remote.SSHKeyIdentity{testSSHKeyIdentity("key-1", "one")}
	store.replace(keys)
	keys[0].Name = "mutated"
	keys[0].PublicKey = "mutated"

	snapshot := store.snapshot()
	if len(snapshot) != 1 || snapshot[0].Key.Name != "one" || snapshot[0].Key.PublicKey == "mutated" {
		t.Fatalf("store did not copy metadata: %+v", snapshot)
	}

	store.replace(nil)
	if snapshot := store.snapshot(); len(snapshot) != 0 {
		t.Fatalf("store retained removed keys: %+v", snapshot)
	}
}

func TestSSHKeyIdentityStoreUpsertsAndRemovesByStableID(t *testing.T) {
	store := newSSHKeyIdentityStore()
	store.replace([]remote.SSHKeyIdentity{testSSHKeyIdentity("key-1", "one")})
	store.upsert(remote.SSHKeyIdentity{ID: "key-1", Name: "updated", Enabled: false})
	store.upsert(remote.SSHKeyIdentity{ID: "key-2", Name: "two", Enabled: true})

	entries := store.snapshot()
	if len(entries) != 2 || entries[0].Key.Name != "updated" || entries[1].Key.ID != "key-2" {
		t.Fatalf("upserted entries = %+v", entries)
	}
	if !store.remove("key-1") || store.remove("missing") {
		t.Fatal("remove returned incorrect result")
	}
	entries = store.snapshot()
	if len(entries) != 1 || entries[0].Key.ID != "key-2" {
		t.Fatalf("remaining entries = %+v", entries)
	}
}

func TestRefreshSSHKeyIdentitiesLoadsMetadataWithoutPrivateMaterial(t *testing.T) {
	ui := NewWindow(nil)
	session := &keyIdentityRemoteSession{
		keys: []remote.SSHKeyIdentity{testSSHKeyIdentity("key-1", "production")},
	}
	ui.model.SetRemoteSession(session, true)
	if !ui.refreshSSHKeyIdentities() {
		t.Fatal("refresh key identities was not started")
	}
	if !waitForKeyIdentityUIEvent(ui, 2*time.Second) {
		t.Fatal("key identity refresh did not complete")
	}
	entries := ui.sshKeys.snapshot()
	if len(entries) != 1 || entries[0].Key.ID != "key-1" || entries[0].Key.Name != "production" {
		t.Fatalf("key identity entries = %+v", entries)
	}
}

func waitForKeyIdentityUIEvent(ui *Window, timeout time.Duration) bool {
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

func TestSSHKeyIdentityFormBuildsInputAndPreservesExistingSecrets(t *testing.T) {
	values := sshKeyIdentityFormValues{
		ID:                  "key-1",
		Name:                "  production  ",
		PublicKey:           "ssh-ed25519 AAAA",
		Fingerprint:         "SHA256:production",
		PrivateKey:          "",
		KeyPassphrase:       "",
		ClearSecretMaterial: false,
		Enabled:             false,
	}

	input, err := values.input()
	if err != nil {
		t.Fatalf("input() error = %v", err)
	}
	if input.Name != "production" || input.PublicKey != "ssh-ed25519 AAAA" || input.Fingerprint != "SHA256:production" {
		t.Fatalf("input metadata = %+v", input)
	}
	if input.PrivateKey != "" || input.KeyPassphrase != "" {
		t.Fatalf("blank secret fields should preserve remote material: %+v", input)
	}
	if input.Enabled == nil || *input.Enabled {
		t.Fatalf("input.Enabled = %v, want explicit false", input.Enabled)
	}

	values.ClearSecretMaterial = true
	input, err = values.input()
	if err != nil {
		t.Fatalf("input() with clear error = %v", err)
	}
	if input.PrivateKey != "" || input.KeyPassphrase != "" || !input.ClearSecretMaterial {
		t.Fatalf("clear secret input = %+v", input)
	}
}

func TestSSHKeyIdentityFormRejectsMissingRequiredValues(t *testing.T) {
	base := sshKeyIdentityFormValues{
		Name:       "production",
		PrivateKey: "private-key",
		Enabled:    true,
	}
	for name, values := range map[string]sshKeyIdentityFormValues{
		"missing name":        {PrivateKey: base.PrivateKey, Enabled: base.Enabled},
		"missing private key": {Name: base.Name, Enabled: base.Enabled},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := values.input(); err == nil {
				t.Fatal("input() unexpectedly accepted invalid values")
			}
		})
	}
}

func TestOpenSSHKeyIdentityFormLoadsMetadataWithoutPrivateMaterial(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshKeys.replace([]remote.SSHKeyIdentity{testSSHKeyIdentity("key-1", "production")})
	if !ui.openSSHKeyIdentityForm("key-1") {
		t.Fatal("edit key form did not open")
	}
	if ui.sshKeyForm.Name != "production" || ui.sshKeyForm.PrivateKey != "" || ui.sshKeyForm.KeyPassphrase != "" {
		t.Fatalf("key form exposed private material: %+v", ui.sshKeyForm)
	}
	ui.closeSSHKeyIdentityForm()
	if ui.sshKeyFormOpen || ui.sshKeyFormID != "" || ui.sshKeyForm.Name != "" {
		t.Fatalf("key form state was not cleared: open=%v id=%q values=%+v", ui.sshKeyFormOpen, ui.sshKeyFormID, ui.sshKeyForm)
	}
}

func TestSubmitSSHKeyIdentityFormUsesRemoteCreateAndUpdatePayloads(t *testing.T) {
	ui := NewWindow(nil)
	session := &keyIdentityCRUDRemoteSession{}
	ui.model.SetRemoteSession(session, true)
	if !ui.openSSHKeyIdentityForm("") {
		t.Fatal("new key form was not opened")
	}
	ui.sshKeyName.SetText("production")
	ui.sshKeyPublicKey.SetText("ssh-ed25519 AAAA")
	ui.sshKeyFingerprint.SetText("SHA256:production")
	ui.sshKeyPrivateKey.SetText("private-material")
	ui.sshKeyPassphrase.SetText("passphrase")
	ui.sshKeyEnabled.Value = false
	if !ui.submitSSHKeyIdentityForm() {
		t.Fatal("key create was not started")
	}
	if !waitForKeyIdentityUIEvent(ui, 2*time.Second) {
		t.Fatal("key create did not complete")
	}
	if session.createdInput.Name != "production" || session.createdInput.PrivateKey != "private-material" || session.createdInput.KeyPassphrase != "passphrase" {
		t.Fatalf("create payload = %+v", session.createdInput)
	}
	if session.createdInput.Enabled == nil || *session.createdInput.Enabled {
		t.Fatalf("create enabled payload = %v", session.createdInput.Enabled)
	}
	if ui.sshKeyFormOpen {
		t.Fatal("successful key create left the form open")
	}

	ui.sshKeys.replace([]remote.SSHKeyIdentity{testSSHKeyIdentity("key-1", "production")})
	if !ui.openSSHKeyIdentityForm("key-1") {
		t.Fatal("edit key form was not opened")
	}
	ui.sshKeyName.SetText("production-updated")
	ui.sshKeyPrivateKey.SetText("")
	ui.sshKeyPassphrase.SetText("")
	ui.sshKeyEnabled.Value = true
	if !ui.submitSSHKeyIdentityForm() {
		t.Fatal("key update was not started")
	}
	if !waitForKeyIdentityUIEvent(ui, 2*time.Second) {
		t.Fatal("key update did not complete")
	}
	if session.updatedID != "key-1" || session.updatedInput.Name != "production-updated" || session.updatedInput.ClearSecretMaterial {
		t.Fatalf("update payload = id:%q input:%+v", session.updatedID, session.updatedInput)
	}
	if session.updatedInput.PrivateKey != "" || session.updatedInput.KeyPassphrase != "" {
		t.Fatalf("blank secret fields must preserve remote material: %+v", session.updatedInput)
	}
}

func TestDeleteSSHKeyIdentityRequiresConfirmationBeforeRemoteCall(t *testing.T) {
	ui := NewWindow(nil)
	session := &keyIdentityCRUDRemoteSession{}
	ui.model.SetRemoteSession(session, true)
	ui.sshKeys.replace([]remote.SSHKeyIdentity{testSSHKeyIdentity("key-1", "production")})
	if !ui.deleteSSHKeyIdentity("key-1") {
		t.Fatal("key delete confirmation was not opened")
	}
	if !ui.confirm.active || session.deletedID != "" {
		t.Fatalf("delete state = confirm:%v deleted:%q", ui.confirm.active, session.deletedID)
	}
	ui.confirm.accept()
	if !waitForKeyIdentityUIEvent(ui, 2*time.Second) {
		t.Fatal("key delete did not complete")
	}
	if session.deletedID != "key-1" || len(ui.sshKeys.snapshot()) != 0 {
		t.Fatalf("delete result = id:%q keys:%+v", session.deletedID, ui.sshKeys.snapshot())
	}
}

func TestSSHKeyIdentityManagementSourcesExposeCreateEditDelete(t *testing.T) {
	want := []string{"New key", "Edit key", "Delete key?"}
	got := sshKeyIdentityManagementSources()
	if len(got) != len(want) {
		t.Fatalf("management sources = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("management source[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestSyncSSHKeyIdentityButtonsFollowsVisibleIDs(t *testing.T) {
	ui := NewWindow(nil)
	ui.syncSSHKeyIdentityButtons([]sshKeyIdentityEntry{
		{Key: testSSHKeyIdentity("one", "One")},
		{Key: testSSHKeyIdentity("two", "Two")},
	})
	if len(ui.sshKeyVisibleIDs) != 2 || len(ui.sshKeyEditBtns) != 2 || len(ui.sshKeyDeleteBtns) != 2 {
		t.Fatalf("key button counts = ids:%d edit:%d delete:%d", len(ui.sshKeyVisibleIDs), len(ui.sshKeyEditBtns), len(ui.sshKeyDeleteBtns))
	}
	ui.syncSSHKeyIdentityButtons([]sshKeyIdentityEntry{{Key: testSSHKeyIdentity("two", "Two")}})
	if len(ui.sshKeyVisibleIDs) != 1 || ui.sshKeyVisibleIDs[0] != "two" || len(ui.sshKeyEditBtns) != 1 || len(ui.sshKeyDeleteBtns) != 1 {
		t.Fatalf("filtered key buttons = ids:%v edit:%d delete:%d", ui.sshKeyVisibleIDs, len(ui.sshKeyEditBtns), len(ui.sshKeyDeleteBtns))
	}
}

func TestClearSSHKeyIdentityViewDropsPreviousAccountMetadata(t *testing.T) {
	ui := NewWindow(nil)
	ui.sshKeys.replace([]remote.SSHKeyIdentity{testSSHKeyIdentity("key-1", "production")})
	ui.syncSSHKeyIdentityButtons(ui.sshKeys.snapshot())

	ui.clearSSHKeyIdentityView()
	if len(ui.sshKeys.snapshot()) != 0 || len(ui.sshKeyVisibleIDs) != 0 || len(ui.sshKeyEditBtns) != 0 || len(ui.sshKeyDeleteBtns) != 0 {
		t.Fatalf("key view retained previous account state: keys=%v ids=%v edit=%d delete=%d", ui.sshKeys.snapshot(), ui.sshKeyVisibleIDs, len(ui.sshKeyEditBtns), len(ui.sshKeyDeleteBtns))
	}
}

func TestFilterSSHKeyIdentityEntriesKeepsMetadataAndStableOrder(t *testing.T) {
	entries := []sshKeyIdentityEntry{
		{Key: testSSHKeyIdentity("one", "Production")},
		{Key: testSSHKeyIdentity("two", "Staging")},
	}

	filtered := filteredSSHKeyIdentityEntries(entries, "sha256:two")
	if len(filtered) != 1 || filtered[0].Key.ID != "two" || filtered[0].Key.Fingerprint != "SHA256:two" {
		t.Fatalf("filtered key entries = %+v", filtered)
	}
	if got := filteredSSHKeyIdentityEntries(entries, ""); len(got) != 2 || got[0].Key.ID != "one" || got[1].Key.ID != "two" {
		t.Fatalf("empty query changed key order = %+v", got)
	}
}
