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

## Telegram 身分驗證服務需求

- 讀取 `todos-auth-tgbot.md`，確認新需求包含 Node.js 22 Telegram Bot、S3/SQL connection 管理、子帳號、身份校驗 port 與 Go GUI 第三種登入方式。
- 使用者確認完整實作；已完成服務代理模式、權限粒度、Telegram 管理員來源、long polling、SQLite、TypeScript/npm、session、GUI 遠端工作區、Bot 管理範圍與稽核的多輪需求澄清。
- 使用 Context7 查核 Fastify v5 完整 JSON Schema/plugin/inject、grammY long polling/private chat/inline keyboard/i18n，以及 Node 22 `node:sqlite` 與 crypto API。
- 技術契約已寫入 `agent/question.md`；正式程式尚未修改，下一步從 server domain/security 的 RED 測試開始。

## Telegram 身分驗證服務實作

- 建立 `server/` Node 22 TypeScript 專案、strict TypeScript/ESLint/Prettier、`.env.example` 與 npm lock；production dependency audit 初始為 0 vulnerabilities。
- 依 RED -> GREEN 完成 config、scrypt/AES-GCM/token crypto、Node SQLite transaction migration、repository、AdminService、AuthService、Fastify API、S3/SQL ProxyService 與真實 adapters。
- Bot 測試先因 controller/adapter 不存在而 RED；後完成私聊/admin allowlist、繁中/英文、inline menu、slash commands、connection wizard、grant/session/audit 與敏感訊息 best-effort delete。
- 品質覆核新增 inline connection wizard 與繁中動態狀態測試；第一次執行為 40/42，分別因 connection menu 無 keyboard 及 `disabled | devices=2` 未翻譯而 RED。完成 callback、keyboard、wizard name state 與動態 label 後 42/42 GREEN。
- 另新增繁中重設密碼回覆不得含 `session` 的回歸斷言；第一次精準失敗於「所有既有 session 已撤銷」，修正為「所有既有工作階段已撤銷」後目標與全套 Node 測試通過。
- Runtime/index 測試先因 module 不存在而 RED；後完成 production dependency composition、health、audit cleanup、HTTP/Bot 啟動、polling failure、訊號 shutdown、冪等/聚合 close。Node 最終 42 tests、typecheck、lint、build 通過。
- Go `internal/remote` 分四輪 TDD 完成 URL/preferences、login/refresh/logout、S3/SQL proxy 與 service/device ID；目標測試與 `go vet ./internal/remote` 通過。
- Gio model/window 測試先 RED，再加入 remote login/workspace screens、assigned resource/grant filtering、S3/SQL代理操作、DPAPI restore/logout及繁中/英文文案；`go test ./internal/gui -count=1` 通過。
- `main.go` 新增獨立 `remote-preferences.json` 與 remote service 注入；root target test通過。SQL nil parameters regression先 RED，再以 `omitempty` 修復並通過 remote tests。
- `.github/workflows/ci.yml` 新增 Node 22 job；根 `.gitignore` 排除 server `.env`、`node_modules`、`dist`、`data` 與 SQLite檔案。
- 新增 `server/README.md`，根 README加入遠端登入入口、Node啟動、安全模型與本機資料說明；server 文件已補充 inline S3/MySQL/PostgreSQL 按鈕與 slash commands 共用精靈。
- 最終 Node 驗證：format check、ESLint、TypeScript typecheck、42/42 tests、build 全通過；production npm audit 為 0 vulnerabilities。測試只有 Node `node:sqlite` experimental warning。
- 最終 Go 驗證：未執行本機 `internal/i18n` package test；其餘專案 package 測試全綠，並通過對同一 package 集合的 `go vet`、`go build`。
- 安全與文件驗證：server source 無 TODO/FIXME/not implemented；常見 AWS/GitHub/Slack/private-key pattern 無命中；`git diff --check` 無 whitespace error；README/CI YAML 可由 Prettier 解析；無舊 TUI 依賴。
- 本輪未執行 Git commit/push，因此新 Node/Go 功能尚未由 GitHub Actions 驗證；未連線真實 Telegram、S3、MySQL/PostgreSQL，亦未重新產生 Windows GUI linker build。
- 使用者明確要求推送後，依 git-master 將 60 個檔案拆成 27 個英文 plain-style 原子提交，推送 `main` 至 `origin/main`；推送範圍為 `d5914e8..2c3912f`。
- 推送前確認工作樹乾淨、27 個提交完整、`server/.env`、SQLite、`node_modules`、`dist` 與 runtime data 未被追蹤，且 `git diff --check origin/main..HEAD` 通過。
- GitHub Actions run `33036030920` 完整成功：Node 22 server checks、Go 1.25.x/1.26.x checks、govulncheck、secret scan、Windows GUI build全部通過；僅有第三方 actions 的 Node.js 20 deprecation 非阻塞提示。

## 2026-08-27：UI 自主疊代升級（檔案操作與驗證）

- 讀取：window.go/remote_window.go/model.go 全文、ui_upgrade 相關、i18n.go translations 區段、module cache 內 Gio v0.10.2（clipboard.WriteCmd stream API、material.Loader、gofont.Collection 含 Go Mono、Editor.Update 迴圈取代 Events()）。
- 修改 `agent/question.md`：增補本輪 UI 疊代範圍、確認框形態、響應式堆疊與驗收標準。
- 新增 `internal/gui/ui_upgrade.go` 與 `ui_upgrade_test.go`（20 個測試：ANSI 過濾、terminal 上限、tailLines、submit 消費、confirmation 狀態機、DB type 正規化、堆疊判定、secret 判定含 passphrase、reveal mask、按鈕配色、必填驗證、fakeTerminal、selectObject、try* 方法、繁中翻譯 17 字串、requestConfirm busy 防護）。
- 修改 `internal/gui/window.go`：八個獨立 List、kind 欄位+selector、drainEditors/requestConfirm/confirmModal、actionButton/outputList/objectBrowser、status Loader、recovery 複製、editorRow 響應式、appendTerminalFilter、Shaper gofont。
- 修改 `internal/gui/remote_window.go`：remoteList、遠端確認框、物件點選、outputList、Signing out...、remoteAction primary/danger。
- 修改 `internal/gui/window_test.go`：setupDBKind 取代 editor、錯誤字串更新。
- 修改 `internal/i18n/i18n.go`：19 個新 Key 英繁對稱、KeySQLBootstrap/KeyDatabaseRequired 移除 type 字樣。
- 驗證：`go test ./internal/gui -count=1` ok；全套 `go test -count=1`（排除本機 i18n）10 套件 ok；`go vet ./...` exit 0；`go build ./...` exit 0；gofmt clean；i18n 字典靜態對稱（translations 87/87、extra 74/74、零差集）。
- 未執行：本機 i18n 測試（防毒隔離 exe，交 GitHub CI）、Windows GUI linker build、commit/push。

## 2026-08-27：UI 升級提交與 CI 回歸修復（Git 操作）

- 讀取：`git status/diff/log -30`（風格偵測 ENGLISH+PLAIN）、i18n diff、service.go:399-417 validateBootstrap、window.go setupBootstrap、i18n.go 字典英繁值、CI 失敗日誌（gh run view --log-failed）。
- 寫入：`agent/question.md`（需求契約增補）、`agent/deep_todos.md`、`agent/memory.md`、`agent/項目表.md` 紀錄更新。
- Git 操作：`$env:GIT_MASTER='1'` 下建立 5 個原子提交——f679527 Extend interface translations、3d19864 Add interface upgrade helpers、1b618d4 Update local workspace interface、9ddcbf0 Update remote workspace interface、ad04369 Align translated validation messages（fix）；push 8fa9202..ad04369 main→origin/main。
- 缺陷修復：run 33043969578 RED（TestGUIStringsTranslateToTraditionalChinese 對舊字串 "SQL type, host, ..." 反查失敗）；根因＝字典改了但 service.go:407 與 i18n_test.go:87 未同步；GREEN 證據＝run 33050084208 六個 job 全部成功（Go 1.25.x/1.26.x test/vet/build、govulncheck、gitleaks、Node 22 server checks、Windows GUI build）。
- 本輪限制照舊：本機不跑 internal/i18n 測試（防毒誤判測試執行檔）、不重產 .exe、race/gopls 不可用；替代證據為精確字串一致性 grep + 字典靜態對稱 + GitHub Actions Windows runner 實跑。
