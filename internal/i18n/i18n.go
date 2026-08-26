// Package i18n contains the application's supported languages and translations.
package i18n

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Language string

const (
	English            Language = "en"
	TraditionalChinese Language = "zh-TW"
)

type Preferences struct {
	Version  int      `json:"version"`
	Language Language `json:"language"`
}

const preferencesVersion = 1

type Key string

const (
	KeyCreateVault       Key = "create_vault"
	KeyVaultDescription  Key = "vault_description"
	KeyVaultName         Key = "vault_name"
	KeyVaultPassword     Key = "vault_password"
	KeyTestConnection    Key = "test_connection"
	KeyCreate            Key = "create"
	KeyUnlockVault       Key = "unlock_vault"
	KeyUnlockDescription Key = "unlock_description"
	KeySignIn            Key = "sign_in"
	KeyUseRecovery       Key = "use_recovery"
	KeySaveRecovery      Key = "save_recovery"
	KeyRecoverVault      Key = "recover_vault"
	KeyOneTimeRecovery   Key = "one_time_recovery"
	KeyContinueSignIn    Key = "continue_sign_in"
	KeyNewVaultName      Key = "new_vault_name"
	KeyNewVaultPassword  Key = "new_vault_password"
	KeyRotateCredentials Key = "rotate_credentials"
	KeyBackToSignIn      Key = "back_to_sign_in"
	KeySSHTerminal       Key = "ssh_terminal"
	KeyStorage           Key = "storage"
	KeySQLDatabase       Key = "sql_database"
	KeyLogOut            Key = "log_out"
	KeySSHProfile        Key = "ssh_profile"
	KeyStorageProfile    Key = "storage_profile"
	KeyDatabaseProfile   Key = "database_profile"
	KeyNewProfile        Key = "new_profile"
	KeySaveProfile       Key = "save_profile"
	KeyConnect           Key = "connect"
	KeyClose             Key = "close"
	KeyTerminalInput     Key = "terminal_input"
	KeySendTerminal      Key = "send_terminal"
	KeyTerminalOutput    Key = "terminal_output"
	KeyEndpoint          Key = "endpoint"
	KeyRegion            Key = "region"
	KeyBucket            Key = "bucket"
	KeyAccessKey         Key = "access_key"
	KeySecretKey         Key = "secret_key"
	KeyPathStyleOff      Key = "path_style_off"
	KeyPathStyleOn       Key = "path_style_on"
	KeyListPrefix        Key = "list_prefix"
	KeyObjectKey         Key = "object_key"
	KeyLocalPath         Key = "local_path"
	KeyInlineData        Key = "inline_data"
	KeyRefreshList       Key = "refresh_list"
	KeyUpload            Key = "upload"
	KeyDownload          Key = "download"
	KeyDelete            Key = "delete"
	KeyStorageOutput     Key = "storage_output"
	KeyType              Key = "type"
	KeyHost              Key = "host"
	KeyPort              Key = "port"
	KeyUser              Key = "user"
	KeyPassword          Key = "password"
	KeyDatabase          Key = "database"
	KeySSLMode           Key = "ssl_mode"
	KeyMySQLTLS          Key = "mysql_tls"
	KeySQLStatement      Key = "sql_statement"
	KeyListTables        Key = "list_tables"
	KeyRunQuery          Key = "run_query"
	KeyRunExec           Key = "run_exec"
	KeyDatabaseOutput    Key = "database_output"
	KeyR2S3Vault         Key = "r2_s3_vault"
	KeySQLVault          Key = "sql_vault"
	KeyStatusCreate      Key = "status_create"
	KeyStatusSignIn      Key = "status_sign_in"
	KeyStatusUnlock      Key = "status_unlock"
	KeyStatusRecovery    Key = "status_recovery"
	KeyStatusReady       Key = "status_ready"
	KeyStatusWorking     Key = "status_working"
	KeyStatusFailed      Key = "status_failed"
	KeyStatusSSHClosed   Key = "status_ssh_closed"
	KeyStatusConnected   Key = "status_connected"
	KeyStatusTest        Key = "status_test"
	KeyStatusUploading   Key = "status_uploading"
	KeyStatusDownloading Key = "status_downloading"
	KeyStatusDeleting    Key = "status_deleting"
	KeyStatusListing     Key = "status_listing"
	KeyStatusDBTables    Key = "status_db_tables"
	KeyStatusDBQuery     Key = "status_db_query"
	KeyStatusDBExec      Key = "status_db_exec"
	KeyStatusSaving      Key = "status_saving"
	KeyNoObjects         Key = "no_objects"
	KeyNoRows            Key = "no_rows"
	KeyLanguageToggle    Key = "language_toggle"
	KeyAppSubtitle       Key = "app_subtitle"
	KeySecureWorkspace   Key = "secure_workspace"
	KeyDefaultSSL        Key = "default_ssl"
	KeySSHRequired       Key = "ssh_required"
	KeySSHAuthRequired   Key = "ssh_auth_required"
	KeyStorageRequired   Key = "storage_required"
	KeyDatabaseRequired  Key = "database_required"
	KeyPortInvalid       Key = "port_invalid"
	KeyOperationFailed   Key = "operation_failed"
	KeyDownloaded        Key = "downloaded"
	KeyRowsAffected      Key = "rows_affected"
	KeyPreview           Key = "preview"
	KeyCreateHint        Key = "create_hint"
	KeyBytes             Key = "bytes"
	KeyLastInsertID      Key = "last_insert_id"
	KeyRecoverySaved     Key = "recovery_saved"
	KeyStatusRotating    Key = "status_rotating"
	KeyStatusConnecting  Key = "status_connecting"
	KeySSHProfiles       Key = "ssh_profiles"
	KeyStorageProfiles   Key = "storage_profiles"
	KeyDatabaseProfiles  Key = "database_profiles"
	KeyNew               Key = "new"
	KeyName              Key = "name"
	KeyKeyPath           Key = "key_path"
	KeyKeyPassphrase     Key = "key_passphrase"
	KeyHostFingerprint   Key = "host_fingerprint"
	KeyVaultBucket       Key = "vault_bucket"
	KeyDownloadedTo      Key = "downloaded_to"
	KeyPreferenceSave    Key = "preference_save_failed"
	KeyVaultRequired     Key = "vault_credentials_required"
	KeyS3Bootstrap       Key = "s3_bootstrap_required"
	KeySQLBootstrap      Key = "sql_bootstrap_required"
	KeyRecoveryRequired  Key = "recovery_credentials_required"
)

var translations = map[Language]map[Key]string{
	English: {
		KeyCreateVault: "Create encrypted vault", KeyVaultDescription: "Bootstrap credentials are protected by Windows DPAPI; profiles are encrypted before upload.", KeyVaultName: "Vault name", KeyVaultPassword: "Vault password", KeyTestConnection: "Test connection", KeyCreate: "Create vault", KeyUnlockVault: "Unlock your vault", KeyUnlockDescription: "Your profiles stay encrypted at rest and are decrypted only after sign-in.", KeySignIn: "Sign in", KeyUseRecovery: "Use recovery key", KeySaveRecovery: "Save your recovery key", KeyRecoverVault: "Recover vault access", KeyOneTimeRecovery: "One-time recovery key", KeyContinueSignIn: "Continue to sign in", KeyNewVaultName: "New vault name", KeyNewVaultPassword: "New vault password", KeyRotateCredentials: "Rotate credentials", KeyBackToSignIn: "Back to sign in", KeySSHTerminal: "SSH terminal", KeyStorage: "S3 / R2", KeySQLDatabase: "SQL database", KeyLogOut: "Log out", KeySSHProfile: "SSH profile", KeyStorageProfile: "S3 / R2 profile", KeyDatabaseProfile: "SQL profile", KeyNewProfile: "New profile", KeySaveProfile: "Save profile", KeyConnect: "Connect", KeyClose: "Close", KeyTerminalInput: "Terminal input", KeySendTerminal: "Send to terminal", KeyTerminalOutput: "Terminal output", KeyEndpoint: "Endpoint", KeyRegion: "Region", KeyBucket: "Bucket", KeyAccessKey: "Access key", KeySecretKey: "Secret key", KeyPathStyleOff: "Path-style requests: off", KeyPathStyleOn: "Path-style requests: on", KeyListPrefix: "List prefix", KeyObjectKey: "Object key", KeyLocalPath: "Local path", KeyInlineData: "Inline upload data", KeyRefreshList: "Refresh list", KeyUpload: "Upload", KeyDownload: "Download", KeyDelete: "Delete", KeyStorageOutput: "Objects and operation output", KeyType: "Type", KeyHost: "Host", KeyPort: "Port", KeyUser: "User", KeyPassword: "Password", KeyDatabase: "Database", KeySSLMode: "SSL mode", KeyMySQLTLS: "MySQL TLS mode", KeySQLStatement: "SQL query or statement", KeyListTables: "List tables", KeyRunQuery: "Run query", KeyRunExec: "Run exec", KeyDatabaseOutput: "Database output", KeyR2S3Vault: "R2 / S3 vault", KeySQLVault: "SQL vault", KeyStatusCreate: "Creating encrypted vault...", KeyStatusSignIn: "Sign in to unlock your encrypted vault.", KeyStatusUnlock: "Unlocking encrypted vault...", KeyStatusRecovery: "Enter the one-time recovery key and new credentials.", KeyStatusReady: "Ready.", KeyStatusWorking: "Working...", KeyStatusFailed: "Operation failed.", KeyStatusSSHClosed: "SSH connection closed.", KeyStatusConnected: "Connected to ", KeyStatusTest: "Testing vault connection...", KeyStatusUploading: "Uploading object...", KeyStatusDownloading: "Downloading object...", KeyStatusDeleting: "Deleting object...", KeyStatusListing: "Listing remote objects...", KeyStatusDBTables: "Loading database tables...", KeyStatusDBQuery: "Running database query...", KeyStatusDBExec: "Executing database statement...", KeyStatusSaving: "Saving encrypted profiles...", KeyNoObjects: "No objects found.", KeyNoRows: "No rows returned.", KeyLanguageToggle: "中", KeyAppSubtitle: "Secure remote workspace", KeySecureWorkspace: "Secure remote workspace", KeyDefaultSSL: "PostgreSQL SSL mode (default require)",
	},
	TraditionalChinese: {
		KeyCreateVault: "建立加密 Vault", KeyVaultDescription: "Bootstrap 憑證由 Windows DPAPI 保護；Profile 上傳前會先加密。", KeyVaultName: "Vault 名稱", KeyVaultPassword: "Vault 密碼", KeyTestConnection: "測試連線", KeyCreate: "建立 Vault", KeyUnlockVault: "解鎖 Vault", KeyUnlockDescription: "Profile 靜態保存時維持加密，登入後才會解密。", KeySignIn: "登入", KeyUseRecovery: "使用復原金鑰", KeySaveRecovery: "保存復原金鑰", KeyRecoverVault: "復原 Vault 存取權", KeyOneTimeRecovery: "一次性復原金鑰", KeyContinueSignIn: "繼續登入", KeyNewVaultName: "新的 Vault 名稱", KeyNewVaultPassword: "新的 Vault 密碼", KeyRotateCredentials: "輪換憑證", KeyBackToSignIn: "返回登入", KeySSHTerminal: "SSH 終端", KeyStorage: "S3 / R2", KeySQLDatabase: "SQL 資料庫", KeyLogOut: "登出", KeySSHProfile: "SSH Profile", KeyStorageProfile: "S3 / R2 Profile", KeyDatabaseProfile: "SQL Profile", KeyNewProfile: "新增 Profile", KeySaveProfile: "保存 Profile", KeyConnect: "連線", KeyClose: "關閉", KeyTerminalInput: "終端輸入", KeySendTerminal: "傳送至終端", KeyTerminalOutput: "終端輸出", KeyEndpoint: "端點", KeyRegion: "區域", KeyBucket: "Bucket", KeyAccessKey: "Access Key", KeySecretKey: "Secret Key", KeyPathStyleOff: "Path-style 請求：關閉", KeyPathStyleOn: "Path-style 請求：開啟", KeyListPrefix: "列表前綴", KeyObjectKey: "物件 Key", KeyLocalPath: "本機路徑", KeyInlineData: "直接上傳資料", KeyRefreshList: "重新整理列表", KeyUpload: "上傳", KeyDownload: "下載", KeyDelete: "刪除", KeyStorageOutput: "物件與操作輸出", KeyType: "類型", KeyHost: "主機", KeyPort: "連接埠", KeyUser: "使用者", KeyPassword: "密碼", KeyDatabase: "資料庫", KeySSLMode: "SSL 模式", KeyMySQLTLS: "MySQL TLS 模式", KeySQLStatement: "SQL 查詢或陳述式", KeyListTables: "列出資料表", KeyRunQuery: "執行查詢", KeyRunExec: "執行指令", KeyDatabaseOutput: "資料庫輸出", KeyR2S3Vault: "R2 / S3 Vault", KeySQLVault: "SQL Vault", KeyStatusCreate: "正在建立加密 Vault...", KeyStatusSignIn: "登入以解鎖加密 Vault。", KeyStatusUnlock: "正在解鎖加密 Vault...", KeyStatusRecovery: "輸入一次性復原金鑰與新憑證。", KeyStatusReady: "就緒。", KeyStatusWorking: "處理中...", KeyStatusFailed: "操作失敗。", KeyStatusSSHClosed: "SSH 連線已關閉。", KeyStatusConnected: "已連線至 ", KeyStatusTest: "正在測試 Vault 連線...", KeyStatusUploading: "正在上傳物件...", KeyStatusDownloading: "正在下載物件...", KeyStatusDeleting: "正在刪除物件...", KeyStatusListing: "正在列出遠端物件...", KeyStatusDBTables: "正在載入資料庫資料表...", KeyStatusDBQuery: "正在執行資料庫查詢...", KeyStatusDBExec: "正在執行資料庫陳述式...", KeyStatusSaving: "正在保存加密 Profile...", KeyNoObjects: "找不到物件。", KeyNoRows: "沒有回傳資料列。", KeyLanguageToggle: "EN", KeyAppSubtitle: "安全遠端工作區", KeySecureWorkspace: "安全遠端工作區", KeyDefaultSSL: "PostgreSQL SSL 模式（預設 require）",
	},
}

var extraTranslations = map[Language]map[Key]string{
	English: {
		KeySSHRequired: "SSH name, host, and user are required", KeySSHAuthRequired: "SSH password or key path is required", KeyStorageRequired: "storage name, endpoint, access key, secret key, and bucket are required", KeyDatabaseRequired: "database name, type, host, user, password, and database are required", KeyPortInvalid: "port must be between 1 and 65535", KeyOperationFailed: "Operation failed.", KeyDownloaded: "Downloaded ", KeyRowsAffected: "Rows affected: ", KeyPreview: " (preview available in output)", KeyCreateHint: "Create a vault to get started.", KeyBytes: "Bytes", KeyLastInsertID: "Last insert ID: ", KeyRecoverySaved: "Save this recovery key before continuing.", KeyStatusRotating: "Rotating recovery credentials...", KeyStatusConnecting: "Connecting to SSH host...", KeySSHProfiles: "SSH profiles", KeyStorageProfiles: "Storage profiles", KeyDatabaseProfiles: "Database profiles", KeyNew: "New", KeyName: "Name", KeyKeyPath: "Key path", KeyKeyPassphrase: "Key passphrase", KeyHostFingerprint: "Host fingerprint", KeyVaultBucket: "Vault bucket", KeyDownloadedTo: " to ", KeyPreferenceSave: "Could not save language preference: ", KeyVaultRequired: "vault name and password are required", KeyS3Bootstrap: "S3 endpoint, bucket, access key, and secret key are required", KeySQLBootstrap: "SQL type, host, port, user, password, and database are required", KeyRecoveryRequired: "recovery key, new vault name, and new vault password are required",
	},
	TraditionalChinese: {
		KeySSHRequired: "SSH 名稱、主機與使用者為必填", KeySSHAuthRequired: "SSH 密碼或 Key 路徑為必填", KeyStorageRequired: "儲存名稱、端點、Access Key、Secret Key 與 Bucket 為必填", KeyDatabaseRequired: "資料庫名稱、類型、主機、使用者、密碼與資料庫為必填", KeyPortInvalid: "連接埠必須介於 1 到 65535", KeyOperationFailed: "操作失敗。", KeyDownloaded: "已下載 ", KeyRowsAffected: "受影響資料列：", KeyPreview: "（輸出區可查看預覽）", KeyCreateHint: "建立 Vault 以開始使用。", KeyBytes: "位元組", KeyLastInsertID: "最後寫入 ID：", KeyRecoverySaved: "繼續前請保存復原金鑰。", KeyStatusRotating: "正在輪換復原憑證...", KeyStatusConnecting: "正在連線至 SSH 主機...", KeySSHProfiles: "SSH Profiles", KeyStorageProfiles: "儲存 Profiles", KeyDatabaseProfiles: "資料庫 Profiles", KeyNew: "新增", KeyName: "名稱", KeyKeyPath: "Key 路徑", KeyKeyPassphrase: "Key 密語", KeyHostFingerprint: "主機指紋", KeyVaultBucket: "Vault Bucket", KeyDownloadedTo: " 至 ", KeyPreferenceSave: "無法保存語言偏好：", KeyVaultRequired: "Vault 名稱與密碼為必填", KeyS3Bootstrap: "S3 端點、Bucket、Access Key 與 Secret Key 為必填", KeySQLBootstrap: "SQL 類型、主機、連接埠、使用者、密碼與資料庫為必填", KeyRecoveryRequired: "復原金鑰、新 Vault 名稱與新 Vault 密碼為必填",
	},
}

func Keys() []Key {
	keys := make([]Key, 0, len(translations[English])+len(extraTranslations[English]))
	for key := range translations[English] {
		keys = append(keys, key)
	}
	for key := range extraTranslations[English] {
		keys = append(keys, key)
	}
	return keys
}

func T(language Language, key Key) string {
	if _, ok := translations[language]; !ok {
		language = English
	}
	if value := translations[language][key]; value != "" {
		return value
	}
	if value := extraTranslations[language][key]; value != "" {
		return value
	}
	if value := extraTranslations[English][key]; value != "" {
		return value
	}
	return translations[English][key]
}

// Text translates a known English UI string. Unknown strings are returned unchanged;
// this preserves raw messages from remote services.
func Text(language Language, source string) string {
	if prefix := extraTranslations[English][KeyPreferenceSave]; prefix != "" && strings.HasPrefix(source, prefix) {
		return T(language, KeyPreferenceSave) + strings.TrimPrefix(source, prefix)
	}
	for key, value := range translations[English] {
		if value == source {
			return T(language, key)
		}
	}
	for key, value := range extraTranslations[English] {
		if value == source {
			return T(language, key)
		}
	}
	return source
}

func LoadPreferences(path string) (Preferences, error) {
	prefs := Preferences{Version: preferencesVersion, Language: English}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return prefs, nil
	}
	if err != nil {
		return prefs, err
	}
	var loaded Preferences
	if err := json.Unmarshal(data, &loaded); err != nil {
		return prefs, nil
	}
	if loaded.Version != preferencesVersion || !validLanguage(loaded.Language) {
		return prefs, nil
	}
	return loaded, nil
}

func SavePreferences(path string, prefs Preferences) error {
	if !validLanguage(prefs.Language) {
		prefs.Language = English
	}
	prefs.Version = preferencesVersion
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func validLanguage(language Language) bool {
	return language == English || language == TraditionalChinese
}
