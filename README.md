# s12ryt-ssh

Windows 優先的 Go 桌面 SSH 工作區，使用 Gio GUI，並整合 S3/R2 物件儲存與 MySQL/PostgreSQL 資料庫操作。

所有 SSH、儲存與資料庫 profile 都會先在本機加密，再以單一密文 vault 保存到使用者選定的遠端 backend。bootstrap 連線資料使用 Windows DPAPI 保存，不寫入明文 JSON。

## 功能

- SSH 密碼或私鑰認證、主機 key fingerprint 驗證、互動式 PTY 終端與視窗調整。
- S3 相容儲存，包括 Cloudflare R2、AWS S3、MinIO：列表、上傳、下載與刪除。
- MySQL/PostgreSQL：資料表列表、Query 與 Exec。
- 首次設定精靈：選擇 R2/S3 或遠端 SQL 作為 vault backend，測試 bootstrap 連線並建立 vault。
- 一次性復原金鑰，可在忘記密碼時輪換 vault 名稱、密碼與復原金鑰。
- 單一 Windows `.exe`，可在 `CGO_ENABLED=0` 下建置。
- GUI 支援英文與繁體中文；右上角「中 / EN」可即時切換，偏好會保存於本機非敏感設定檔。

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

`-H windowsgui` 會隱藏 Windows 主控台視窗。安全儲存與 GUI 發行流程以 Windows 為首要支援目標。

## 使用方式

```powershell
.\s12ryt-ssh.exe
```

首次啟動會開啟設定精靈：

1. 選擇 R2/S3 或 SQL vault backend。
2. 輸入 bootstrap 連線資料並測試連線。
3. 設定 vault 名稱與密碼。
4. 保存 GUI 顯示的一次性復原金鑰。

後續啟動使用 vault 名稱與密碼登入。設定完成後，profile 可在 SSH、S3/R2 與 SQL 分頁中管理。

## 本機資料

應用程式使用作業系統使用者設定目錄：

- `metadata.json`：vault ID、名稱、backend 類型與 securestore key，不包含 profile 或 bootstrap secret。
- `securestore/`：Windows DPAPI 保護的 bootstrap 連線資料。
- `preferences.json`：版本化語言偏好，只保存 `en` 或 `zh-TW`，不保存任何憑證或 profile。

遠端只保存版本化的 AES-GCM vault 密文。舊版明文 `config.json` 不再是應用程式入口，也不保證相容。

## 專案結構

```
main.go                  Gio Windows GUI 入口
internal/app/            vault 設定、登入、復原與 session service
internal/config/         profile 型別
internal/gui/            Gio 設定精靈、登入與工作區畫面
internal/vault/           AES-GCM vault 與 R2/SQL backend
internal/securestore/     Windows DPAPI 與記憶體測試實作
internal/ssh/             SSH host key、逾時與 PTY
internal/storage/         S3/R2 與記憶體儲存
internal/database/        MySQL/PostgreSQL/SQLite client
agent/                   需求契約與工作紀錄
```

## 測試

```powershell
go test ./... -count=1
go vet ./...
```

測試涵蓋 vault 加密與復原輪換、securestore、SSH fingerprint/逾時/PTY、S3 分頁、資料庫 DSN/TLS/關閉防護、service workflow 與 GUI state/profile 行為。

## 依賴

- [Gio](https://gioui.org/)：桌面 GUI。
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh)：SSH。
- [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2)：S3/R2。
- [database/sql](https://pkg.go.dev/database/sql)：資料庫連線抽象。
