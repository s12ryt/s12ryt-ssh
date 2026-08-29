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
	KeyAppSubtitle         Key = "app_subtitle"
	KeyLanguageToggle      Key = "language_toggle"
	KeyLogOut              Key = "log_out"
	KeyStatusWorking       Key = "status_working"
	KeyStatusFailed        Key = "status_failed"
	KeyTabSSH              Key = "tab_ssh"
	KeyStorage             Key = "storage"
	KeySQLDatabase         Key = "sql_database"
	KeyConnect             Key = "connect"
	KeySSHHosts            Key = "ssh_hosts"
	KeyNoSSHHosts          Key = "no_ssh_hosts"
	KeyNewHost             Key = "new_host"
	KeySSHHostDetails      Key = "ssh_host_details"
	KeySaveHost            Key = "save_host"
	KeyDeleteHost          Key = "delete_host"
	KeyDeleteSSHHostTitle  Key = "delete_ssh_host_title"
	KeyTrustHostKey        Key = "trust_host_key"
	KeyCloseTerminal       Key = "close_terminal"
	KeyTerminalInput       Key = "terminal_input"
	KeySendTerminal        Key = "send_terminal"
	KeyTerminalPlaceholder Key = "terminal_placeholder"
	KeyNoTerminal          Key = "terminal_not_connected"
	KeySSHAlreadyConnected Key = "ssh_already_connected"
	KeySSHSelectHost       Key = "ssh_select_host"
	KeyStatusSSHHosts      Key = "status_ssh_hosts"
	KeyStatusSSHSaving     Key = "status_ssh_saving"
	KeyStatusSSHDeleting   Key = "status_ssh_deleting"
	KeyStatusSSHTrusting   Key = "status_ssh_trusting"
	KeyStatusSSHConnecting Key = "status_ssh_connecting"
	KeyStatusSSHConnected  Key = "status_ssh_connected"
	KeyStatusSSHClosed     Key = "status_ssh_closed"
	KeyName                Key = "name"
	KeyHost                Key = "host"
	KeyPort                Key = "port"
	KeyUsername            Key = "username"
	KeyPassword            Key = "password"
	KeyPrivateKey          Key = "private_key"
	KeyKeyPassphrase       Key = "key_passphrase"
	KeyHostFingerprint     Key = "host_fingerprint"
	KeyNameRequired        Key = "name_required"
	KeyHostRequired        Key = "host_required"
	KeyUsernameRequired    Key = "username_required"
	KeySSHAuthRequired     Key = "ssh_auth_required"
	KeyPortInvalid         Key = "port_invalid"
	KeyRemoteTitle         Key = "remote_title"
	KeyRemoteDescription   Key = "remote_description"
	KeyRemoteURL           Key = "remote_url"
	KeyAccount             Key = "account"
	KeyRemoteSignIn        Key = "remote_sign_in"
	KeyRemoteRestore       Key = "remote_restore"
	KeyRemoteWorkspace     Key = "remote_workspace"
	KeyAssigned            Key = "assigned_connections"
	KeyNoAssigned          Key = "no_assigned_connections"
	KeyAssignedStorage     Key = "assigned_storage"
	KeyAssignedDatabase    Key = "assigned_database"
	KeyRemoteAccount       Key = "remote_account"
	KeyRemoteRequired      Key = "remote_credentials_required"
	KeyStatusRemoteLogin   Key = "status_remote_login"
	KeyStatusRemoteLoad    Key = "status_remote_load"
	KeyStatusSigningOut    Key = "status_signing_out"
	KeyRemoteReady         Key = "remote_ready"
	KeyRemoteHint          Key = "remote_hint"
	KeyRemoteForbidden     Key = "remote_forbidden"
	KeyRemoteUnselected    Key = "remote_unselected"
	KeyRemoteObjectOut     Key = "remote_object_output"
	KeyRemoteDatabaseOut   Key = "remote_database_output"
	KeyRemoteUnavailable   Key = "remote_unavailable"
	KeyStatusRemoteList    Key = "status_remote_resource_list"
	KeyListPrefix          Key = "list_prefix"
	KeyObjectKey           Key = "object_key"
	KeyLocalPath           Key = "local_path"
	KeyInlineData          Key = "inline_data"
	KeyRefreshList         Key = "refresh_list"
	KeyUpload              Key = "upload"
	KeyDownload            Key = "download"
	KeyDelete              Key = "delete"
	KeyUploaded            Key = "uploaded"
	KeyDownloaded          Key = "downloaded"
	KeyDownloadedTo        Key = "downloaded_to"
	KeyNoObjects           Key = "no_objects"
	KeyObjectsCount        Key = "objects_count"
	KeyBytes               Key = "bytes"
	KeyPreview             Key = "preview"
	KeyStatusUploading     Key = "status_uploading"
	KeyStatusDownloading   Key = "status_downloading"
	KeyStatusDeleting      Key = "status_deleting"
	KeyStatusListing       Key = "status_listing"
	KeyDeleteObjectTitle   Key = "delete_object_title"
	KeyDeleteObjectMsg     Key = "delete_object_message"
	KeyObjectKeyRequired   Key = "object_key_required"
	KeySQLStatement        Key = "sql_statement"
	KeyListTables          Key = "list_tables"
	KeyRunQuery            Key = "run_query"
	KeyRunExec             Key = "run_exec"
	KeyNoRows              Key = "no_rows"
	KeyStatusDBTables      Key = "status_db_tables"
	KeyStatusDBQuery       Key = "status_db_query"
	KeyStatusDBExec        Key = "status_db_exec"
	KeyExecSQLTitle        Key = "exec_sql_title"
	KeyExecSQLMsg          Key = "exec_sql_message"
	KeyRowsAffected        Key = "rows_affected"
	KeyLastInsertID        Key = "last_insert_id"
	KeySQLRequired         Key = "sql_statement_required"
	KeyCancel              Key = "cancel"
	KeyConfirm             Key = "confirm"
	KeyShow                Key = "show"
	KeyHide                Key = "hide"
	KeyPreferenceSave      Key = "preference_save_failed"
)

var translations = map[Language]map[Key]string{
	English: {
		KeyAppSubtitle: "Secure remote workspace", KeyLanguageToggle: "中", KeyLogOut: "Log out", KeyStatusWorking: "Working...", KeyStatusFailed: "Operation failed.", KeyTabSSH: "SSH", KeyStorage: "S3 / R2", KeySQLDatabase: "SQL database", KeyConnect: "Connect", KeySSHHosts: "SSH hosts", KeyNoSSHHosts: "No SSH hosts yet.", KeyNewHost: "New host", KeySSHHostDetails: "SSH host details", KeySaveHost: "Save host", KeyDeleteHost: "Delete host", KeyDeleteSSHHostTitle: "Delete SSH host?", KeyTrustHostKey: "Trust this host key?", KeyCloseTerminal: "Close terminal", KeyTerminalInput: "Terminal input", KeySendTerminal: "Send", KeyTerminalPlaceholder: "Terminal output will appear here", KeyNoTerminal: "SSH terminal is not connected", KeySSHAlreadyConnected: "SSH terminal is already connected", KeySSHSelectHost: "Select or save an SSH host first", KeyStatusSSHHosts: "Loading SSH hosts...", KeyStatusSSHSaving: "Saving SSH host...", KeyStatusSSHDeleting: "Deleting SSH host...", KeyStatusSSHTrusting: "Trusting host key...", KeyStatusSSHConnecting: "Connecting to SSH host...", KeyStatusSSHConnected: "SSH terminal connected.", KeyStatusSSHClosed: "SSH terminal closed.", KeyName: "Name", KeyHost: "Host", KeyPort: "Port", KeyUsername: "Username", KeyPassword: "Password", KeyPrivateKey: "Private key", KeyKeyPassphrase: "Key passphrase", KeyHostFingerprint: "Host fingerprint", KeyNameRequired: "name is required", KeyHostRequired: "host is required", KeyUsernameRequired: "username is required", KeySSHAuthRequired: "password or private key is required", KeyPortInvalid: "port must be between 1 and 65535", KeyRemoteTitle: "Sign in with authentication service", KeyRemoteDescription: "Use a complete HTTP or HTTPS URL. The password is never saved.", KeyRemoteURL: "Authentication service URL", KeyAccount: "Account", KeyRemoteSignIn: "Sign in remotely", KeyRemoteRestore: "Restore saved session", KeyRemoteWorkspace: "Remote workspace", KeyAssigned: "Assigned connections", KeyNoAssigned: "No assigned connections.", KeyAssignedStorage: "Assigned S3 / R2", KeyAssignedDatabase: "Assigned SQL database", KeyRemoteAccount: "Remote account: ", KeyRemoteRequired: "Remote sign-in URL, account, and password are required", KeyStatusRemoteLogin: "Signing in to authentication service...", KeyStatusRemoteLoad: "Restoring remote session...", KeyStatusSigningOut: "Signing out...", KeyRemoteReady: "Remote workspace ready.", KeyRemoteHint: "Sign in to the remote authentication service.", KeyRemoteForbidden: "Permission not granted for this operation", KeyRemoteUnselected: "No connection selected", KeyRemoteObjectOut: "Remote objects and operation output", KeyRemoteDatabaseOut: "Remote database output", KeyRemoteUnavailable: "Remote authentication service is unavailable", KeyStatusRemoteList: "Loading assigned connections...", KeyListPrefix: "List prefix", KeyObjectKey: "Object key", KeyLocalPath: "Local path", KeyInlineData: "Inline upload data", KeyRefreshList: "Refresh list", KeyUpload: "Upload", KeyDownload: "Download", KeyDelete: "Delete", KeyUploaded: "Uploaded ", KeyDownloaded: "Downloaded ", KeyDownloadedTo: " to ", KeyNoObjects: "No objects found.", KeyObjectsCount: "%d objects", KeyBytes: "Bytes", KeyPreview: " (preview available in output)", KeyStatusUploading: "Uploading object...", KeyStatusDownloading: "Downloading object...", KeyStatusDeleting: "Deleting object...", KeyStatusListing: "Listing remote objects...", KeyDeleteObjectTitle: "Delete object", KeyDeleteObjectMsg: "This permanently deletes the object. This action cannot be undone.", KeyObjectKeyRequired: "object key is required", KeySQLStatement: "SQL query or statement", KeyListTables: "List tables", KeyRunQuery: "Run query", KeyRunExec: "Run exec", KeyNoRows: "No rows returned.", KeyStatusDBTables: "Loading database tables...", KeyStatusDBQuery: "Running database query...", KeyStatusDBExec: "Executing database statement...", KeyExecSQLTitle: "Execute SQL statement", KeyExecSQLMsg: "This runs a statement that can modify data. Continue?", KeyRowsAffected: "Rows affected: ", KeyLastInsertID: "Last insert ID: ", KeySQLRequired: "SQL statement is required", KeyCancel: "Cancel", KeyConfirm: "Confirm", KeyShow: "Show", KeyHide: "Hide", KeyPreferenceSave: "Could not save language preference: ",
	},
	TraditionalChinese: {
		KeyAppSubtitle: "安全遠端工作區", KeyLanguageToggle: "EN", KeyLogOut: "登出", KeyStatusWorking: "處理中...", KeyStatusFailed: "操作失敗。", KeyTabSSH: "SSH", KeyStorage: "S3 / R2", KeySQLDatabase: "SQL 資料庫", KeyConnect: "連線", KeySSHHosts: "SSH 主機", KeyNoSSHHosts: "尚無 SSH 主機。", KeyNewHost: "新增主機", KeySSHHostDetails: "SSH 主機詳細資料", KeySaveHost: "儲存主機", KeyDeleteHost: "刪除主機", KeyDeleteSSHHostTitle: "刪除 SSH 主機？", KeyTrustHostKey: "信任此主機金鑰？", KeyCloseTerminal: "關閉終端機", KeyTerminalInput: "終端機輸入", KeySendTerminal: "傳送", KeyTerminalPlaceholder: "終端機輸出將顯示於此", KeyNoTerminal: "SSH 終端機尚未連線", KeySSHAlreadyConnected: "SSH 終端機已連線", KeySSHSelectHost: "請先選擇或儲存 SSH 主機", KeyStatusSSHHosts: "正在載入 SSH 主機...", KeyStatusSSHSaving: "正在儲存 SSH 主機...", KeyStatusSSHDeleting: "正在刪除 SSH 主機...", KeyStatusSSHTrusting: "正在信任主機金鑰...", KeyStatusSSHConnecting: "正在連線至 SSH 主機...", KeyStatusSSHConnected: "SSH 終端機已連線。", KeyStatusSSHClosed: "SSH 終端機已關閉。", KeyName: "名稱", KeyHost: "主機", KeyPort: "連接埠", KeyUsername: "使用者名稱", KeyPassword: "密碼", KeyPrivateKey: "私鑰", KeyKeyPassphrase: "金鑰密語", KeyHostFingerprint: "主機指紋", KeyNameRequired: "名稱為必填", KeyHostRequired: "主機為必填", KeyUsernameRequired: "使用者名稱為必填", KeySSHAuthRequired: "密碼或私鑰為必填", KeyPortInvalid: "連接埠必須介於 1 到 65535", KeyRemoteTitle: "以驗證服務登入", KeyRemoteDescription: "請輸入完整的 HTTP 或 HTTPS URL。密碼不會被儲存。", KeyRemoteURL: "驗證服務 URL", KeyAccount: "帳號", KeyRemoteSignIn: "遠端登入", KeyRemoteRestore: "還原已儲存的 Session", KeyRemoteWorkspace: "遠端工作區", KeyAssigned: "已指派的連線", KeyNoAssigned: "沒有已指派的連線。", KeyAssignedStorage: "已指派 S3 / R2", KeyAssignedDatabase: "已指派 SQL 資料庫", KeyRemoteAccount: "遠端帳號：", KeyRemoteRequired: "遠端登入 URL、帳號與密碼為必填", KeyStatusRemoteLogin: "正在登入驗證服務...", KeyStatusRemoteLoad: "正在還原遠端 Session...", KeyStatusSigningOut: "正在登出...", KeyRemoteReady: "遠端工作區已就緒。", KeyRemoteHint: "登入遠端驗證服務。", KeyRemoteForbidden: "此操作未獲授權", KeyRemoteUnselected: "尚未選擇連線", KeyRemoteObjectOut: "遠端物件與操作輸出", KeyRemoteDatabaseOut: "遠端資料庫輸出", KeyRemoteUnavailable: "遠端驗證服務無法使用", KeyStatusRemoteList: "正在載入已指派連線...", KeyListPrefix: "列出前綴", KeyObjectKey: "物件 Key", KeyLocalPath: "本機路徑", KeyInlineData: "內嵌上傳資料", KeyRefreshList: "重新整理列表", KeyUpload: "上傳", KeyDownload: "下載", KeyDelete: "刪除", KeyUploaded: "已上傳 ", KeyDownloaded: "已下載 ", KeyDownloadedTo: " 到 ", KeyNoObjects: "找不到物件。", KeyObjectsCount: "%d 個物件", KeyBytes: "位元組", KeyPreview: "（可在輸出預覽）", KeyStatusUploading: "正在上傳物件...", KeyStatusDownloading: "正在下載物件...", KeyStatusDeleting: "正在刪除物件...", KeyStatusListing: "正在列出遠端物件...", KeyDeleteObjectTitle: "刪除物件", KeyDeleteObjectMsg: "這將永久刪除該物件。此操作無法復原。", KeyObjectKeyRequired: "物件 Key 為必填", KeySQLStatement: "SQL 查詢或語句", KeyListTables: "列出資料表", KeyRunQuery: "執行查詢", KeyRunExec: "執行語句", KeyNoRows: "沒有回傳資料列。", KeyStatusDBTables: "正在載入資料表...", KeyStatusDBQuery: "正在執行資料庫查詢...", KeyStatusDBExec: "正在執行資料庫語句...", KeyExecSQLTitle: "執行 SQL 語句", KeyExecSQLMsg: "這將執行可能修改資料的語句，要繼續嗎？", KeyRowsAffected: "影響資料列：", KeyLastInsertID: "最後寫入 ID：", KeySQLRequired: "SQL 語句為必填", KeyCancel: "取消", KeyConfirm: "確認", KeyShow: "顯示", KeyHide: "隱藏", KeyPreferenceSave: "無法儲存語言偏好：",
	},
}

func Keys() []Key {
	keys := make([]Key, 0, len(translations[English]))
	for key := range translations[English] {
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
	return translations[English][key]
}

// Text translates a known English UI string. Unknown strings are returned unchanged;
// this preserves raw messages from remote services.
func Text(language Language, source string) string {
	if prefix := translations[English][KeyPreferenceSave]; prefix != "" && strings.HasPrefix(source, prefix) {
		return T(language, KeyPreferenceSave) + strings.TrimPrefix(source, prefix)
	}
	for key, value := range translations[English] {
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
