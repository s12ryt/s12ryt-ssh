# 操作記錄

## 2026-08-26

- 讀取 `main.go`、`go.mod`、`README.md`、`todos.md` 與 `internal/{config,ssh,storage,database,ui}` 相關程式。
- 確認工作區不是 Git repository，未執行任何 Git 寫入操作。
- 建立 `agent/` 目錄。
- 建立 `agent/question.md`、`agent/deep_todos.md`、`agent/項目表.md`。
- 需求確認完成：移除 TUI、採 Gio Windows GUI、完整三類功能、R2/S3 或 SQL vault 二選一、本機加密遠端密文、Windows bootstrap 安全儲存、一次性復原金鑰。
- 基線驗證已於前置檢查完成：`go test ./...`、`go vet ./...`、`go build ./...` 通過；race 受 CGO 未啟用限制；gopls 未安裝。
- 新增並先以 RED 驗證 `internal/gui/window_test.go`：storage profile 必須保留 `UsePathStyle`，Window.Close 必須取消互動 PTY context。
- 修改 `internal/gui/window.go`：加入 setup/storage path-style 控制與保存；加入 `terminalCtx`/`terminalCancel`，由 `closeSSH` 統一處理視窗關閉、手動 SSH 關閉與重新連線。
- 驗證 `go test ./internal/gui -run 'Test(StorageProfilePreservesPathStyle|CloseCancelsInteractiveTerminalContext)' -count=1` 與 `go test ./internal/gui -count=1` 通過。
- README 改寫為 Gio GUI、遠端密文 vault、Windows DPAPI 與 PowerShell 建置說明。
- 針對 path-style toggle 新增 `TestToggleStoragePathStyleAffectsProfile`，先驗證缺少 `toggleStoragePathStyle` 的 RED，再加入 helper 與 `handleStorage` 事件接線；`gofmt` 後目標測試與 `go test ./internal/gui -count=1` 通過。
- SSH Connect 增加 busy guard，避免 async 工作忙碌時先建立但無法交給 worker 的 terminal context。
- 最終驗證通過：`go test ./... -count=1`、`go test ./... -cover -count=1`、`go vet ./...`、`go build ./...`、`CGO_ENABLED=0 go build -ldflags='-H windowsgui' -o s12ryt-ssh.exe .`。
- `go list -deps .` 無 `charmbracelet`、`bubbletea`、`lipgloss`、`bubbles`，且 `internal/ui` 已不存在。
- `go test -race ./...` 實際回報 `-race requires cgo; enable cgo by setting CGO_ENABLED=1`；LSP 實際回報 `gopls` 未安裝，因此兩者均列為未完整驗證。

## 2026-08-27

- 使用者要求提供中文/英文切換；已在 `agent/question.md` 記錄英文預設、繁體中文、右上角「中 / EN」、即時切換、跨重啟保存與外部原始錯誤保留原文。
- 先新增 `internal/i18n/i18n_test.go`，RED 原因為缺少 `Language`、偏好與翻譯 API；新增 `internal/i18n/i18n.go` 後完成偏好格式與字典實作。
- 新增 GUI Window 語言偏好測試，並接入 `NewWindowWithPreferences`、`toggleLanguage`、`applicationPreferencesPath`。
- GUI 的 button/editor/read-only/status/output 路徑已集中經 `i18n.Text` 解析；未知來源字串維持原文以保留外部服務錯誤。
- `go test ./internal/gui ./...` 中除 `internal/i18n` 外套件通過；`internal/i18n` 測試執行檔被 Windows 防毒攔截，先前亦曾回報 `Access is denied`/找不到被隔離檔案。
- `go vet ./...`、`go build ./...`、TUI 依賴檢查通過。因 `Ransom/Genasom.p` 告警，未執行新的 Windows GUI linker build。

- 追加 i18n RED 測試來源：`Unlocking encrypted vault...`、`PostgreSQL SSL mode (default require)` 與外部錯誤原文保留；先前測試執行檔已被防毒隔離，本輪未重新執行 `go test`。
- i18n 正式碼新增 `KeyStatusUnlock` 英文/繁體中文翻譯；`Text` 維持本機偏好錯誤前綴翻譯及外部錯誤尾端原文保留。
- 重新執行 `gofmt`、`go vet ./...`、`go build ./...` 均通過；依賴樹過濾輸出 `No TUI dependencies found`。
- 本輪未產生或執行新的 `.exe`，所以 Windows GUI linker build 未驗證；race 仍因 `CGO_ENABLED=0` 不可用，gopls 未安裝。

## GitHub 推送

- 使用者確認目標為公開 repository `s12ryt/s12ryt-ssh`、`main` 分支、直接推送。
- 執行 `git init -b main`，以 `.gitignore` 排除本機設定、securestore、執行檔與測試執行檔；修正 `/securestore/` 規則後確認 `internal/securestore` 仍被追蹤。
- 依 git-master 規則使用 `$env:GIT_MASTER='1'` 執行 Git 命令，建立 19 個英文 plain-style 原子提交，並設定 `origin`。
- 新增並提交 `.github/workflows/ci.yml`：push/PR、Go 1.25.x/1.26.x、Windows test/vet/build、GUI linker build、govulncheck、gitleaks。
- 新增並提交 `.github/workflows/release.yml`：`vX.Y.Z` tag 或 dispatch、Windows amd64/arm64、ZIP、checksums、GitHub Release。
- `git push -u origin main` 成功；遠端為 `https://github.com/s12ryt/s12ryt-ssh`，本地狀態為 `main...origin/main` 且乾淨。
- GitHub workflow 檔案已透過 GitHub API 驗證存在。`go vet ./...`、`go build ./...`、`git diff --check` 與常見憑證 pattern scan 通過。
- 未執行最新 `go test`、Windows linker build、`actionlint`；原因分別為防毒隔離測試執行檔、避免產生新的 `.exe`、工具未安裝。這些限制需在交付報告中明確標示。

## CI 修復

- 讀取 GitHub Actions run `33000629262` 失敗日誌，確認 i18n `KeyBytes: "bytes"` 觸發翻譯完整性測試失敗，以及 `golang.org/x/image v0.26.0` 觸發 4 個 govulncheck TIFF 漏洞。
- 先新增 `TestBytesTranslationUsesHumanReadableEnglishLabel` 作 RED 回歸測試；本機嘗試執行時測試執行檔被 Windows 回報 `Access is denied`，因此依使用者要求不再於本機執行 i18n 測試，改交 GitHub Actions。
- 將 `KeyBytes` 英文文案與 GUI source 改為 `Bytes`，下載輸出與物件列表均使用集中式翻譯來源。
- 執行 `go get golang.org/x/image@v0.45.0` 與 `go mod tidy`，`go list -m` 確認安全版本；`go mod verify` 通過。
- 新增 `.playwright-mcp/` 至 `.gitignore`。
- 本輪未執行任何本地 i18n 測試；`go vet ./...`、`go build ./...`、`gofmt`、TUI dependency check 通過。下一步為原子提交、推送及 GitHub CI 驗證。
- 依 git-master 規則建立 5 個英文 plain-style 原子提交並推送 `main`；GitHub Actions run `33002069967` 的 Go 1.25.x/1.26.x test、vet、build、Windows GUI build、govulncheck、secret scan 全部成功。
- GitHub run 顯示 Node.js 20 action deprecation annotation，屬非阻塞警告，未影響 CI 結果；本地仍未執行 i18n 測試。

## README 設定文件

- 讀取 `README.md`、`internal/config/config.go`、`internal/storage/s3.go`、`internal/database/database.go`、`internal/vault/backend.go` 與 `internal/app/service.go`，以實際欄位和行為為文件依據。
- 在 README 的使用方式後新增 Vault 與工作區 profile 的設定模型，說明 bootstrap secret 使用 Windows DPAPI、工作區 profile 進入遠端加密 vault，且 Vault backend 只選 R2/S3 或 SQL 其中一種。
- 新增 Cloudflare R2、AWS S3、MinIO 與通用 S3 相容服務設定參考；明確說明 endpoint、region、bucket、static credentials、Path-style，以及 vault/工作區所需 S3 API 權限。
- 新增 MySQL/MariaDB 與 PostgreSQL 設定參考；記錄 MySQL `TLSMode` 預設 `true`、PostgreSQL `SSLMode` 預設 `require`、安全模式差異，以及 SQL Vault 所需 schema 和權限。
- 文件明確列出目前未支援的 STS session token、自訂 CA 欄位、Unix socket、內建 SSH tunnel 與額外 DSN query parameter，避免使用者依文件設定不存在的功能。
- README 驗證：章節與欄位 grep 通過，常見 access key/token/private key pattern 未命中，`git diff --check` 通過；本次未執行 go test，也未執行 Git commit/push。
