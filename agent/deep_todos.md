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
- 已依 git-master 規則將 60 個變更檔案拆成 27 個英文 plain-style 原子提交，並推送至 `origin/main`，遠端最新提交為 `2c3912f`。
- GitHub Actions CI run `33036030920` 全部通過：Node 22 format/lint/typecheck/test/build/npm audit、Go 1.25.x/1.26.x test/vet/build、govulncheck、gitleaks secret scan 與 Windows GUI linker build均成功。
- CI 僅保留 actions/checkout、actions/setup-go 與 gitleaks 的 Node.js 20 deprecation annotation，屬非阻塞工作流程維護事項。

## 2026-08-27：UI 自主疊代升級（合理性修正）

- 使用者經 /ui-ux-pro-max 要求「自主疊代升級：UI 部分還有許多不合理之處，請找出並修正」；共識範圍＝缺陷級+UX 準則級+視覺級全部修正。
- 使用者確認：破壞性操作（S3 Delete/SQL Exec，本機+遠端）採確認對話框；S3 物件列表可點擊自動填入 Object key（本機+遠端）納入。
- 使用者回報「登入頁/SQL 表單欄位缺失」→ 診斷為共用元件 editorRow 窄視窗水平雙欄壓縮右欄（如 Secret key）缺陷；使用者選定「響應式堆疊」（<640dp 垂直堆疊）。
- 發現並納入修正的既有 bug：Key passphrase 欄位未遮罩（密碼判定漏 passphrase）、表單按 Enter 無反應（Submit 設定但無人消費）、`ui.list` 三處共用導致滾動位置跨畫面污染。
- 新增 `internal/gui/ui_upgrade.go`（純邏輯）與 `ui_upgrade_test.go`（20 個測試）：stripANSI/appendTerminalFilter（PTY ANSI 過濾+65536 rune 上限）、tailLines（輸出尾部 1000 行）、consumeSubmit/drainEditors（Enter 提交）、confirmation 狀態機（request/cancel/accept 防重）、normalizeDBType/applyDatabaseKind（MySQL/PostgreSQL 二元選擇，別名正規化）、useStackedRow（響應式堆疊）、isSecretHint、passwordReveal（Show/Hide）、buttonColors（busy 灰/danger 紅）、requireObjectKey/requireSQLStatement、objectsHeader、selectObject/selectRemoteObject、sendTerminalInput、try* 方法（SignIn/CreateVault/RotateRecovery/RemoteSignIn/SSHConnect 自 handle* 平移，行為不變）。
- window.go 接線：八個獨立 layout.List（setup/profile/remote/terminal/storage/database/object/remoteObject）取代共用 ui.list；DB Type 自由文字改 dbTypeSelector 按鈕（setup+workspace）；破壞性操作經 requestConfirm modal（scrim/Cancel/Confirm，busy 時拒開、active 時攔截全部輸入）；upload/download/query 空值驗證；錯誤訊息移除 MaxLines=4 截斷；busy 時 material.Loader 動畫+按鈕變灰；輸出區改 outputList（滾動+自動貼底+GoMono 等寬字型+tailLines）；終端 appendTerminalFilter+stripANSI；profile 側欄空狀態；復原金鑰一鍵複製（clipboard.WriteCmd）；登出 busy guard；editorRow 響應式堆疊+isSecretHint 遮罩修正；Shaper 注入 gofont.Collection（含 Go Mono）。
- remote_window.go 接線：remoteList 獨立；遠端 delete/exec 確認框；遠端物件點選；輸出區 outputList；logout「Signing out...」；tryRemoteSignIn；remoteAction 加 primary/danger。
- i18n 新增 19 個 Key 英繁對稱（Cancel/Confirm/Delete object 與訊息/Execute SQL 與訊息/Show/Hide/Copy recovery key/Recovery key copied/No profiles/物件與 SQL 必填/SSH terminal is not connected/%d objects/Signing out/database type）；KeySQLBootstrap/KeyDatabaseRequired 移除 type 字樣對應 selector 化。
- RED→GREEN 證據：三批測試（16+3+1 個）先以 undefined 編譯失敗確認 RED，實作後 `go test ./internal/gui -count=1` 全綠。
- 最終回歸：全套 go test（排除本機 i18n）10 套件 ok、`go vet ./...` 0、`go build ./...` 0、gofmt clean；i18n 字典靜態對稱驗證（translations en/zh 各 87 keys、extra 各 74 keys、無差集）作為本機防毒限制的替代證據。
- 未執行：本機 `internal/i18n` 測試（防毒隔離，交 GitHub CI）、Windows GUI linker build（避免產生 .exe 觸發防毒）、真實 PTY/S3/SQL 端到端操作、commit/push（待使用者要求）。

## 2026-08-27：UI 升級提交與 CI 回歸修復

- 使用者選定將 UI 疊代變更以原子提交推送並由 GitHub CI 驗證；預推本地驗證全綠（gofmt clean、go vet exit 0、go build exit 0、go test ./internal/gui ok）。
- 依 git-master 建立 4 個英文 plain-style 原子提交：`f679527` Extend interface translations（17 個 i18n key）、`3d19864` Add interface upgrade helpers（ui_upgrade.go+test 新檔）、`1b618d4` Update local workspace interface（window.go+window_test.go）、`9ddcbf0` Update remote workspace interface；push `8fa9202..9ddcbf0` main→origin/main。
- GitHub Actions run `33043969578` 失敗（RED）：Go 1.25.x/1.26.x 兩版本的 TestGUIStringsTranslateToTraditionalChinese 報 GUI string "SQL type, host, port, user, password, and database are required" was not translated；govulncheck、secret scan、Node 22 其餘 job 全綠，Windows GUI build 因前置失敗 skip。
- 根因：`f679527` 將 KeySQLBootstrap 英文字典值改為不含 type 的新字串（對應 DB type selector 取代手填），但 internal/app/service.go:407 validateBootstrap SQL 分支仍回傳含 type 的舊字串，internal/i18n/i18n_test.go:87 反查清單亦未同步 → Text() 反查不到 → 測試失敗。
- 修復採單一事實來源方案：service.go 與 i18n_test.go 同步為 "SQL host, port, user, password, and database are required"；bootstrap.DB.Type=="" 必填防禦檢查保留；GUI window.go:1615/window_test.go:234 早已使用同一新字串。
- 本地替代驗證（i18n 測試依慣例交 GitHub Actions）：git grep 舊字串 0 殘留、新字串恰出現於字典英繁+service.go+window.go+window_test.go+i18n_test.go 五處；gofmt clean；go vet/go build exit 0；全套 go test（排除 internal/i18n）10 套件 ok。
- 提交 `ad04369` Align translated validation messages（service.go+i18n_test.go 同一訊息契約的兩半不可分離故同體提交）並 push `9ddcbf0..ad04369`。
- GitHub Actions run `33050084208` 全部通過：Go 1.25.x checks (1m7s)、Go 1.26.x checks (1m19s)、govulncheck (18s)、secret scan (8s)、Node 22 server checks (39s)、Windows GUI build (1m19s)；僅餘 actions 的 Node.js 20 deprecation annotation（非阻塞工作流程維護事項）。

## 2026-08-29：server 目錄審查（無程式變更）

- 使用者要求「看一下 server/」；執行完整基線驗證並通讀 server/src 全部 17 個原始檔。
- 基線全綠：format:check、lint、typecheck、npm test 42/42、build、npm audit --omit=dev 0 vulnerabilities；工作樹乾淨（HEAD c816c07）。
- 曾懷疑 http/app.ts 全域 bodyLimit 1MB 會擋下 1MB～100MB 的 GUI 上傳（Go client 會設 Content-Length）；以暫存重現腳本實證 Fastify v5 對串流 content-type parser 不強制 app-level bodyLimit（content-length 5MB 與 chunked 5MB 均 200），疑慮不成立，實際限制是 proxy 的 S3_MAX_BYTES/limitBytes；暫存腳本已刪除。
- 審查結論：無重大或高風險缺陷。提出 6 項非必要建議：token/session/refresh_history 表無限期成長（audit 有每日清理但 token 表沒有）、login 帳號不存在時跳過 scrypt 的計時側通道、Bot /connection_test 無逾時、runtime 把 audit cleanup 錯誤路由到 onBotError 會導致整個服務關閉、TRUSTED_PROXIES CIDR 格式未驗證、wizard 敏感值 trim。
- 正向確認：分層架構乾淨、SQL 全面 prepared statements、session 建立/輪換皆在 transaction、refresh reuse 撤銷整個 family、S3 key `..` 防護、上傳下載雙重 byte 上限、稽核僅存 statement hash、敏感 wizard 訊息 best-effort 刪除屬實。
- 未連線真實 Telegram、S3、MySQL/PostgreSQL；本輪無程式碼或文件變更，僅更新 agent 紀錄。

## 2026-08-29：server 拆分為獨立倉庫

- 使用者確認：以 `git subtree split -P server` 保留完整歷史，拆分至公開新倉庫 `s12ryt/s12ryt-ssh-auth-server`；主倉庫移除 `server/` 目錄、CI 的 node-checks job、README 與 .gitignore 的 server 條目，並立即提交推送。
- split 產生 13 個歷史提交（811f9d2..3ec0607 對應新 SHA），調整前驗證 `git diff server-split main:server` 為空（tree 完全一致）。
- 新倉庫在 split 歷史上加 3 個調整提交：bdae7de Adapt README for standalone repository、b01e2ee Add Node CI workflow（Node 22 checks + secret scan，移除 working-directory）、0c9044d Add repository ignore rules；新倉庫 main 推送成功（HEAD 0c9044d）。
- README/ci.yml 調整以主倉庫 server 的 prettier 驗證通過；提交 blob 經 cat-file 驗證為純 LF。
- 主倉庫依 git-master 建立 4 個功能提交：c9348e2 Remove authentication server directory（34 檔案）、25d606c Drop Node server checks from CI、3d08475 Point documentation to the auth server repository（7 處指向新倉庫）、3dcac97 Remove server ignore rules；外加 2 個紀錄提交 019521d/5fc9065 與本輪紀錄提交。
- 待驗證：兩倉庫 GitHub Actions CI 結果（推送後檢查）；worktree F:\Project\ssh\s12ryt-auth-server 於驗證後移除。
- CI 驗證結果：主倉庫 run 33229213719 全部通過（Go 1.25.x/1.26.x test/vet/build、Windows GUI build、govulncheck、secret scan），node-checks job 已不存在。
- 新倉庫首次 push run 33228963820：Node 22 checks 通過，但 secret scan 因 gitleaks-action 對「根提交為範圍起點」的已知邊界情況失敗（`root^..HEAD` 無效，與主倉庫首次推送 run 33000245056 完全同型；非真實洩漏）。
- 修復：新倉庫提交 daa3cb1 Allow manual CI runs（CI 加 workflow_dispatch 觸發，prettier 驗證通過）並推送；push run 33229463300 全綠；手動 dispatch run 33229593777 完成 17 commits / 295 KB 全歷史掃描，no leaks found。
- 收尾：worktree F:\Project\ssh\s12ryt-auth-server 已移除、本地 server-split 分支已刪除、主倉庫 remote 清理；新倉庫 main 最終為 daa3cb1（17 個提交）。
