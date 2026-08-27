# 需求契約

## 已確認需求

- 移除 Bubble Tea/Lipgloss/Bubbles TUI，不保留假連線流程。
- 程式改為正式桌面 GUI `.exe`，第一階段 Windows 優先。
- GUI 採 Gio，建置不依賴 CGO，Windows 版本使用 `-ldflags="-H windowsgui"` 隱藏主控台。
- 首次啟動提供設定精靈，建立 vault、選擇 R2/S3 或遠端 SQL 作為 vault backend，並測試 bootstrap 連線。
- R2/S3 與 SQL 都支援，但單一 vault 只選擇其中一種 backend，不做雙寫同步。
- 程式首次建立隨機 UUID，使用者設定名稱與密碼；以 UUID、名稱與密碼在本機衍生加密金鑰。
- 採本機加密、遠端只存密文；SSH、S3、DB 全部 profile 均放在遠端加密 vault，登入後解密載入。
- bootstrap 連線憑證使用 Windows 安全儲存，不把 bootstrap secret 寫入明文 JSON。
- 首次註冊後產生一次性復原金鑰，GUI 顯示並要求使用者保存；復原金鑰不可回推出原密碼。
- 首版 GUI 完整承接三類功能：
  - SSH 真實連線、互動式 PTY 終端、即時輸入/輸出、視窗尺寸調整與關閉。
  - S3/R2 物件列表、上傳、下載、刪除。
  - MySQL/PostgreSQL 連線、查詢、Exec、資料表檢視。
- 既有 `config.json` 允許破壞性調整，不承諾舊明文 profile 格式相容。
- 自主疊代需同時修復先前審查出的高風險問題：SSH 主機驗證與操作逾時、明文憑證、DB DSN/TLS、S3 分頁、錯誤處理、真實 GUI 狀態。
- GUI 提供英文與繁體中文兩種介面語言。
- 第一次啟動與沒有有效偏好設定時預設使用英文。
- 所有畫面右上角提供全域「中 / EN」切換按鈕，按下後立即切換，不需重新啟動或重新登入。
- 語言偏好跨重啟保存到不含敏感資料的本機偏好設定。
- 翻譯範圍包含應用程式自己的按鈕、標籤、提示、狀態、空資料訊息與輸入驗證錯誤；SSH/S3/SQL 等外部服務回傳的原始錯誤保留原文，避免扭曲診斷資訊。
- 專案推送至公開 GitHub repository `s12ryt/s12ryt-ssh`，使用 `main` 分支直接推送，不建立 Pull Request。
- CI workflow 於 push 與 Pull Request 執行，使用 Go 1.25.x/1.26.x 矩陣，涵蓋測試、vet、一般 build、Windows GUI build、govulncheck 與秘密掃描。
- Release workflow 於 `vX.Y.Z` tag 或手動 dispatch 觸發，建立 Windows amd64/arm64 GUI 發行包、SHA-256 checksums 與 GitHub Release。
- i18n 測試不在本機執行，改由推送後的 GitHub Actions CI 自動驗證，避免 Windows 防毒再次攔截測試執行檔。
- README 提供完整的 R2/S3 相容儲存與 SQL 設定參考。
- 設定文件同時涵蓋首次設定精靈使用的遠端 vault backend，以及登入後工作區使用的 S3/R2 與 SQL profile。
- S3 相容範例涵蓋 Cloudflare R2、AWS S3、MinIO，並說明通用 S3 相容服務可依相同欄位套用。
- 文件提供欄位意義、可用值、安全預設與可直接套用的設定範例；本次不擴張成完整權限管理或故障排除手冊。
- 新增可由 Node.js 22 啟動的 Telegram Bot 身分驗證服務端，程式碼放在 repository 的 `server/`，使用 TypeScript 與 npm。
- 服務端使用 Fastify v5 提供版本化 REST API，使用 grammY 提供 Telegram Bot；Bot 同時支援繁中/英文 inline button 精靈與 slash commands，採 long polling。
- Telegram 最高管理員由環境變數提供一個或多個 numeric user ID；未列入名單的 Telegram 使用者不得操作管理功能。
- Bot 預設語言依 Telegram `language_code` 判斷：`zh*` 使用繁體中文，其餘使用英文；每位管理員可切換並保存偏好。
- Bot 提供完整管理集：子帳號建立、列表、啟用、停權、刪除、重設長期密碼、調整裝置上限、撤銷單一或全部 session；S3/SQL 連線新增、測試、編輯、停用、刪除；權限指派與稽核查詢。
- 子帳號密碼由 Bot 產生高強度長期密碼並只顯示一次；管理員可重設，服務端只保存 scrypt password hash，不保存明文。
- S3/SQL 連線憑證由最高管理員透過 Bot 私聊精靈輸入；Bot 盡力刪除敏感訊息，服務端以環境主金鑰 AES-256-GCM 加密後保存 SQLite。文件必須說明 Telegram 平台仍曾接收該訊息。
- 服務端使用 Node 22 內建 `node:sqlite`，最低支援 Node 22.13；schema 使用版本化 transaction migration，啟動失敗時不得帶著半完成 schema 繼續服務。
- 部署設定使用環境變數並提供 `.env.example`；至少包含 Bot token、Telegram 管理員 IDs、32-byte 主金鑰、SQLite 路徑、listen host/port、trusted proxy、token TTL 與代理限制。
- 身分 API 以反向代理終止 HTTPS 為正式部署方式；loopback 可使用 HTTP，非 loopback 的純 HTTP 登入預設拒絕。trusted proxy 必須明確設定，不得無條件信任所有 forwarded headers。
- Go GUI 新增第三種進入方式「登入校驗」：在既有本機設定/登入畫面增加入口，不取代本機 Vault；遠端登出後回原本畫面。
- 登入校驗只要求服務端完整 HTTP/HTTPS base URL、帳號與密碼。URL 與帳號保存為非敏感偏好；密碼永不保存。
- 登入成功後使用 15 分鐘短期 opaque access token 與 30 天 rotation refresh token；refresh token 使用 Windows DPAPI 保存，服務端資料庫只保存 token hash並偵測 refresh reuse。
- 每個子帳號可同時登入的裝置數由管理員調整，預設 3 台；每台裝置有獨立 session，可撤銷單一裝置或全部 session。
- 遠端登入使用獨立工作區，只顯示管理員指派的 S3/SQL 連線與允許操作，不顯示 SSH，不允許客戶端新增、修改或取得遠端連線密鑰。
- Node 服務端代理所有 S3/SQL 操作，Go 客戶端不得取得 S3 access key、secret key、SQL user 或 password；子帳號停權或 session 撤銷後必須立即失去代理存取能力。
- 權限粒度為「connection + operation」：S3 `read/write/delete`，SQL `tables/query/exec`。S3 connection 固定 bucket 與可選 base prefix；SQL connection 固定 database，帳號不得越過 connection 邊界。
- SQL `query` 在 read-only transaction 中執行並受 timeout/row limit 保護；`exec` 才允許具副作用的 SQL。S3 upload/download 以串流代理並受 byte limit 保護。
- 所有代理限制可由環境變數調整，預設採保守值：SQL 30 秒、最多 1000 rows、S3 單次 100 MiB、登入/API rate limit、稽核保留 90 天。
- 稽核只保存安全 metadata：時間、帳號、裝置/IP、操作、connection、成功/失敗、耗時、rows/bytes；SQL 只保存 statement hash/類型，不保存完整 SQL，S3 不保存物件內容。

## 實作決策

- 使用 `gioui.org` 建立桌面視窗與表單/分頁；以既有核心套件為 backend，不把連線邏輯塞入繪圖迴圈。
- 使用標準函式庫 AES-GCM 與 `golang.org/x/crypto/argon2`；密文格式包含版本、salt、nonce、ciphertext 與 recovery-wrapped key 所需 metadata。
- 使用 Windows DPAPI/安全儲存 adapter 保存 bootstrap secret；非 Windows 僅提供明確錯誤或測試替身，不宣稱跨平台安全儲存已完成。
- R2 vault 以單一版本化物件保存密文；SQL vault 使用參數化 SQL 建立/讀取單一版本化資料列。
- SSH profile 新增明確的 host key fingerprint 欄位；沒有已信任 fingerprint 時，連線必須由 GUI 顯示 fingerprint 並要求使用者確認，禁止 `InsecureIgnoreHostKey`。
- SSH 的 dial、handshake、Exec 與 PTY 操作都必須可取消或受 timeout 保護。
- DB DSN 使用 driver 提供的安全編碼方式；PostgreSQL SSL mode 由 profile 控制，預設不得偷偷停用 TLS。
- S3 List 使用 paginator，完整取得所有頁面並尊重 context cancellation。
- GUI 不以固定延遲模擬成功；所有狀態來自 backend 真實結果，錯誤要可見且不導致程式崩潰。
- 使用集中式翻譯 key 與英文/繁體中文字典，禁止在 Gio layout 中分散判斷語言。
- 語言偏好使用版本化 JSON，與 `metadata.json`、DPAPI `securestore/` 分離；只保存語言代碼，不保存任何 profile 或 bootstrap secret。
- 狀態與應用程式錯誤保存為翻譯 key，在繪製時依目前語言解析，確保切換後既有畫面訊息也能立即更新。
- 公開 repository 不追蹤 `config.json`、metadata、preferences、securestore、本機執行檔與測試執行檔；GitHub Actions 以 Windows runner 執行需要 Windows API 的 build/test。
- CI 的 i18n 測試由 GitHub Actions 執行；本機驗證僅使用不啟動 i18n 測試執行檔的格式化、靜態分析、建置與依賴檢查。
- README 的設定值必須以目前 `config.S3Profile`、`config.DBProfile`、S3 client 與 SQL DSN 實作為準，不記載 GUI 或 backend 尚未支援的選項。
- 文件必須區分 vault backend 與工作區 profile：前者保存加密 vault 密文，後者提供登入後的物件與資料庫操作；同一組遠端服務可使用不同 bucket、database 或最小權限帳號。
- 新服務採模組化單體，不拆微服務；domain/application 層不依賴 Fastify、grammY、SQLite、AWS 或 SQL driver，外部 adapter 透過介面注入。
- Fastify v5 route 使用完整 JSON Schema 驗證，API 固定在 `/api/v1`；測試使用 `fastify.inject`，不得依賴真實網路 port。
- Bot 僅接受私聊管理操作；按鈕與 slash commands 共用同一 application service，不各自複製權限與驗證邏輯。
- 密碼使用 Node 內建 `crypto.scrypt` 與 random salt；連線 secret 使用 random nonce AES-256-GCM；access/refresh token 使用 CSPRNG opaque token，資料庫只保存 SHA-256 hash。
- refresh token 每次使用即輪換；重用已輪換 token 時撤銷該 token family。密碼重設、帳號停權或刪除會撤銷所有 session。
- SQLite repository 使用 prepared statements、foreign keys、WAL 與明確 transaction；自動 migration 由 `schema_migrations` 追蹤版本。
- Go 端新增 remote-auth API client 與 session/resource 介面；Gio presentation 不直接組 HTTP request，現有本機 `app.Session` 行為保持不變。
- Go remote refresh token 與 device secret 使用既有 `securestore.Store`/Windows DPAPI；非敏感 server URL、account 與 device ID 使用獨立版本化偏好，不寫入現有 vault metadata。
- Bot/API/Go remote UI 的應用文案提供英文與繁體中文；外部 S3/SQL 原始診斷依既有原則保留原文。
- S3 adapter 使用 AWS SDK v3；SQL adapter 使用 `mysql2` 與 `pg`。Node package build 後以 `node dist/index.js` 啟動，測試使用 Node 內建 test runner。
- CI 新增 Node 22 job，執行 `npm ci`、format/lint、typecheck、test、build 與 production dependency audit；既有 Go CI 保留。

## 驗收標準

- `internal/ui` 與 Bubble Tea 相關正式相依移除；`go list -deps .` 不再包含 Bubble Tea/Lipgloss/Bubbles。
- Windows 可用 `CGO_ENABLED=0 go build -ldflags="-H windowsgui" -o s12ryt-ssh.exe .` 建置。
- 首次啟動可完成 vault backend 設定、bootstrap 測試、UUID/名稱/密碼建立、復原金鑰顯示與遠端密文寫入。
- 後續啟動可用名稱/密碼從遠端讀取密文並解密，錯誤密碼明確失敗；不在本機 JSON 保存 profile secret。
- SSH、S3/R2、MySQL/PostgreSQL 的成功與錯誤狀態均是真實 backend 結果。
- 新增行為均有自動化測試；每一個新測試先在 RED 階段因預期缺少行為而失敗，再在 GREEN 階段通過。
- 受影響測試、`go vet ./...`、`go build ./...`、可行的 `go test`/lint/靜態檢查均通過；CGO 或工具缺失要明確列為未完整驗證。
- README、建置指令、設定說明與專案結構反映 GUI/vault 現況，不再宣稱尚未實作的功能。
- 首次啟動預設英文；切換為繁體中文後，重新啟動仍維持繁體中文。
- 「中 / EN」按鈕在設定、登入、復原與工作區畫面均可使用，且切換後所有應用程式文案立即更新。
- 無效或缺少語言偏好檔時安全回退英文；偏好檔不得包含 profile、密碼、access key、secret key 或 bootstrap 資料。
- 英文與繁體中文字典對所有 GUI 使用的翻譯 key 具完整覆蓋，新增測試防止漏翻或回退成 key 字串。
- GitHub `main` 分支存在且遠端工作樹乾淨；CI 與 Release workflow 已提交並可由 GitHub Actions 觸發。
- README 可讓使用者依欄位表完成 R2、AWS S3、MinIO、MySQL 與 PostgreSQL 設定，並能判斷何時啟用 path-style、MySQL TLS mode 與 PostgreSQL SSL mode。
- README 明確說明 bootstrap secret 由 Windows DPAPI 保存、工作區 profile 進入遠端加密 vault，且範例不得包含真實憑證。
- `npm --prefix server run build` 可在 Node.js 22.13+ 完成，`npm --prefix server start` 可啟動 REST API 與 Telegram long polling；缺少必要環境變數時需明確失敗。
- 自動 migration 可從空 SQLite 建立 schema，重複啟動冪等；migration 中途失敗不得留下部分版本。
- 非 Telegram 管理員、群組訊息及偽造 callback 不可執行管理操作；繁中/英文按鈕與 slash commands 呼叫相同權限檢查。
- Bot 可完成子帳號與 S3/MySQL/PostgreSQL connection 全生命週期、連線測試、權限指派、session 撤銷及安全 metadata 稽核查詢。
- SQLite、log、Bot 回覆、API response 與稽核紀錄不得包含明文 S3/SQL secret、帳號密碼、access token 或 refresh token。
- `/api/v1/auth/login`、refresh、logout 與 password/reset 相關流程具 rate limit、constant-time secret comparison、裝置上限、停權及 refresh reuse 測試。
- 未授權 connection/operation、停權帳號、停用 connection、過期或撤銷 token均回明確 401/403/404，不得觸發後端 S3/SQL 呼叫。
- S3 proxy 的 list/upload/download/delete 尊重 base prefix、operation permission、100 MiB 預設限制與 context cancellation；不得把 remote credential 回傳客戶端。
- SQL proxy 的 tables/query/exec 尊重 database connection 與 operation permission；query 預設 read-only、30 秒、1000 rows，exec 需獨立權限。
- Go GUI 在本機 setup/login 畫面可進入「登入校驗」，能以 URL/帳密登入、處理長期密碼、載入 assigned resources、操作 S3/SQL、refresh、登出及被撤銷狀態。
- Go remote 模式不顯示 SSH、不允許編輯 connection secret；URL/account 可保存，refresh token 只存在 DPAPI，密碼不落盤。
- Node 與 Go 新增行為皆依 RED -> GREEN -> REFACTOR 建立測試；GitHub CI 的 Go 與 Node jobs、vet/typecheck/build、安全掃描均通過。

## 不在本次範圍

- 不建立獨立遠端帳號服務；vault backend 只提供密文儲存。
- 不做 R2 與 SQL 雙寫同步或衝突合併。
- 不保證 Linux/macOS 的安全儲存與 GUI 發行流程。
- 不提供瀏覽器管理後台；管理入口為 Telegram Bot，資料操作入口為 Go 桌面客戶端。
- 不下發 S3/SQL 原始憑證，不支援客戶端繞過服務端直接連線。
- 不支援 SSH 代理、R2/SQL vault 同步或將既有本機 vault 搬移到帳號服務。
- 不提供多實例 Node 服務、外部 Redis session、PostgreSQL 作服務自身資料庫或高可用叢集。
- 不保存完整 SQL、S3 object body 或其他業務資料到稽核紀錄。

## 2026-08-27 自主疊代升級：UI 合理性修正（本輪範圍）

使用者確認：修正範圍「全部含視覺級」；破壞性操作採「確認對話框（modal）」；S3 物件列表可點擊自動填入 Object key（本機與遠端）。

### 缺陷級（必修正）
- 單行 editor 已設定 Submit 但無人消費 SubmitEvent：所有表單按 Enter 無反應。修正：登入、遠端登入、建立 vault、復原輪換、SSH 連線與終端輸入支援 Enter 送出；動作抽成可測試方法與按鈕共用。
- 終端/輸出區以 MaxLines 靜態文字顯示、不可滾動：終端超過 100 行後看不到新輸出。修正：改為可滾動列表、終端貼底跟隨、緩衝上限避免無限成長。
- `ui.list` 被設定表單、本機側欄、遠端側欄三處共用造成滾動位置跨畫面污染。修正：各視圖獨立 list。
- busy 期間按鈕外觀不變、點擊被靜默忽略。修正：busy 時按鈕視覺禁用（灰）；語言切換、SSH Close（取消連線）與 modal 按鈕保持可用；本機登出在 busy 時由禁用涵蓋競爭問題。
- 錯誤訊息截斷 4 行。修正：完整換行顯示。

### UX 準則級
- S3 Delete 與 SQL Exec（本機+遠端）以遮罩式確認對話框二次確認；確認接受鍵與 Delete/Exec 按鈕採危險色樣式；物件 key 與 SQL 敘述空白時先回驗證錯誤。
- busy 時狀態列顯示 indeterminate 進度條。
- DB Type 由自由文字改為 MySQL/PostgreSQL 選擇按鈕；載入既有 profile 時 postgres/postgresql/pg 正規化為 postgres。
- 本機 profile 側欄空狀態提示；復原金鑰一鍵複製到剪貼簿。

### 視覺級
- 終端輸出與 SQL 欄位使用等寬字型（既有依賴 gofont GoMono，零新增依賴）。
- 密碼欄位提供 Show/Hide 顯示切換。
- PTY 輸出過濾 ANSI escape 序列與歸位字元，維持基本可讀。

### 驗收標準（本輪）
- 上述行為均有單元測試先 RED 後 GREEN；渲染層接線以 build + 既有 GUI 測試 + 程式碼審查驗證並明列例外。
- `go test ./internal/gui` 及其他本機可跑套件全綠；`go vet ./...`、`go build ./...`、`gofmt` 通過；本機不執行 `internal/i18n` 測試（防毒限制），i18n 新 key 交付 CI 驗證。
- 新增 UI 字串均加入英/繁字典；外部服務原始錯誤保持原文。
- 不改動本機 Vault / 遠端服務的公開行為契約；不 commit/push（未獲指示）。
