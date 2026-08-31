package gui

import (
	"context"
	"errors"
	"strings"

	"gioui.org/widget"

	"s12ryt-ssh/internal/remote"
)

// remoteKeyIdentitySession is optional so older authenticated sessions remain
// usable while reusable key identities are introduced.
type remoteKeyIdentitySession interface {
	SSHKeyIdentities(context.Context) ([]remote.SSHKeyIdentity, error)
}

type remoteKeyIdentityCRUDSession interface {
	remoteKeyIdentitySession
	CreateSSHKeyIdentity(context.Context, remote.SSHKeyIdentityInput) (remote.SSHKeyIdentity, error)
	UpdateSSHKeyIdentity(context.Context, string, remote.SSHKeyIdentityInput) (remote.SSHKeyIdentity, error)
	DeleteSSHKeyIdentity(context.Context, string) error
}

func (ui *Window) refreshSSHKeyIdentities() bool {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteKeyIdentitySession)
	if !ok {
		ui.model.Error = "SSH key identity service is unavailable"
		return false
	}
	if ui.sshKeys == nil {
		ui.sshKeys = newSSHKeyIdentityStore()
	}
	if ui.busy {
		return false
	}
	ui.async("Loading SSH key identities...", func(ctx context.Context) (func(), error) {
		keys, err := session.SSHKeyIdentities(ctx)
		if err != nil {
			return nil, err
		}
		return func() { ui.sshKeys.replace(keys) }, nil
	})
	return true
}

func (ui *Window) syncSSHKeyIdentityButtons(entries []sshKeyIdentityEntry) {
	if ui == nil {
		return
	}
	ui.sshKeyVisibleIDs = make([]string, len(entries))
	ui.sshKeyEditBtns = make([]widget.Clickable, len(entries))
	ui.sshKeyDeleteBtns = make([]widget.Clickable, len(entries))
	for index, entry := range entries {
		ui.sshKeyVisibleIDs[index] = entry.Key.ID
	}
}

func (ui *Window) clearSSHKeyIdentityView() {
	if ui == nil {
		return
	}
	if ui.sshKeys != nil {
		ui.sshKeys.replace(nil)
	}
	ui.sshKeyVisibleIDs = nil
	ui.sshKeyEditBtns = nil
	ui.sshKeyDeleteBtns = nil
}

func (ui *Window) optionalKeyIdentitySession() (remoteKeyIdentitySession, error) {
	if ui == nil || ui.model == nil || ui.model.RemoteSession == nil {
		return nil, errors.New("remote session is not active")
	}
	session, ok := ui.model.RemoteSession.(remoteKeyIdentitySession)
	if !ok {
		return nil, errors.New("SSH key identity service is unavailable")
	}
	return session, nil
}

func (ui *Window) openSSHKeyIdentityForm(id string) bool {
	if ui == nil || ui.sshKeyFormOpen || ui.busy || ui.sshKeys == nil {
		return false
	}
	values := sshKeyIdentityFormValues{Enabled: true}
	if id == "" {
		ui.sshKeyFormOpen = true
		ui.sshKeyFormID = ""
		ui.sshKeyForm = values
		ui.setSSHKeyIdentityFormEditors(values)
		ui.model.Error = ""
		return true
	}
	for _, entry := range ui.sshKeys.snapshot() {
		if entry.Key.ID != id {
			continue
		}
		values = sshKeyIdentityFormValues{
			ID:            entry.Key.ID,
			Name:          entry.Key.Name,
			PublicKey:     entry.Key.PublicKey,
			Fingerprint:   entry.Key.Fingerprint,
			Enabled:       entry.Key.Enabled,
			HasPassphrase: entry.Key.HasPassphrase,
		}
		ui.sshKeyFormOpen = true
		ui.sshKeyFormID = id
		ui.sshKeyForm = values
		ui.setSSHKeyIdentityFormEditors(values)
		ui.model.Error = ""
		return true
	}
	return false
}

func (ui *Window) closeSSHKeyIdentityForm() {
	if ui == nil {
		return
	}
	ui.sshKeyFormOpen = false
	ui.sshKeyFormID = ""
	ui.sshKeyForm = sshKeyIdentityFormValues{}
	ui.sshKeyName.SetText("")
	ui.sshKeyPublicKey.SetText("")
	ui.sshKeyFingerprint.SetText("")
	ui.sshKeyPrivateKey.SetText("")
	ui.sshKeyPassphrase.SetText("")
	ui.sshKeyClearSecrets.Value = false
	ui.sshKeyEnabled.Value = false
	ui.sshKeyFormClose = widget.Clickable{}
	ui.sshKeyFormCancel = widget.Clickable{}
	ui.sshKeyFormSave = widget.Clickable{}
	ui.sshKeyFormDelete = widget.Clickable{}
	ui.sshKeyFormScrim = widget.Clickable{}
}

func (ui *Window) setSSHKeyIdentityFormEditors(values sshKeyIdentityFormValues) {
	ui.sshKeyName.SetText(values.Name)
	ui.sshKeyPublicKey.SetText(values.PublicKey)
	ui.sshKeyFingerprint.SetText(values.Fingerprint)
	ui.sshKeyPrivateKey.SetText("")
	ui.sshKeyPassphrase.SetText("")
	ui.sshKeyClearSecrets.Value = values.ClearSecretMaterial
	ui.sshKeyEnabled.Value = values.Enabled
}

func (ui *Window) currentSSHKeyIdentityForm() sshKeyIdentityFormValues {
	values := ui.sshKeyForm
	values.ID = ui.sshKeyFormID
	values.Name = ui.sshKeyName.Text()
	values.PublicKey = ui.sshKeyPublicKey.Text()
	values.Fingerprint = ui.sshKeyFingerprint.Text()
	values.PrivateKey = ui.sshKeyPrivateKey.Text()
	values.KeyPassphrase = ui.sshKeyPassphrase.Text()
	values.ClearSecretMaterial = ui.sshKeyClearSecrets.Value
	values.Enabled = ui.sshKeyEnabled.Value
	return values
}

func keyIdentityFormErrorSource(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	for _, source := range []string{"Key name is required.", "Private key is required."} {
		if strings.HasPrefix(message, source) {
			return source
		}
	}
	return message
}

func (ui *Window) submitSSHKeyIdentityForm() bool {
	if ui == nil || !ui.sshKeyFormOpen || ui.busy {
		return false
	}
	input, err := ui.currentSSHKeyIdentityForm().input()
	if err != nil {
		ui.model.Error = ui.text(keyIdentityFormErrorSource(err))
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteKeyIdentityCRUDSession)
	if !ok {
		ui.model.Error = "SSH key identity service is unavailable"
		return false
	}
	id := ui.sshKeyFormID
	ui.closeSSHKeyIdentityForm()
	ui.async(ui.text("Saving SSH key identity..."), func(ctx context.Context) (func(), error) {
		var key remote.SSHKeyIdentity
		if id == "" {
			key, err = session.CreateSSHKeyIdentity(ctx, input)
		} else {
			key, err = session.UpdateSSHKeyIdentity(ctx, id, input)
		}
		if err != nil {
			return nil, err
		}
		return func() {
			if ui.sshKeys == nil {
				ui.sshKeys = newSSHKeyIdentityStore()
			}
			ui.sshKeys.upsert(key)
		}, nil
	})
	return true
}

func (ui *Window) deleteSSHKeyIdentity(id string) bool {
	if ui == nil || ui.sshKeys == nil || ui.busy {
		return false
	}
	found := false
	for _, entry := range ui.sshKeys.snapshot() {
		if entry.Key.ID == id {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	session, ok := ui.model.RemoteSession.(remoteKeyIdentityCRUDSession)
	if !ok {
		ui.model.Error = "SSH key identity service is unavailable"
		return false
	}
	ui.requestConfirm(ui.text("Delete key?"), ui.text("This key identity will be permanently deleted."), func() {
		ui.async(ui.text("Deleting SSH key identity..."), func(ctx context.Context) (func(), error) {
			if err := session.DeleteSSHKeyIdentity(ctx, id); err != nil {
				return nil, err
			}
			return func() { ui.sshKeys.remove(id) }, nil
		})
	})
	return true
}
