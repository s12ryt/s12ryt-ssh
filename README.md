# s12ryt-ssh

Windows 優先的 Go 桌面 SSH 工作區，使用 Gio GUI，並整合 S3/R2 物件儲存與 MySQL/PostgreSQL 資料庫操作。

所有 SSH、儲存與資料庫 profile 都會先在本機加密，再以單一密文 vault 保存到使用者選定的遠端 backend。bootstrap 連線資料使用 Windows DPAPI 保存，不寫入明文 JSON。

## 功能

- SSH 密碼或私鑰認證、主機 key fingerprint 驗證、互動式 PTY 終端與視窗調整。
- S3 相容儲存，包括 Cloudflare R2、AWS S3、MinIO：列表、上傳、下載與刪除。
- MySQL/PostgreSQL：資料表列表、Query 與 Exec。
- 可選的 Node.js 22 身分驗證服務：由 Telegram Bot 管理子帳號、S3/SQL 連線、細粒度權限、裝置 session 與安全稽核。
- GUI 的「登入校驗」模式只接收服務 URL、帳號與密碼；登入後只顯示已授權的 S3/SQL 資源，遠端憑證不會下發到桌面客戶端。
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

Node 身分驗證服務：

```powershell
Set-Location server
npm ci
npm run typecheck
npm run lint
npm test
npm run build
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

既有本機 Vault 與遠端登入校驗是兩個獨立入口：

- **本機 Vault**：使用自己的加密 vault profile，支援 SSH、S3/R2 與 SQL。
- **登入校驗**：連線到 Node 身分驗證服務，只顯示管理員透過 Telegram Bot 指派的 S3/SQL connection 與允許操作，不提供 SSH，也不能查看或修改遠端 connection secret。

## 登入校驗與 Telegram 管理服務

Node 服務是遠端 S3/SQL credentials 的唯一信任邊界。桌面 GUI 只持有短期 access token；rotation refresh token 使用 Windows DPAPI 保存。帳號密碼不保存，S3 access key、secret key、SQL user 與 password 都不會出現在 Go 客戶端 API model 或回應中。

快速啟動摘要：

1. 安裝 Node.js `22.13+` 與 npm。
2. 進入 `server/`，執行 `npm ci`。
3. 將 `.env.example` 複製為 `.env`，填入 Telegram Bot token、最高管理員 numeric user ID 與 32-byte Base64 主金鑰。
4. 執行 `npm run build` 與 `npm start`。
5. 正式環境使用 Caddy、Nginx 或 Cloudflare Tunnel 終止 HTTPS，並只信任明確設定的 proxy。
6. 最高管理員在 Telegram 私聊 Bot 建立子帳號、connection 與 grant；使用者再從 GUI 點選「登入校驗」。

完整的環境變數、Bot 指令、connection 精靈、REST API、HTTPS 部署與安全模型請見 [`server/README.md`](server/README.md)。

產生 32-byte Base64 主金鑰的 Node 指令：

```powershell
node -e "console.log(require('node:crypto').randomBytes(32).toString('base64'))"
```

主金鑰用於 AES-256-GCM 加密 SQLite 中的 S3/SQL connection secret。遺失主金鑰將無法解密既有 connection；洩漏主金鑰則必須立即輪換所有遠端 credentials。

## 設定模型：Vault 與工作區 Profile

應用程式將「用來找到加密 vault 的連線」與「登入後實際操作的連線」分開處理：

| 類型           | 使用時機                      | 保存位置                                                              | 用途                           |
| -------------- | ----------------------------- | --------------------------------------------------------------------- | ------------------------------ |
| Vault backend  | 首次設定、登入、復原          | bootstrap 連線資料在 Windows DPAPI；profile 密文在選定的 R2/S3 或 SQL | 保存單一加密 vault             |
| 工作區 profile | 登入後的 SSH、S3/R2、SQL 分頁 | 遠端加密 vault                                                        | 執行終端、物件儲存與資料庫操作 |

Vault backend 只選擇一種，不會同時寫入 R2/S3 與 SQL。工作區 profile 的名稱、密碼、access key、secret key 等內容會與其他 profile 一起放進遠端 AES-GCM 密文；本機 `metadata.json` 不保存這些內容。

可以使用同一個 S3 bucket 或 SQL database，但正式環境建議為 vault 與工作區分配不同 bucket、database 或最小權限帳號，降低誤刪或權限過大的影響。

## R2 / S3 相容設定

首次設定精靈的 R2/S3 vault backend 與登入後的 S3/R2 工作區 profile 使用相同的 S3 相容欄位。`Name` 只用於工作區 profile；Vault 設定不需要名稱欄位。

| 欄位         | 必填       | 說明                                                                                                  |
| ------------ | ---------- | ----------------------------------------------------------------------------------------------------- |
| `Name`       | 工作區需要 | GUI 中辨識 profile 的名稱。                                                                           |
| `Endpoint`   | 是         | S3 API endpoint，必須是 `http://` 或 `https://` URL。不要填控制台、公開物件 URL 或 website endpoint。 |
| `Region`     | 建議       | 供 SDK 建立請求的區域；依服務商要求填寫。R2 通常使用 `auto`。                                         |
| `Access Key` | 是         | S3 相容服務的 static access key。                                                                     |
| `Secret Key` | 是         | S3 相容服務的 static secret key。                                                                     |
| `Bucket`     | 是         | 只填 bucket 名稱，不要加 `s3://`、endpoint 或結尾斜線。                                               |
| `Path-style` | 依服務     | On 使用 `endpoint/bucket/key`；Off 使用 `bucket.endpoint/key`。                                       |

本程式沒有 session token 欄位，因此不支援需要額外 session token 的 AWS STS temporary credentials。請使用服務商允許的 static credentials，並依實際用途限制 bucket、prefix 與操作權限。

HTTPS endpoint 的憑證必須由 Windows/系統信任鏈信任；程式目前沒有自訂 CA 檔案欄位。`http://` 只適合受信任的內部網路，不能用於一般網際網路傳輸。

### Cloudflare R2

在 Cloudflare Dashboard 建立 R2 API Token 時，至少要讓該 token 能對目標 bucket 執行 vault 或工作區所需的列出、讀取、寫入與刪除操作。建議將 token 限制在單一 bucket，不要直接使用與帳號管理相關的 API token。

| 欄位       | 範例                                            |
| ---------- | ----------------------------------------------- |
| Endpoint   | `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` |
| Region     | `auto`                                          |
| Access Key | R2 API Token 的 Access Key ID                   |
| Secret Key | R2 API Token 的 Secret Access Key               |
| Bucket     | `<bucket-name>`                                 |
| Path-style | On                                              |

`<ACCOUNT_ID>` 是 Cloudflare Account ID，不是 bucket 名稱。R2 的 `auto` 是建議值；R2 也接受部分相容用法中的其他區域別名，但應以服務商設定為準。

若用 R2 保存 vault，程式會在 bucket 中以 `s12ryt/vault/<vault-id>.json` 保存單一密文，測試連線時會列出 `s12ryt/vault/` prefix。

### AWS S3

以下以 `ap-northeast-1` 為例，請將 endpoint 與 region 換成 bucket 所在區域：

| 欄位       | 範例                                                |
| ---------- | --------------------------------------------------- |
| Endpoint   | `https://s3.ap-northeast-1.amazonaws.com`           |
| Region     | `ap-northeast-1`                                    |
| Access Key | IAM user 或其他受控 static credential 的 access key |
| Secret Key | 對應的 secret key                                   |
| Bucket     | `my-app-bucket`                                     |
| Path-style | Off                                                 |

AWS 一般使用 virtual-host style，因此 Path-style 建議關閉。請使用 S3 API endpoint，不要使用 `https://<bucket>.s3-website-...` website endpoint，也不要把 bucket 名稱重複放進 Endpoint。

AWS 不支援以本程式欄位直接填入 STS session token。若必須使用短期憑證，需先確認服務端是否提供不需要 session token 的替代憑證配置；不要將未支援的 token 拼接到 Secret Key。

### MinIO

以一個使用 HTTPS 的 MinIO S3 API 服務為例：

| 欄位       | 範例                                 |
| ---------- | ------------------------------------ |
| Endpoint   | `https://minio.example.com:9000`     |
| Region     | `us-east-1`，或部署環境指定的 region |
| Access Key | MinIO access key                     |
| Secret Key | MinIO secret key                     |
| Bucket     | `my-app-bucket`                      |
| Path-style | On                                   |

MinIO 常見的 S3 API port 是 `9000`；`9001` 通常是管理控制台，不是本程式要連線的 S3 API endpoint。若 MinIO 已配置 bucket DNS、憑證與 virtual-host style，Path-style 才可改為 Off。

自簽憑證必須先加入 Windows/系統信任鏈，因為本程式沒有自訂 CA 欄位。若只在受信任的內部測試網路使用 HTTP，仍應避免將 access key、secret key 或 vault 密文暴露給不受信任的網路節點。

### 通用 S3 相容服務

其他 S3 相容服務可依同一組欄位設定：從服務商取得 S3 API endpoint、region、static access key、static secret key 與 bucket 名稱，再依服務商文件決定 Path-style。

| 情況                                        | Path-style 建議                           |
| ------------------------------------------- | ----------------------------------------- |
| 服務商只提供 `endpoint/bucket/key`          | On                                        |
| bucket DNS 與 HTTPS wildcard 憑證已正確配置 | 可使用 Off                                |
| 不確定服務商支援哪種 addressing             | 先依服務商文件；R2/自架 MinIO 通常先用 On |

### Vault 與工作區需要的 S3 API

Vault backend 需要能對目標 bucket 的 `s12ryt/vault/` prefix 執行：

- `ListObjectsV2`：測試連線與列出 vault prefix。
- `GetObject`：登入或復原時讀取 vault 密文。
- `PutObject`：建立或更新 vault 密文。
- `DeleteObject`：建立失敗時的清理，以及刪除流程所需的回滾能力。

工作區 S3/R2 profile 需要對其指定 bucket 與所需 prefix 執行列出、讀取、寫入與刪除。若服務商支援 prefix 限制，建議依使用者實際操作範圍設定最小權限。

## SQL 設定

SQL 設定同時用於首次設定的 SQL Vault backend，以及登入後的 MySQL/PostgreSQL 工作區 profile。程式以個別欄位建立 DSN，會處理帳號、密碼與 database 名稱中的特殊字元；不需要也不能貼上自訂 DSN。

| 欄位       | 說明                                                              |
| ---------- | ----------------------------------------------------------------- |
| `Name`     | 工作區 profile 的顯示名稱；SQL Vault 設定不使用此欄位。           |
| `Type`     | `mysql`、`postgres`、`postgresql` 或 `pg`。                       |
| `Host`     | TCP 主機名稱或 IP。                                               |
| `Port`     | TCP port；MySQL/MariaDB 通常是 `3306`，PostgreSQL 通常是 `5432`。 |
| `User`     | 資料庫使用者。                                                    |
| `Password` | 資料庫密碼。                                                      |
| `Database` | 要連線的 database 名稱。                                          |
| `TLS Mode` | MySQL/MariaDB 的 TLS 選項；空白時預設為 `true`。                  |
| `SSL Mode` | PostgreSQL 的 SSL 選項；空白時預設為 `require`。                  |

目前只支援 TCP `Host`/`Port`，不支援 Unix socket、應用程式內建 SSH tunnel 或額外 DSN query parameter。資料庫憑證會隨工作區 profile 放入遠端加密 vault；不會寫入本機明文 metadata。

### MySQL / MariaDB

以下是一般 MySQL/MariaDB 工作區 profile 的參考值：

| 欄位     | 範例                |
| -------- | ------------------- |
| Name     | `production-mysql`  |
| Type     | `mysql`             |
| Host     | `mysql.example.com` |
| Port     | `3306`              |
| User     | `app_user`          |
| Password | `使用者自己的密碼`  |
| Database | `app`               |
| TLS Mode | `true`              |

`TLSMode` 可使用下列值：

| 值            | 行為與建議                                                    |
| ------------- | ------------------------------------------------------------- |
| 空白或 `true` | 使用 TLS；程式預設為 `true`，一般應優先使用。                 |
| `false`       | 明文連線；不建議在不完全受信任的網路使用。                    |
| `skip-verify` | 使用 TLS 但跳過憑證驗證；只適合受控測試環境，不建議正式使用。 |
| `preferred`   | 優先嘗試 TLS，但可能降級為明文；不建議。                      |

首次設定精靈的 SQL Vault 表單目前沒有獨立的 MySQL TLS Mode 欄位。選擇 SQL Vault 且 Type 為 MySQL 時，空值會依程式安全預設使用 `true`；若需要其他 TLS 模式，請在登入後建立或編輯工作區 SQL profile。程式沒有自訂 CA 或 TLS 設定名稱欄位，資料庫伺服器憑證必須符合系統可用的信任設定。

### PostgreSQL

以下是 PostgreSQL 工作區 profile 的參考值：

| 欄位     | 範例                   |
| -------- | ---------------------- |
| Name     | `production-postgres`  |
| Type     | `postgres`             |
| Host     | `postgres.example.com` |
| Port     | `5432`                 |
| User     | `app_user`             |
| Password | `使用者自己的密碼`     |
| Database | `app`                  |
| SSL Mode | `verify-full`          |

`SSLMode` 支援 PostgreSQL 常見的下列值：

| 值            | 行為與建議                                                                         |
| ------------- | ---------------------------------------------------------------------------------- |
| `disable`     | 停用 TLS；僅適合完全受信任的本機或隔離網路。                                       |
| `allow`       | 先嘗試非 TLS，再視情況使用 TLS；不建議。                                           |
| `prefer`      | 優先 TLS，但可降級為非 TLS；不建議。                                               |
| `require`     | 要求加密連線；程式預設值。未提供完整憑證驗證設定時，不等同於最嚴格的主機名稱驗證。 |
| `verify-ca`   | 驗證受信任的 CA，但不驗證主機名稱。                                                |
| `verify-full` | 驗證 CA 並驗證主機名稱；公開網路優先使用。                                         |

程式目前沒有 `sslrootcert`、`sslcert` 或 `sslkey` 等自訂憑證路徑欄位。使用 `verify-full` 前，請先讓 PostgreSQL 憑證鏈可由執行環境信任，且憑證中的主機名稱要與 `Host` 相符。

### SQL Vault

選擇 SQL Vault 時，設定精靈會先以 `Ping` 測試資料庫，再在建立或存取 vault 時使用參數化 SQL。SQL 使用者至少需要：

- 連線與 `Ping` 權限。
- 建立 `s12ryt_vault` 表的權限。
- 該表的 `SELECT`、`INSERT`、`UPDATE` 與 `DELETE` 權限。

程式使用的表結構如下：

```sql
CREATE TABLE IF NOT EXISTS s12ryt_vault (
  vault_id VARCHAR(255) PRIMARY KEY,
  payload TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL
)
```

每個 vault 以一個 UUID 對應一列，`payload` 是版本化的 AES-GCM 加密 envelope，不是可直接查閱的 profile JSON。SQL backend 使用參數化查詢進行新增、更新、讀取與刪除；不會把 vault ID 直接拼接進 SQL。

### 工作區 SQL Profile

登入後的 SQL 分頁提供：

- `Tables`：列出目前 database 的資料表。MySQL 使用目前 database；PostgreSQL 使用 `public` schema。
- `Query`：執行讀取查詢並在 GUI 顯示結果。
- `Exec`：執行不回傳資料列的 SQL，並顯示受影響列數與可用的 insert ID。

工作區 profile 所使用的資料庫權限等同於該 SQL 使用者的權限。請使用最小權限帳號，並在執行 `Exec` 或其他修改資料的語句前確認目標 database 與 SQL 內容。

## 設定安全摘要

- Windows bootstrap 連線資料使用 DPAPI 保存；`metadata.json` 只含 vault ID、名稱、backend 類型與 securestore key。
- SSH、S3/R2 與 SQL 工作區 profile 會在本機加密後保存到遠端單一 vault；遠端只看到加密 envelope。
- 不要把真實 access key、secret key、資料庫密碼、復原金鑰或完整 endpoint 憑證貼到 README、issue、日誌或螢幕截圖。
- 公開網路優先使用 HTTPS、MySQL TLS 與 PostgreSQL `verify-full`。`false`、`skip-verify`、`preferred` 或 PostgreSQL `disable`/可降級模式都會降低傳輸安全性。
- 使用不同用途的 bucket/database 與最小權限帳號，並定期依服務商流程輪換 bootstrap、S3 與 SQL 憑證。

## 官方參考

- [Cloudflare R2 S3 API](https://developers.cloudflare.com/r2/api/s3/api/)
- [AWS S3 service endpoints](https://docs.aws.amazon.com/general/latest/gr/s3.html)
- [MinIO S3 API](https://www.min.io/product/aistor/s3-api)
- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
- [pgx PostgreSQL driver](https://github.com/jackc/pgx)

## 本機資料

應用程式使用作業系統使用者設定目錄：

- `metadata.json`：vault ID、名稱、backend 類型與 securestore key，不包含 profile 或 bootstrap secret。
- `securestore/`：Windows DPAPI 保護的 bootstrap 連線資料。
- `preferences.json`：版本化語言偏好，只保存 `en` 或 `zh-TW`，不保存任何憑證或 profile。
- `remote-preferences.json`：登入校驗的服務 URL、帳號與隨機 device ID，不保存密碼或 token。
- `securestore/` 中獨立 namespaced 項目：Windows DPAPI 保護的 rotation refresh token；短期 access token 只存在記憶體。

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
internal/remote/          遠端登入、token 輪換與 S3/SQL 代理 client
server/                   Node 22 Fastify、Telegram Bot、SQLite 與代理服務
agent/                   需求契約與工作紀錄
```

## 測試

```powershell
go test ./... -count=1
go vet ./...
```

測試涵蓋 vault 加密與復原輪換、securestore、SSH fingerprint/逾時/PTY、S3 分頁、資料庫 DSN/TLS/關閉防護、service workflow 與 GUI state/profile 行為。

Node 服務驗證：

```powershell
Set-Location server
npm run format:check
npm run lint
npm run typecheck
npm test
npm run build
npm audit --omit=dev --audit-level=high
```

本機 Windows 防毒曾誤判 `internal/i18n` 的 Go 測試執行檔，因此該 package 的測試由 GitHub Actions Windows runner 執行；其他 Go 套件仍可在本機個別驗證。

## 依賴

- [Gio](https://gioui.org/)：桌面 GUI。
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh)：SSH。
- [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2)：S3/R2。
- [database/sql](https://pkg.go.dev/database/sql)：資料庫連線抽象。
- [Fastify](https://fastify.dev/)：Node 身分驗證 REST API。
- [grammY](https://grammy.dev/)：Telegram Bot long polling 與 inline keyboard。
- [node:sqlite](https://nodejs.org/api/sqlite.html)：服務自身的 SQLite 儲存。
