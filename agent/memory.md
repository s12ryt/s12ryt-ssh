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
