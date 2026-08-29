# s12ryt-ssh

Windows 桌面 Go 應用程式：以遠端登入服務為唯一入口的安全工作區，整合 SSH 主機管理與連線、S3/R2 物件儲存與 MySQL/PostgreSQL 資料庫操作。GUI 使用 Gio。

## 遠端登入

本應用程式以遠端帳號登入作為唯一入口：

1. 啟動後輸入驗證服務 URL、帳號與密碼（密碼不會儲存）。
2. 服務端核發短期 access token（15 分鐘）與輪換式 refresh token（30 天，以 Windows DPAPI 加密保存在本機）。
3. 之後啟動可一鍵還原 session。

驗證服務（Node.js + Fastify + SQLite + Telegram Bot 管理介面）位於獨立倉庫：https://github.com/s12ryt/s12ryt-ssh-auth-server

## SSH 主機

登入後（帳號需開啟 SSH 功能）可在「SSH」分頁自助管理私人主機：

- 每帳號最多 50 台；名稱、主機、連接埠、使用者名稱必填。
- 認證支援密碼或私鑰（可加密碼短語），兩者至少其一。
- 憑證以 AES-256-GCM 加密保存在服務端；連線時經 HTTPS 下發，只在記憶體中使用。
- 首次連線會顯示主機金鑰指紋（TOFU），確認後存回服務端；主機或連接埠變更時指紋會重設。
- 連線由客戶端直連（PTY 終端機），不經服務端轉發。
- 管理員可用 Bot 指令 `/ssh_enable`、`/ssh_disable` 開關帳號 SSH 功能；關閉時 GUI 隱藏 SSH 分頁，API 拒絕存取。

注意：撤銷 session 或關閉 SSH 功能不會切斷已建立的 SSH 連線，只會阻止後續憑證下發。

## 儲存與資料庫

S3/R2 與 SQL 連線由管理員設定並指派：

- 連線憑證只存在服務端，永不下發用戶端。
- 所有 S3/SQL 操作都由服務端代理執行，依指派的權限（`s3.read` / `s3.write` / `s3.delete`、`sql.tables` / `sql.query` / `sql.exec`）控制。
- 上傳/下載有位元組上限，SQL 查詢有逾時與列數限制。

## 安全模型

- 服務端是唯一信任邊界。S3/SQL 憑證絕不離開服務端；SSH 憑證是唯一例外（個人主機，HTTPS 下發、記憶體使用、每次下發都寫稽核）。
- 本機只保存：驗證服務 URL、帳號名稱、裝置 ID（preferences JSON）與 DPAPI 加密的 refresh token。
- 密碼與 SSH 憑證內容不落地。

## 建置

需求：Go（Windows）。

```
go build .
```

執行檔即為 GUI 應用程式。驗證服務請另行部署 s12ryt-ssh-auth-server。

## 專案結構

```
main.go                 應用程式入口（路徑與服務組裝）
internal/config         SSH profile 型別（含私鑰內容 KeyData）
internal/gui            Gio GUI（遠端登入、遠端工作區、SSH 主機與終端機）
internal/i18n           英文/繁體中文字典
internal/remote         遠端 API client（登入、token 輪換、資源、SSH hosts、S3/SQL 代理）
internal/securestore    DPAPI 保護的 secret 儲存
internal/ssh            SSH 客戶端（密碼/私鑰認證、TOFU 指紋、PTY 終端機）
```
