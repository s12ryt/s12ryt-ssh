# 工作歷史

## 2026-08-26：GUI 與安全 vault 自主升級

- 原專案為 Go backend skeleton 加 Bubble Tea TUI prototype。
- 基線：`go test -timeout=60s ./...`、`go vet ./...`、`go build ./...` 通過。
- 已確認高風險缺口：TUI 假連線、SSH 不驗證 host key、明文憑證、SSH Exec 逾時未落實、DB DSN/TLS、S3 List 未分頁。
- 使用者確認移除 TUI，改為 Windows 優先 Gio GUI；完整承接 SSH/S3/SQL 功能。
- 使用者確認遠端 vault 支援 R2/S3 或 SQL 二選一；本機加密後遠端只存密文；bootstrap 憑證用 Windows 安全儲存；首次註冊產生一次性復原金鑰。
- 已完成 vault、securestore、SSH、S3、資料庫、app service、Gio GUI 與 Gio 入口的 TDD 實作。
- 已移除 `internal/ui` 及 Bubble Tea/Lipgloss/Bubbles 依賴；profile 不再由明文 `config.json` 啟動。
- 已補強 GUI 的 S3 path-style 設定保存，以及互動式 SSH PTY 的長生命週期取消流程。
- README 已更新為 GUI/vault 使用方式。
- 已補強 GUI 的 storage path-style toggle：先以 `TestToggleStoragePathStyleAffectsProfile` 取得預期 RED，再完成 helper 與事件接線，目標測試及 GUI 套件測試通過。
- 已補強 SSH Connect 的 busy guard，避免背景工作忙碌時建立未被使用的 PTY context。
- 最終驗證：`go test ./... -count=1`、`go test ./... -cover -count=1`、`go vet ./...`、`go build ./...` 與 `CGO_ENABLED=0 go build -ldflags='-H windowsgui' -o s12ryt-ssh.exe .` 均通過。
- `go list -deps .` 確認不含 Bubble Tea/Lipgloss/Bubbles；`internal/ui` 已移除。
- `go test -race ./...` 受 `CGO_ENABLED=0` 環境限制而無法執行；`gopls` 未安裝，未完成 LSP 診斷。
- 本次自主升級與驗收項目已完成，僅保留上述工具環境限制與未連接真實外部服務的整合驗證風險。

## 2026-08-27：GUI 語言切換

- 已確認 GUI 預設英文、支援繁體中文、右上角「中 / EN」即時切換，並跨重啟保存非敏感語言偏好。
- 新增 `internal/i18n`：版本化 `preferences.json`、英文/繁中字典、翻譯 key 完整性測試與未知文字原文保留。
- `internal/gui.Window` 已接入語言偏好載入/保存；按鈕、欄位提示、狀態、空資料訊息、驗證訊息與操作輸出使用翻譯解析。
- `main.go` 已將偏好檔放在使用者設定目錄，與 metadata/securestore 分離。
- README 已補充語言切換與 `preferences.json` 說明。
- `go vet ./...`、`go build ./...` 與 GUI/main 測試通過；完整測試受防毒程式隔離 `internal/i18n` 測試執行檔影響，未能完整執行。
- 因防毒告警 `Ransom/Genasom.p`，未再次產生或執行 Windows GUI `.exe`；Windows linker build 尚未重新驗證。

## 2026-08-27：語言切換最後覆核

- 先新增 i18n RED 測試，補強 `Unlocking encrypted vault...`、PostgreSQL SSL 欄位提示及外部服務錯誤原文保留契約。
- 新增 `KeyStatusUnlock` 英文/繁體中文翻譯；GUI 現有 setup、login、recovery、workspace、profile、操作狀態、空結果與本機驗證來源均已完成字典覆蓋。
- `Text` 對語言偏好保存錯誤只翻譯本機錯誤前綴，保留尾端原始錯誤；未知遠端錯誤維持原文。
- 已執行 `gofmt`、`go vet ./...`、`go build ./...`，均通過；`go list -deps .` 過濾結果為 `No TUI dependencies found`。
- 本輪未執行 `go test`，因 Windows 防毒曾將 `internal/i18n` 測試執行檔判定為 `Ransom/Genasom.p` 並隔離；因此本輪新增 i18n 測試及全套測試不能宣稱 GREEN。
- 本輪未執行 Windows linker build，以避免再次產生 `.exe`；`go test -race` 仍受 `CGO_ENABLED=0` 限制，`gopls` 未安裝。

## 2026-08-27：GitHub、CI 與 Release

- 使用者確認建立公開 repository `https://github.com/s12ryt/s12ryt-ssh`，直接推送 `main`，不建立 Pull Request。
- 建立根目錄 `.gitignore`，排除 `config.json`、metadata、preferences、`/securestore/`、本機執行檔與測試執行檔；修正過寬的 `securestore/` 規則，保留 `internal/securestore` 正式程式碼。
- 以 19 個英文 plain-style 原子提交建立本地 Git 歷史，避免將不同模組、文件與 workflow 混成單一大型提交。
- 新增 `.github/workflows/ci.yml`：Go 1.25.x/1.26.x、Windows test/vet/build、Windows GUI build、Windows dependency govulncheck，以及 gitleaks 秘密掃描。
- 新增 `.github/workflows/release.yml`：限制 `vX.Y.Z` 版本格式，支援 tag/手動觸發，編譯 Windows amd64/arm64、產生 ZIP 與 SHA-256 checksums 並建立 GitHub Release。
- 已成功設定 `origin` 並推送 `main` 至 `origin/main`；遠端 workflow 與最新提交已用 GitHub API 驗證。
- 推送前 `go vet ./...`、`go build ./...`、`git diff --check` 與秘密 pattern 檢查通過；本機工作樹乾淨。
- `actionlint` 未安裝；本機最新版本仍未執行 `go test`，因防毒隔離 `internal/i18n` 測試執行檔；Windows GUI linker build 亦未重新執行，避免再次產生 `.exe`。

## 2026-08-27：修復 GitHub CI

- GitHub Actions run `33000629262` 顯示 Go 1.25.x/1.26.x 的 i18n 測試失敗：英文 `KeyBytes` 值與內部 key 同為 `bytes`，被完整性測試判定為缺少翻譯。
- 先新增 `TestBytesTranslationUsesHumanReadableEnglishLabel` 回歸測試，再將英文 canonical 文案改為 `Bytes`，同步修正 GUI 的下載與物件列表來源；繁中仍顯示 `位元組`。
- govulncheck 發現 `golang.org/x/image v0.26.0` 的 4 個 TIFF 漏洞（GO-2026-5066、GO-2026-5062、GO-2026-5032、GO-2026-4815），已透過 Go module 命令升級至 `v0.45.0` 並執行 `go mod tidy`。
- 補充 `.gitignore` 排除 `.playwright-mcp/`，避免瀏覽器工具產物進入 repository。
- 使用者指定 i18n 測試改由 GitHub Actions CI 執行；本輪未再次執行本地 i18n 測試。
- 本地 `gofmt`、`go vet ./...`、`go build ./...`、`go mod verify` 與 TUI 依賴檢查通過；尚待提交、推送並以新的 GitHub Actions run 驗證測試與 govulncheck。
- 已建立 5 個英文 plain-style 原子提交並推送至 `main`；GitHub Actions run `33002069967` 全部通過：Go 1.25.x/1.26.x 測試、vet、build、Windows GUI build、govulncheck 與 secret scan。
- 該 run 僅有 actions/checkout、actions/setup-go 與 gitleaks 使用 Node.js 20 的棄用提示，未影響結果；可在後續獨立升級 workflow action major version。

## 2026-08-27：README 儲存與 SQL 設定參考

- 使用者確認 README 需同時說明首次設定的 Vault backend，以及登入後的 S3/R2 與 SQL 工作區 profile。
- README 新增設定模型，明確區分本機 DPAPI bootstrap、遠端加密 vault 與登入後工作區 profile 的保存位置和用途。
- README 新增 R2/S3 共用欄位、Cloudflare R2、AWS S3、MinIO 與通用 S3 相容服務設定範例，包含 endpoint、region、bucket、static credentials 與 Path-style 判斷。
- README 說明 S3 Vault 所需的 `ListObjectsV2`、`GetObject`、`PutObject`、`DeleteObject` 權限，以及不支援 STS session token、自訂 CA、Unix socket、SSH tunnel 和額外 DSN 參數等限制。
- README 新增 MySQL/MariaDB 與 PostgreSQL 欄位表、TLS/SSL mode 行為、預設值、SQL Vault `s12ryt_vault` schema、必要資料庫權限與工作區 Tables/Query/Exec 行為。
- README 補充 DPAPI、AES-GCM、HTTPS/TLS、最小權限帳號與憑證輪換安全注意事項，並加入 Cloudflare、AWS、MinIO、MySQL driver 與 pgx 官方參考連結。
- 本次為純文件變更，未新增執行測試；以章節、欄位、連結人工檢查、敏感字串 pattern 檢查與 `git diff --check` 作替代驗證，均通過。
- 本次未執行 commit 或 push，待使用者明確要求後再依 git-master 規則處理。

## 2026-08-27：Telegram 身分驗證服務與遠端代理

- 使用者要求依 `todos-auth-tgbot.md` 完整實作 Node.js 22 Telegram Bot 服務端、子帳號管理、身分校驗 port 與 Go GUI 第三種「登入校驗」入口。
- 已確認服務端代理全部 S3/SQL 操作，不向子帳號或 Go 客戶端下發遠端連線憑證；權限粒度為 connection + operation。
- 已確認 TypeScript/npm、Fastify v5、grammY long polling、Node 內建 SQLite/crypto、環境變數設定與自動 migration。
- 已確認 Bot 由環境變數 Telegram user IDs 管理，支援繁中/英文、按鈕與 slash commands、完整帳號/連線/權限/session/稽核管理。
- 已確認 Go GUI 保留本機 Vault 流程並增加獨立遠端工作區；URL/account 保存為非敏感偏好，refresh token 使用 Windows DPAPI，密碼不保存。
- 已確認 access/refresh、裝置上限、反向代理 HTTPS、固定 bucket/prefix/database 邊界、可配置保守代理限制與安全 metadata 稽核契約。
- 已以 TDD 完成 Node config、crypto、SQLite transaction migration、repository、account/session/connection/grant/audit services。
- 已完成 Fastify `/api/v1`、HTTPS/trusted proxy guard、rate limit、opaque access/rotation refresh、S3/SQL permission proxy 與安全 metadata audit。
- 已完成 AWS SDK v3 S3 adapter、mysql2/pg SQL adapter，涵蓋 paginator、streaming、固定 bucket/prefix/database、read-only query、timeout、row/byte limit。
- 已完成繁中/英文 BotController 與 grammY private-chat adapter，支援 account、connection wizard、grant、session、audit 與敏感 incoming message best-effort delete。
- Bot connection 清單已提供 inline S3/MySQL/PostgreSQL 新增按鈕，與 slash commands 共用同一精靈；繁中帳號、裝置、工作階段、connection 與 audit 動態狀態已完成本地化。
- inline 精靈與繁中動態文案先由 2 個回歸測試取得預期 RED，再完成狀態機、鍵盤與翻譯修正；重設密碼回覆的 `session` 混用亦先新增失敗斷言後修正為「工作階段」。
- 已完成 Node runtime/index、audit retention、HTTP/Bot 啟動、SIGINT/SIGTERM graceful shutdown 與聚合資源清理；Node typecheck、lint、build 與 42 項測試通過。
- 已以 TDD 新增 Go `internal/remote`：URL 驗證、非敏感 preferences、DPAPI refresh token、token rotation/reuse response、S3 streaming 與 SQL proxy client。
- 已完成 Gio 第三種「登入校驗」、獨立遠端 S3/SQL workspace、grant-aware controls、無 SSH/無 secret editor，以及 main 的 remote service 注入。
- 已修復 SQL JSON contract：nil parameters 使用 `omitempty`，避免 Fastify schema 收到 `null`。
- CI 已加入 Node 22 format/lint/typecheck/test/build/npm audit job；README 與 `server/README.md` 已補上部署、Bot、API、HTTPS與安全文件，並說明 inline 與 slash command 兩種精靈入口。
- 最終 Node 驗證通過：`npm run format:check`、`npm run lint`、`npm run typecheck`、`npm test`（42/42）、`npm run build`、`npm audit --omit=dev --audit-level=high`（0 vulnerabilities）。
- 最終 Go 驗證明確排除本機 `internal/i18n` 測試後，root、app、config、database、gui、remote、securestore、ssh、storage、vault 全數通過；`go vet` 與 `go build` 對同一組專案 package 通過。
- CI YAML/README 可由 Prettier 解析，`git diff --check` 無 whitespace error，server runtime 資料與 SQLite 檔案可被 `.gitignore` 排除，TUI dependency 與常見憑證 pattern 檢查均無命中。
- 本機仍未執行 `internal/i18n` package tests；本次未推送，因此 GitHub CI 尚未驗證這批變更；真實 Telegram、S3、MySQL/PostgreSQL 服務整合與最新 Windows GUI linker build亦未執行。
