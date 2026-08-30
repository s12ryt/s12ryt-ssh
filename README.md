# s12ryt-ssh

Windows 優先的 Go 桌面 SSH 工作區，使用 Gio GUI。應用程式以遠端帳號登入服務為唯一入口，登入後提供自助管理的 SSH 主機與互動式 PTY 終端機。

## 功能

- 遠端登入唯一入口：啟動後輸入驗證服務 URL、帳號與密碼；密碼不會儲存。
- SSH 主機管理：常駐主機清單搭配新增/編輯彈出表單；支援密碼或私鑰（可加密碼短語）認證。
- 主機 key fingerprint 驗證（TOFU）：首次連線顯示指紋，確認後存回服務端。
- 分頁式互動 PTY 終端機；同一主機可同時開啟多個獨立連線，切換分頁不會中斷連線。連線由客戶端直連，不經服務端轉發。
- 帳號層級 SSH 開關：管理員關閉帳號 SSH 功能時，GUI 顯示「此帳號未啟用 SSH 存取」提示，仍可登出。
- 關閉視窗即結束進程：退出時同步撤銷遠端 session（逾時 2 秒），不殘留背景進程。
- GUI 支援英文與繁體中文；右上角「中 / EN」可即時切換，偏好保存於本機非敏感設定檔。

## 遠端登入

1. 啟動後輸入驗證服務 URL、帳號與密碼（密碼不會儲存）。
2. 服務端核發短期 access token 與輪換式 refresh token（以 Windows DPAPI 加密保存在本機）。
3. 之後啟動可一鍵還原 session。

驗證服務（Node.js + Fastify + SQLite + Telegram Bot 管理介面）位於獨立倉庫：https://github.com/s12ryt/s12ryt-ssh-auth-server

## SSH 主機

登入後（帳號需開啟 SSH 功能）可在工作區自助管理私人主機：

- 每帳號最多 50 台；名稱、主機、連接埠、使用者名稱必填。
- 點選主機會建立新的終端分頁；每個分頁各自保存連線、輸出與輸入狀態，連線失敗時可在原分頁重試或關閉。
- 新增、編輯與刪除主機在彈出表單內完成；關閉有未儲存內容的表單前會先確認。
- 認證支援密碼或私鑰（可加密碼短語），兩者至少其一；憑證以 AES-256-GCM 加密保存在服務端，連線時經 HTTPS 下發，只在記憶體中使用。
- 首次連線會顯示主機金鑰指紋（TOFU），確認後存回服務端；主機或連接埠變更時指紋會重設。
- 管理員可用 Bot 指令開關帳號 SSH 功能；關閉時 GUI 顯示未啟用提示，API 拒絕存取。

注意：撤銷 session 或關閉 SSH 功能不會切斷已建立的 SSH 連線，只會阻止後續憑證下發。

## 安全模型

- 服務端是唯一信任邊界。SSH 憑證是唯一會下發的憑證（個人主機，HTTPS 下發、記憶體使用、每次下發都寫稽核）。
- 本機只保存：驗證服務 URL、帳號名稱、裝置 ID（remote-preferences JSON）、語言偏好（preferences JSON）與 DPAPI 加密的 refresh token。
- 密碼與 SSH 憑證內容不落地。

## 建置

Windows PowerShell：

```powershell
$env:CGO_ENABLED = '0'
go build -ldflags='-H windowsgui' -o s12ryt-ssh.exe .
```

一般開發建置與測試：

```powershell
go test ./... -count=1
go vet ./...
go build ./...
```

`-H windowsgui` 會隱藏 Windows 主控台視窗。

## 使用方式

```powershell
.\s12ryt-ssh.exe
```

啟動後即為遠端登入畫面；登入成功進入工作區，右上角可登出。關閉視窗會自動撤銷 session 並結束進程。

## 本機資料

應用程式使用作業系統使用者設定目錄（`<UserConfigDir>/s12ryt-ssh/`）：

- `remote-preferences.json`：驗證服務 URL、帳號與隨機 device ID，不保存密碼或 token。
- `preferences.json`：版本化語言偏好，只保存 `en` 或 `zh-TW`。
- `securestore/`：Windows DPAPI 保護的 rotation refresh token；短期 access token 只存在記憶體。

## 專案結構

```
main.go                  應用程式入口（Gio 標準退出模式、路徑與服務組裝）
internal/config          SSH profile 型別（含私鑰內容 KeyData）
internal/gui             Gio GUI（遠端登入、工作區、SSH 主機與終端機）
internal/i18n            英文/繁體中文字典
internal/remote          遠端 API client（登入、token 輪換、資源概覽、SSH hosts）
internal/securestore     DPAPI 保護的 secret 儲存
internal/ssh             SSH 客戶端（密碼/私鑰認證、TOFU 指紋、PTY 終端機）
agent/                   需求契約與工作紀錄
```

## 測試

```powershell
go test ./... -count=1
go vet ./...
```

測試涵蓋 securestore、SSH fingerprint/逾時/PTY、遠端 API client、GUI state 與工作區行為。

本機 Windows 防毒曾誤判 `internal/i18n` 的 Go 測試執行檔，因此該 package 的測試由 GitHub Actions Windows runner 執行；其他 Go 套件仍可在本機個別驗證。

## 依賴

- [Gio](https://gioui.org/)：桌面 GUI。
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh)：SSH。
