package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"s12ryt-ssh/internal/config"
	"s12ryt-ssh/internal/remote"
	sshclient "s12ryt-ssh/internal/ssh"
)

// sshFingerprintPattern extracts the host key fingerprint that the SSH client
// reports inside the trailing parentheses of ErrHostKeyNotTrusted.
var sshFingerprintPattern = regexp.MustCompile(`\(([^()]+)\)\s*$`)

// sshProfileFromCredentials converts issued remote credentials into the SSH
// client profile shape. Credentials never touch disk and live in memory only.
func sshProfileFromCredentials(creds remote.SSHHostCredentials) config.SSHProfile {
	return config.SSHProfile{
		Name:               creds.Name,
		Host:               creds.Host,
		Port:               creds.Port,
		User:               creds.Username,
		Password:           creds.Password,
		KeyData:            creds.PrivateKey,
		KeyPassphrase:      creds.KeyPassphrase,
		HostKeyFingerprint: creds.TrustedFingerprint,
	}
}

// parsePendingFingerprint reports whether err is a first-connection host key
// rejection and extracts the actual fingerprint that awaits user trust.
func parsePendingFingerprint(err error) (string, bool) {
	if err == nil || !errors.Is(err, sshclient.ErrHostKeyNotTrusted) {
		return "", false
	}
	match := sshFingerprintPattern.FindStringSubmatch(err.Error())
	if len(match) < 2 {
		return "", false
	}
	fingerprint := strings.TrimSpace(match[1])
	if fingerprint == "" {
		return "", false
	}
	return fingerprint, true
}

// applySSHHosts stores the account host list and keeps the selected host
// attached when the refreshed list still contains it.
func (ui *Window) applySSHHosts(hosts []remote.SSHHost) {
	ui.sshHosts = append([]remote.SSHHost(nil), hosts...)
	ui.sshHostButtons = make([]widget.Clickable, len(ui.sshHosts))
	ui.sshHostIndex = -1
	for i := range ui.sshHosts {
		if ui.sshHosts[i].ID == ui.sshHostID {
			ui.sshHostIndex = i
			break
		}
	}
}

func (ui *Window) selectSSHHost(index int) {
	if index < 0 || index >= len(ui.sshHosts) {
		return
	}
	ui.sshHostIndex = index
	ui.sshHostID = ui.sshHosts[index].ID
	ui.loadSSHHostForm(ui.sshHosts[index])
}

// loadSSHHostForm fills the editor fields from a stored host. Secret fields
// stay empty: leaving them blank preserves the stored credentials.
func (ui *Window) loadSSHHostForm(host remote.SSHHost) {
	ui.sshName.SetText(host.Name)
	ui.sshHost.SetText(host.Host)
	ui.sshPort.SetText(strconv.Itoa(host.Port))
	ui.sshUser.SetText(host.Username)
	ui.sshPassword.SetText("")
	ui.sshPrivateKey.SetText("")
	ui.sshKeyPass.SetText("")
	ui.sshFingerprint.SetText(host.TrustedFingerprint)
}

func (ui *Window) clearSSHHostForm() {
	ui.sshHostIndex = -1
	ui.sshHostID = ""
	ui.sshName.SetText("")
	ui.sshHost.SetText("")
	ui.sshPort.SetText("22")
	ui.sshUser.SetText("")
	ui.sshPassword.SetText("")
	ui.sshPrivateKey.SetText("")
	ui.sshKeyPass.SetText("")
	ui.sshFingerprint.SetText("")
}

// sshHostInputFromForm validates the form. A new host requires a password or
// private key; editing an existing host may leave both blank to keep the
// stored secret unchanged.
func (ui *Window) sshHostInputFromForm() (remote.SSHHostInput, error) {
	name := strings.TrimSpace(ui.sshName.Text())
	host := strings.TrimSpace(ui.sshHost.Text())
	user := strings.TrimSpace(ui.sshUser.Text())
	if name == "" {
		return remote.SSHHostInput{}, errors.New("name is required")
	}
	if host == "" {
		return remote.SSHHostInput{}, errors.New("host is required")
	}
	if user == "" {
		return remote.SSHHostInput{}, errors.New("username is required")
	}
	port := 22
	if value := strings.TrimSpace(ui.sshPort.Text()); value != "" {
		parsed, err := parsePort(value)
		if err != nil {
			return remote.SSHHostInput{}, errors.New("port must be a number between 1 and 65535")
		}
		port = parsed
	}
	input := remote.SSHHostInput{
		Name:               name,
		Host:               host,
		Port:               port,
		Username:           user,
		Password:           ui.sshPassword.Text(),
		PrivateKey:         strings.TrimSpace(ui.sshPrivateKey.Text()),
		KeyPassphrase:      ui.sshKeyPass.Text(),
		TrustedFingerprint: strings.TrimSpace(ui.sshFingerprint.Text()),
	}
	if ui.sshHostID == "" && input.Password == "" && input.PrivateKey == "" {
		return remote.SSHHostInput{}, errors.New("password or private key is required")
	}
	return input, nil
}

func (ui *Window) refreshSSHHosts() {
	ui.async("Loading SSH hosts...", func(ctx context.Context) (func(), error) {
		session := ui.model.RemoteSession
		if session == nil {
			return nil, errors.New("remote session is not active")
		}
		hosts, err := session.SSHHosts(ctx)
		if err != nil {
			return nil, err
		}
		return func() { ui.applySSHHosts(hosts) }, nil
	})
}

func (ui *Window) handleRemoteSSH(gtx layout.Context) {
	if ui.terminal == nil {
		if ui.drainEditors(gtx,
			&ui.sshName, &ui.sshHost, &ui.sshPort, &ui.sshUser,
			&ui.sshPassword, &ui.sshPrivateKey, &ui.sshKeyPass, &ui.sshFingerprint,
		) {
			return
		}
	} else if ui.drainEditors(gtx, &ui.terminalInput) {
		return
	}
	for i := range ui.sshHostButtons {
		if ui.sshHostButtons[i].Clicked(gtx) {
			ui.selectSSHHost(i)
			return
		}
	}
	if ui.sshNew.Clicked(gtx) {
		ui.clearSSHHostForm()
		return
	}
	if ui.sshSave.Clicked(gtx) {
		ui.saveSSHHost()
		return
	}
	if ui.sshDelete.Clicked(gtx) {
		ui.deleteSSHHost()
		return
	}
	if ui.sshConnect.Clicked(gtx) {
		ui.connectSSHHost()
		return
	}
	if ui.sshClose.Clicked(gtx) {
		ui.closeSSH()
		ui.model.Status = "SSH terminal closed."
		return
	}
	if ui.sshSend.Clicked(gtx) {
		ui.sendTerminalInput()
		return
	}
}

func (ui *Window) saveSSHHost() {
	if ui.busy {
		return
	}
	input, err := ui.sshHostInputFromForm()
	if err != nil {
		ui.model.Error = err.Error()
		return
	}
	hostID := ui.sshHostID
	ui.async("Saving SSH host...", func(ctx context.Context) (func(), error) {
		session := ui.model.RemoteSession
		if session == nil {
			return nil, errors.New("remote session is not active")
		}
		if hostID == "" {
			host, err := session.CreateSSHHost(ctx, input)
			if err != nil {
				return nil, err
			}
			return func() {
				ui.sshHostID = host.ID
				ui.refreshSSHHosts()
			}, nil
		}
		if _, err := session.UpdateSSHHost(ctx, hostID, input); err != nil {
			return nil, err
		}
		return func() { ui.refreshSSHHosts() }, nil
	})
}

func (ui *Window) deleteSSHHost() {
	if ui.busy || ui.sshHostID == "" {
		return
	}
	hostID := ui.sshHostID
	name := strings.TrimSpace(ui.sshName.Text())
	message := fmt.Sprintf("Delete %q? This cannot be undone.", name)
	ui.requestConfirm("Delete SSH host?", message, func() {
		ui.async("Deleting SSH host...", func(ctx context.Context) (func(), error) {
			session := ui.model.RemoteSession
			if session == nil {
				return nil, errors.New("remote session is not active")
			}
			if err := session.DeleteSSHHost(ctx, hostID); err != nil {
				return nil, err
			}
			return func() {
				ui.clearSSHHostForm()
				ui.refreshSSHHosts()
			}, nil
		})
	})
}

func (ui *Window) connectSSHHost() {
	if ui.busy {
		return
	}
	if ui.terminal != nil {
		ui.model.Error = "SSH terminal is already connected"
		return
	}
	hostID := ui.sshHostID
	if hostID == "" {
		ui.model.Error = "Select or save an SSH host first"
		return
	}
	size := ui.terminalSize
	ui.async("Connecting to SSH host...", func(ctx context.Context) (func(), error) {
		session := ui.model.RemoteSession
		if session == nil {
			return nil, errors.New("remote session is not active")
		}
		creds, err := session.SSHHostCredentials(ctx, hostID)
		if err != nil {
			return nil, err
		}
		client, term, err := dialSSHTerminal(creds, size)
		if err == nil {
			return func() { ui.attachSSHTerminal(client, term) }, nil
		}
		fingerprint, ok := parsePendingFingerprint(err)
		if !ok {
			return nil, err
		}
		return func() { ui.beginFingerprintConfirm(hostID, fingerprint, creds) }, nil
	})
}

// dialSSHTerminal dials the host directly from this client using credentials
// issued by the remote service. The PTY context is Background because the
// terminal outlives the async worker that spawned it.
func dialSSHTerminal(creds remote.SSHHostCredentials, size image.Point) (*sshclient.Client, ptyTerminal, error) {
	client := sshclient.NewClient(sshProfileFromCredentials(creds))
	if err := client.Connect(); err != nil {
		client.Close()
		return nil, nil, err
	}
	width, height := 100, 30
	if size.X > 0 {
		width = size.X
	}
	if size.Y > 0 {
		height = size.Y
	}
	term, err := client.OpenPTY(context.Background(), width, height)
	if err != nil {
		client.Close()
		return nil, nil, err
	}
	return client, term, nil
}

// beginFingerprintConfirm asks the user to trust the host key reported by the
// first connection attempt, persists it, then retries the connection.
func (ui *Window) beginFingerprintConfirm(hostID, fingerprint string, creds remote.SSHHostCredentials) {
	message := fmt.Sprintf("Host key fingerprint:\n%s\nTrust this key and connect?", fingerprint)
	ui.requestConfirm("Trust this host key?", message, func() {
		size := ui.terminalSize
		ui.async("Trusting host key...", func(ctx context.Context) (func(), error) {
			session := ui.model.RemoteSession
			if session == nil {
				return nil, errors.New("remote session is not active")
			}
			if err := session.SetSSHHostFingerprint(ctx, hostID, fingerprint); err != nil {
				return nil, err
			}
			creds.TrustedFingerprint = fingerprint
			client, term, err := dialSSHTerminal(creds, size)
			if err != nil {
				return nil, err
			}
			return func() { ui.attachSSHTerminal(client, term) }, nil
		})
	})
}

func (ui *Window) attachSSHTerminal(client *sshclient.Client, term ptyTerminal) {
	ui.closeSSH()
	ui.ssh = client
	ui.terminal = term
	ui.terminalCtx, ui.terminalCancel = context.WithCancel(context.Background())
	go ui.readTerminal(term)
	ui.model.Status = "SSH terminal connected."
}

// remoteSSHView renders the main SSH pane only. The host sidebar is drawn by
// remoteWorkspaceView; rendering it here as well duplicated the sidebar and
// broke click handling.
func (ui *Window) remoteSSHView(gtx layout.Context) layout.Dimensions {
	if ui.terminal != nil {
		return ui.remoteSSHTerminalView(gtx)
	}
	return ui.remoteSSHFormView(gtx)
}

func (ui *Window) remoteSSHSidebar(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(230)
		gtx.Constraints.Max.X = gtx.Dp(270)
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(rowGap)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text("SSH hosts")).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(ui.sshHosts) == 0 {
						return material.Body2(ui.theme, ui.text("No SSH hosts yet.")).Layout(gtx)
					}
					return ui.remoteList.Layout(gtx, len(ui.sshHosts), func(gtx layout.Context, index int) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return ui.button(gtx, &ui.sshHostButtons[index], ui.sshHosts[index].Name, ui.sshHostIndex == index)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.buttonBlock(gtx, &ui.sshNew, "New host", false)
				}),
			)
		})
	})
}

func (ui *Window) remoteSSHFormView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.sshFormList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(cardGap)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Subtitle1(ui.theme, ui.text("SSH host details")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.editorRow(gtx, "Name", &ui.sshName, "Host", &ui.sshHost)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.editorRow(gtx, "Port", &ui.sshPort, "Username", &ui.sshUser)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.labeledField(gtx, &ui.sshPassword, "Password", true, true)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = gtx.Dp(privateKeyMinHeight)
						return ui.labeledField(gtx, &ui.sshPrivateKey, "Private key", false, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.labeledField(gtx, &ui.sshKeyPass, "Key passphrase", true, true)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.labeledField(gtx, &ui.sshFingerprint, "Host fingerprint", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(rowGap)}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.button(gtx, &ui.sshSave, "Save host", true)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.actionButton(gtx, &ui.sshDelete, "Delete host", false, true)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.button(gtx, &ui.sshConnect, "Connect", false)
							}),
						)
					}),
				)
			})
		})
	})
}

func (ui *Window) remoteSSHTerminalView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(cardPadding), Bottom: unit.Dp(cardPadding), Left: unit.Dp(cardPadding), Right: unit.Dp(cardPadding)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(sectionGap)}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.outputList(gtx, &ui.terminalList, ui.terminalSnapshot(), "Terminal output will appear here", true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.editorField(gtx, &ui.terminalInput, "Terminal input")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.button(gtx, &ui.sshSend, "Send", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.actionButton(gtx, &ui.sshClose, "Close terminal", false, true)
						}),
					)
				}),
			)
		})
	})
}
