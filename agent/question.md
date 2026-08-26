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

## 不在本次範圍

- 不建立獨立遠端帳號服務；vault backend 只提供密文儲存。
- 不做 R2 與 SQL 雙寫同步或衝突合併。
- 不保證 Linux/macOS 的安全儲存與 GUI 發行流程。
