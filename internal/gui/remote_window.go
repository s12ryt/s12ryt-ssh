package gui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"s12ryt-ssh/internal/remote"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (ui *Window) handleRemoteLogin(gtx layout.Context) {
	if ui.remoteBack.Clicked(gtx) {
		ui.remotePassword.SetText("")
		ui.model.CancelRemoteLogin()
		return
	}
	if ui.drainEditors(gtx, &ui.remoteURL, &ui.remoteUsername, &ui.remotePassword) {
		ui.tryRemoteSignIn()
		return
	}
	if ui.remoteLogin.Clicked(gtx) {
		ui.tryRemoteSignIn()
	}
	if ui.remoteRestore.Clicked(gtx) {
		if ui.model.RemoteService == nil {
			ui.model.Error = "Remote authentication service is unavailable"
			return
		}
		service := ui.model.RemoteService
		ui.async("Restoring remote session...", func(ctx context.Context) (func(), error) {
			session, err := service.Restore(ctx)
			if err != nil {
				return nil, err
			}
			resources, err := session.Resources(ctx)
			if err != nil {
				_ = session.Logout(ctx)
				return nil, err
			}
			return func() { ui.activateRemoteSession(session, resources) }, nil
		})
	}
}

func (ui *Window) activateRemoteSession(session RemoteSession, resources []remote.Resource) {
	ui.model.SetRemoteSession(session)
	ui.remoteResources = append([]remote.Resource(nil), resources...)
	ui.remoteResourceButtons = make([]widget.Clickable, len(resources))
	ui.remoteIndex = -1
	ui.selectFirstRemoteResource()
	ui.remoteObjects = nil
	ui.remoteObjectButtons = nil
	ui.storageText = ""
	ui.databaseText = ""
}

func (ui *Window) selectFirstRemoteResource() {
	indices := ui.remoteResourceIndices(ui.model.Tab)
	if len(indices) == 0 {
		ui.remoteIndex = -1
		return
	}
	ui.remoteIndex = indices[0]
}

func (ui *Window) handleRemoteWorkspace(gtx layout.Context) {
	if ui.logout.Clicked(gtx) {
		session := ui.model.RemoteSession
		ui.asyncAlways("Signing out...", func(ctx context.Context) (func(), error) {
			var err error
			if session != nil {
				err = session.Logout(ctx)
			}
			return func() {
				ui.model.finishRemoteLogout()
				ui.remoteResources = nil
				ui.remoteResourceButtons = nil
				ui.remoteIndex = -1
			}, err
		})
		return
	}
	if ui.storageTab.Clicked(gtx) {
		ui.model.SelectTab(TabStorage)
		ui.selectFirstRemoteResource()
	}
	if ui.databaseTab.Clicked(gtx) {
		ui.model.SelectTab(TabDatabase)
		ui.selectFirstRemoteResource()
	}
	for _, index := range ui.remoteResourceIndices(ui.model.Tab) {
		if index < len(ui.remoteResourceButtons) && ui.remoteResourceButtons[index].Clicked(gtx) {
			ui.remoteIndex = index
		}
	}
	if ui.remoteRefresh.Clicked(gtx) {
		ui.refreshRemoteResources()
	}
	switch ui.model.Tab {
	case TabStorage:
		ui.handleRemoteStorage(gtx)
	case TabDatabase:
		ui.handleRemoteDatabase(gtx)
	}
}

func (ui *Window) refreshRemoteResources() {
	session := ui.model.RemoteSession
	if session == nil {
		return
	}
	ui.async("Loading assigned connections...", func(ctx context.Context) (func(), error) {
		resources, err := session.Resources(ctx)
		if err != nil {
			return nil, err
		}
		return func() {
			ui.remoteResources = append([]remote.Resource(nil), resources...)
			ui.remoteResourceButtons = make([]widget.Clickable, len(resources))
			ui.selectFirstRemoteResource()
		}, nil
	})
}

func (ui *Window) selectedRemoteResource(operation remote.Operation) (RemoteSession, remote.Resource, error) {
	if ui.model.RemoteSession == nil || ui.remoteIndex < 0 || ui.remoteIndex >= len(ui.remoteResources) {
		return nil, remote.Resource{}, fmt.Errorf("No connection selected")
	}
	resource := ui.remoteResources[ui.remoteIndex]
	if !resource.Enabled || !resource.Allows(operation) {
		return nil, remote.Resource{}, fmt.Errorf("Permission not granted for this operation")
	}
	return ui.model.RemoteSession, resource, nil
}

func (ui *Window) handleRemoteStorage(gtx layout.Context) {
	for i := range ui.remoteObjectButtons {
		if ui.remoteObjectButtons[i].Clicked(gtx) {
			ui.selectRemoteObject(i)
		}
	}
	ui.drainEditors(gtx, &ui.storagePrefix, &ui.storageKey, &ui.storagePath, &ui.storageData)
	if ui.storageRefresh.Clicked(gtx) {
		session, resource, err := ui.selectedRemoteResource(remote.OperationS3Read)
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		prefix := ui.storagePrefix.Text()
		ui.async("Listing remote objects...", func(ctx context.Context) (func(), error) {
			objects, err := session.ListObjects(ctx, resource.ID, prefix)
			return func() {
				ui.remoteObjects = objects
				ui.storageText = ui.formatRemoteObjects(objects)
			}, err
		})
	}
	if ui.storageUpload.Clicked(gtx) {
		if err := requireObjectKey(ui.storageKey.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		session, resource, err := ui.selectedRemoteResource(remote.OperationS3Write)
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		key, path, inline := ui.storageKey.Text(), strings.TrimSpace(ui.storagePath.Text()), ui.storageData.Text()
		ui.async("Uploading object...", func(ctx context.Context) (func(), error) {
			var body io.ReadSeeker
			var length int64
			if path != "" {
				file, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				defer file.Close()
				info, err := file.Stat()
				if err != nil {
					return nil, err
				}
				body, length = file, info.Size()
			} else {
				reader := strings.NewReader(inline)
				body, length = reader, int64(reader.Len())
			}
			result, err := session.UploadObject(ctx, resource.ID, key, body, length)
			return func() { ui.storageText = fmt.Sprintf("%s%d %s", ui.text("Uploaded "), result.Bytes, ui.text("Bytes")) }, err
		})
	}
	if ui.storageDownload.Clicked(gtx) {
		if err := requireObjectKey(ui.storageKey.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		session, resource, err := ui.selectedRemoteResource(remote.OperationS3Read)
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		key, path := ui.storageKey.Text(), strings.TrimSpace(ui.storagePath.Text())
		ui.async("Downloading object...", func(ctx context.Context) (func(), error) {
			download, err := session.DownloadObject(ctx, resource.ID, key)
			if err != nil {
				return nil, err
			}
			defer download.Body.Close()
			if path != "" {
				file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
				if err != nil {
					return nil, err
				}
				written, copyErr := io.Copy(file, download.Body)
				closeErr := file.Close()
				if copyErr != nil {
					return nil, copyErr
				}
				if closeErr != nil {
					return nil, closeErr
				}
				return func() {
					ui.storageText = fmt.Sprintf("%s%d %s%s", ui.text("Downloaded "), written, ui.text("Bytes"), ui.downloadedTo(path))
				}, nil
			}
			data, err := io.ReadAll(download.Body)
			if err != nil {
				return nil, err
			}
			return func() {
				ui.storageText = fmt.Sprintf("%s%d %s\n\n%s", ui.text("Downloaded "), len(data), ui.text("Bytes"), string(data))
			}, nil
		})
	}
	if ui.storageDelete.Clicked(gtx) {
		if err := requireObjectKey(ui.storageKey.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		session, resource, err := ui.selectedRemoteResource(remote.OperationS3Delete)
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		key := ui.storageKey.Text()
		ui.requestConfirm("Delete object", "This permanently deletes the object. This action cannot be undone.", func() {
			ui.async("Deleting object...", func(ctx context.Context) (func(), error) {
				return nil, session.DeleteObject(ctx, resource.ID, key)
			})
		})
	}
}

func (ui *Window) handleRemoteDatabase(gtx layout.Context) {
	ui.drainEditors(gtx, &ui.databaseSQL)
	if ui.databaseTables.Clicked(gtx) {
		session, resource, err := ui.selectedRemoteResource(remote.OperationSQLTables)
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		ui.async("Loading database tables...", func(ctx context.Context) (func(), error) {
			tables, err := session.Tables(ctx, resource.ID)
			return func() { ui.databaseText = strings.Join(tables, "\n") }, err
		})
	}
	if ui.databaseQuery.Clicked(gtx) {
		if err := requireSQLStatement(ui.databaseSQL.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		session, resource, err := ui.selectedRemoteResource(remote.OperationSQLQuery)
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		statement := ui.databaseSQL.Text()
		ui.async("Running database query...", func(ctx context.Context) (func(), error) {
			result, err := session.Query(ctx, resource.ID, statement, nil)
			return func() { ui.databaseText = ui.formatRemoteRows(result) }, err
		})
	}
	if ui.databaseExec.Clicked(gtx) {
		if err := requireSQLStatement(ui.databaseSQL.Text()); err != nil {
			ui.model.Error = err.Error()
			return
		}
		session, resource, err := ui.selectedRemoteResource(remote.OperationSQLExec)
		if err != nil {
			ui.model.Error = err.Error()
			return
		}
		statement := ui.databaseSQL.Text()
		ui.requestConfirm("Execute SQL statement", "This runs a statement that can modify data. Continue?", func() {
			ui.async("Executing database statement...", func(ctx context.Context) (func(), error) {
				result, err := session.Exec(ctx, resource.ID, statement, nil)
				return func() {
					ui.databaseText = fmt.Sprintf("%s%d\n%s%s", ui.text("Rows affected: "), result.RowsAffected, ui.text("Last insert ID: "), result.LastInsertID)
				}, err
			})
		})
	}
}

func (ui *Window) remoteLoginView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(36), Bottom: unit.Dp(36), Left: unit.Dp(42), Right: unit.Dp(42)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(14)}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.H5(ui.theme, ui.text("Sign in with authentication service")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Body2(ui.theme, ui.text("Use a complete HTTP or HTTPS URL. The password is never saved.")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.field(gtx, &ui.remoteURL, "Authentication service URL", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.field(gtx, &ui.remoteUsername, "Account", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.field(gtx, &ui.remotePassword, "Password", true, true)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.actionButton(gtx, &ui.remoteLogin, "Sign in remotely", true, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.button(gtx, &ui.remoteRestore, "Restore saved session", false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.button(gtx, &ui.remoteBack, "Back", false) }),
				)
			})
		})
	})
}

func (ui *Window) remoteWorkspaceView(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(12)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.remoteTabs(gtx) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(ui.theme, ui.text("Remote account: ")+ui.model.RemoteAccountName).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.status(gtx) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(12)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.remoteSidebar(gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if ui.model.Tab == TabStorage {
						return ui.remoteStorageView(gtx)
					}
					return ui.remoteDatabaseView(gtx)
				}),
			)
		}),
	)
}

func (ui *Window) remoteTabs(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.storageTab, "S3 / R2", ui.model.Tab == TabStorage)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.button(gtx, &ui.databaseTab, "SQL database", ui.model.Tab == TabDatabase)
		}),
	)
}

func (ui *Window) remoteSidebar(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(230)
		gtx.Constraints.Max.X = gtx.Dp(270)
		return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(14), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			indices := ui.remoteResourceIndices(ui.model.Tab)
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text("Assigned connections")).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(indices) == 0 {
						return material.Body2(ui.theme, ui.text("No assigned connections.")).Layout(gtx)
					}
					return ui.remoteList.Layout(gtx, len(indices), func(gtx layout.Context, listIndex int) layout.Dimensions {
						resourceIndex := indices[listIndex]
						return ui.button(gtx, &ui.remoteResourceButtons[resourceIndex], ui.remoteResources[resourceIndex].Name, ui.remoteIndex == resourceIndex)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.button(gtx, &ui.remoteRefresh, "Refresh list", false)
				}),
			)
		})
	})
}

func (ui *Window) remoteStorageView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text("Assigned S3 / R2")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "List prefix", &ui.storagePrefix, "Object key", &ui.storageKey)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.editorRow(gtx, "Local path", &ui.storagePath, "Inline upload data", &ui.storageData)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.remoteActionRow(gtx, []remoteAction{
						{&ui.storageRefresh, "Refresh list", remote.OperationS3Read, false, false},
						{&ui.storageUpload, "Upload", remote.OperationS3Write, false, false},
						{&ui.storageDownload, "Download", remote.OperationS3Read, true, false},
						{&ui.storageDelete, "Delete", remote.OperationS3Delete, false, true},
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.remoteObjectBrowser(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.outputList(gtx, &ui.storageOutputList, ui.storageText, "Remote objects and operation output", false)
						}),
					)
				}),
			)
		})
	})
}

func (ui *Window) remoteDatabaseView(gtx layout.Context) layout.Dimensions {
	return ui.surface(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Subtitle1(ui.theme, ui.text("Assigned SQL database")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.field(gtx, &ui.databaseSQL, "SQL query or statement", false, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.remoteActionRow(gtx, []remoteAction{
						{&ui.databaseTables, "List tables", remote.OperationSQLTables, false, false},
						{&ui.databaseQuery, "Run query", remote.OperationSQLQuery, true, false},
						{&ui.databaseExec, "Run exec", remote.OperationSQLExec, false, true},
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.outputList(gtx, &ui.databaseOutputList, ui.databaseText, "Remote database output", true)
				}),
			)
		})
	})
}

type remoteAction struct {
	button    *widget.Clickable
	label     string
	operation remote.Operation
	primary   bool
	danger    bool
}

func (ui *Window) remoteActionRow(gtx layout.Context, actions []remoteAction) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(actions))
	for _, action := range actions {
		action := action
		if !ui.remoteAllows(action.operation) {
			continue
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.actionButton(gtx, action.button, action.label, action.primary, action.danger)
		}))
	}
	if len(children) == 0 {
		return material.Body2(ui.theme, ui.text("Permission not granted for this operation")).Layout(gtx)
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(8)}.Layout(gtx, children...)
}

// remoteObjectBrowser renders the clickable listing of remote S3 objects.
func (ui *Window) remoteObjectBrowser(gtx layout.Context) layout.Dimensions {
	ui.ensureRemoteObjectButtons()
	if len(ui.remoteObjects) == 0 {
		muted := material.Body2(ui.theme, ui.objectsHeader(0))
		muted.Color = colorMuted
		return muted.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(4)}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			header := material.Body2(ui.theme, ui.objectsHeader(len(ui.remoteObjects)))
			header.Color = colorMuted
			return header.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.remoteObjectList.Layout(gtx, len(ui.remoteObjects), func(gtx layout.Context, index int) layout.Dimensions {
				label := fmt.Sprintf("%s (%d %s)", ui.remoteObjects[index].Key, ui.remoteObjects[index].Size, ui.text("Bytes"))
				return ui.button(gtx, &ui.remoteObjectButtons[index], label, false)
			})
		}),
	)
}

func (ui *Window) formatRemoteObjects(objects []remote.S3Object) string {
	if len(objects) == 0 {
		return ui.text("No objects found.")
	}
	var b strings.Builder
	for _, object := range objects {
		fmt.Fprintf(&b, "%s  (%d %s)\n", object.Key, object.Size, ui.text("Bytes"))
	}
	return b.String()
}

func (ui *Window) formatRemoteRows(result remote.SQLQueryResult) string {
	if len(result.Rows) == 0 {
		return ui.text("No rows returned.")
	}
	return formatRemoteRows(result)
}

func validateRemoteCredentials(rawURL, username, password string) error {
	if strings.TrimSpace(rawURL) == "" || strings.TrimSpace(username) == "" || password == "" {
		return fmt.Errorf("Remote sign-in URL, account, and password are required")
	}
	return nil
}
