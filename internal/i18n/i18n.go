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
	KeyRemoteAccount       Key = "remote_account"
	KeyRemoteRequired      Key = "remote_credentials_required"
	KeyStatusRemoteLogin   Key = "status_remote_login"
	KeyStatusRemoteLoad    Key = "status_remote_load"
	KeyStatusSigningOut    Key = "status_signing_out"
	KeyRemoteReady         Key = "remote_ready"
	KeyRemoteHint          Key = "remote_hint"
	KeyRemoteUnavailable   Key = "remote_unavailable"
	KeySSHDisabled         Key = "ssh_disabled"
	KeyCancel              Key = "cancel"
	KeyConfirm             Key = "confirm"
	KeyShow                Key = "show"
	KeyHide                Key = "hide"
	KeyPreferenceSave      Key = "preference_save_failed"
	KeySSHWorkspaceTitle   Key = "ssh_workspace_title"
	KeySSHWorkspaceEmpty   Key = "ssh_workspace_empty"
	KeySSHClose            Key = "ssh_close"
	KeySSHConnecting       Key = "ssh_connecting"
	KeySSHConnected        Key = "ssh_connected"
	KeySSHConnectionFailed Key = "ssh_connection_failed"
	KeySSHRetryHint        Key = "ssh_retry_hint"
	KeySSHConnectingHost   Key = "ssh_connecting_host"
	KeyEdit                Key = "edit"
	KeyNewSSHHost          Key = "new_ssh_host"
	KeyEditSSHHost         Key = "edit_ssh_host"
	KeyDiscardChanges      Key = "discard_changes"
	KeySSHFormUnsaved      Key = "ssh_form_unsaved"
)

var translations = map[Language]map[Key]string{
	English: {
		KeyAppSubtitle: "Secure remote workspace", KeyLanguageToggle: "中", KeyLogOut: "Log out", KeyStatusWorking: "Working...", KeyStatusFailed: "Operation failed.", KeyConnect: "Connect", KeySSHHosts: "SSH hosts", KeyNoSSHHosts: "No SSH hosts yet.", KeyNewHost: "New host", KeySSHHostDetails: "SSH host details", KeySaveHost: "Save host", KeyDeleteHost: "Delete host", KeyDeleteSSHHostTitle: "Delete SSH host?", KeyTrustHostKey: "Trust this host key?", KeyCloseTerminal: "Close terminal", KeyTerminalInput: "Terminal input", KeySendTerminal: "Send", KeyTerminalPlaceholder: "Terminal output will appear here", KeyNoTerminal: "SSH terminal is not connected", KeySSHAlreadyConnected: "SSH terminal is already connected", KeySSHSelectHost: "Select or save an SSH host first", KeyStatusSSHHosts: "Loading SSH hosts...", KeyStatusSSHSaving: "Saving SSH host...", KeyStatusSSHDeleting: "Deleting SSH host...", KeyStatusSSHTrusting: "Trusting host key...", KeyStatusSSHConnecting: "Connecting to SSH host...", KeyStatusSSHConnected: "SSH terminal connected.", KeyStatusSSHClosed: "SSH terminal closed.", KeyName: "Name", KeyHost: "Host", KeyPort: "Port", KeyUsername: "Username", KeyPassword: "Password", KeyPrivateKey: "Private key", KeyKeyPassphrase: "Key passphrase", KeyHostFingerprint: "Host fingerprint", KeyNameRequired: "name is required", KeyHostRequired: "host is required", KeyUsernameRequired: "username is required", KeySSHAuthRequired: "password or private key is required", KeyPortInvalid: "port must be between 1 and 65535", KeyRemoteTitle: "Sign in with authentication service", KeyRemoteDescription: "Use a complete HTTP or HTTPS URL. The password is never saved.", KeyRemoteURL: "Authentication service URL", KeyAccount: "Account", KeyRemoteSignIn: "Sign in remotely", KeyRemoteRestore: "Restore saved session", KeyRemoteWorkspace: "Remote workspace", KeyRemoteAccount: "Remote account: ", KeyRemoteRequired: "Remote sign-in URL, account, and password are required", KeyStatusRemoteLogin: "Signing in to authentication service...", KeyStatusRemoteLoad: "Restoring remote session...", KeyStatusSigningOut: "Signing out...", KeyRemoteReady: "Remote workspace ready.", KeyRemoteHint: "Sign in to the remote authentication service.", KeyRemoteUnavailable: "Remote authentication service is unavailable", KeySSHDisabled: "SSH access is not enabled for this account.", KeyCancel: "Cancel", KeyConfirm: "Confirm", KeyShow: "Show", KeyHide: "Hide", KeyPreferenceSave: "Could not save language preference: ",
	},
	TraditionalChinese: {
		KeyAppSubtitle: "安全遠端工作區", KeyLanguageToggle: "EN", KeyLogOut: "登出", KeyStatusWorking: "處理中...", KeyStatusFailed: "操作失敗。", KeyConnect: "連線", KeySSHHosts: "SSH 主機", KeyNoSSHHosts: "尚無 SSH 主機。", KeyNewHost: "新增主機", KeySSHHostDetails: "SSH 主機詳細資料", KeySaveHost: "儲存主機", KeyDeleteHost: "刪除主機", KeyDeleteSSHHostTitle: "刪除 SSH 主機？", KeyTrustHostKey: "信任此主機金鑰？", KeyCloseTerminal: "關閉終端機", KeyTerminalInput: "終端機輸入", KeySendTerminal: "傳送", KeyTerminalPlaceholder: "終端機輸出將顯示於此", KeyNoTerminal: "SSH 終端機尚未連線", KeySSHAlreadyConnected: "SSH 終端機已連線", KeySSHSelectHost: "請先選擇或儲存 SSH 主機", KeyStatusSSHHosts: "正在載入 SSH 主機...", KeyStatusSSHSaving: "正在儲存 SSH 主機...", KeyStatusSSHDeleting: "正在刪除 SSH 主機...", KeyStatusSSHTrusting: "正在信任主機金鑰...", KeyStatusSSHConnecting: "正在連線至 SSH 主機...", KeyStatusSSHConnected: "SSH 終端機已連線。", KeyStatusSSHClosed: "SSH 終端機已關閉。", KeyName: "名稱", KeyHost: "主機", KeyPort: "連接埠", KeyUsername: "使用者名稱", KeyPassword: "密碼", KeyPrivateKey: "私鑰", KeyKeyPassphrase: "金鑰密語", KeyHostFingerprint: "主機指紋", KeyNameRequired: "名稱為必填", KeyHostRequired: "主機為必填", KeyUsernameRequired: "使用者名稱為必填", KeySSHAuthRequired: "密碼或私鑰為必填", KeyPortInvalid: "連接埠必須介於 1 到 65535", KeyRemoteTitle: "以驗證服務登入", KeyRemoteDescription: "請輸入完整的 HTTP 或 HTTPS URL。密碼不會被儲存。", KeyRemoteURL: "驗證服務 URL", KeyAccount: "帳號", KeyRemoteSignIn: "遠端登入", KeyRemoteRestore: "還原已儲存的 Session", KeyRemoteWorkspace: "遠端工作區", KeyRemoteAccount: "遠端帳號：", KeyRemoteRequired: "遠端登入 URL、帳號與密碼為必填", KeyStatusRemoteLogin: "正在登入驗證服務...", KeyStatusRemoteLoad: "正在還原遠端 Session...", KeyStatusSigningOut: "正在登出...", KeyRemoteReady: "遠端工作區已就緒。", KeyRemoteHint: "登入遠端驗證服務。", KeyRemoteUnavailable: "遠端驗證服務無法使用", KeySSHDisabled: "此帳號未啟用 SSH 存取。", KeyCancel: "取消", KeyConfirm: "確認", KeyShow: "顯示", KeyHide: "隱藏", KeyPreferenceSave: "無法儲存語言偏好：",
	},
}

func init() {
	translations[English][KeySSHWorkspaceTitle] = "SSH terminal workspace"
	translations[English][KeySSHWorkspaceEmpty] = "Select an SSH host to open a terminal tab."
	translations[English][KeySSHClose] = "Close"
	translations[English][KeySSHConnecting] = "Connecting"
	translations[English][KeySSHConnected] = "Connected"
	translations[English][KeySSHConnectionFailed] = "Connection failed"
	translations[English][KeySSHRetryHint] = "Use Retry to try this host again, or Close to remove this tab."
	translations[English][KeySSHConnectingHost] = "Connecting to SSH host..."
	translations[English][KeyEdit] = "Edit"
	translations[English][KeyNewSSHHost] = "New SSH host"
	translations[English][KeyEditSSHHost] = "Edit SSH host"
	translations[English][KeyDiscardChanges] = "Discard changes?"
	translations[English][KeySSHFormUnsaved] = "This SSH host form has unsaved changes."

	translations[TraditionalChinese][KeySSHWorkspaceTitle] = "SSH 終端機工作區"
	translations[TraditionalChinese][KeySSHWorkspaceEmpty] = "請選擇 SSH 主機以開啟終端機分頁。"
	translations[TraditionalChinese][KeySSHClose] = "關閉"
	translations[TraditionalChinese][KeySSHConnecting] = "連線中"
	translations[TraditionalChinese][KeySSHConnected] = "已連線"
	translations[TraditionalChinese][KeySSHConnectionFailed] = "連線失敗"
	translations[TraditionalChinese][KeySSHRetryHint] = "按下「重試」再次連線，或按下「關閉」移除此分頁。"
	translations[TraditionalChinese][KeySSHConnectingHost] = "正在連線至 SSH 主機..."
	translations[TraditionalChinese][KeyEdit] = "編輯"
	translations[TraditionalChinese][KeyNewSSHHost] = "新增 SSH 主機"
	translations[TraditionalChinese][KeyEditSSHHost] = "編輯 SSH 主機"
	translations[TraditionalChinese][KeyDiscardChanges] = "捨棄變更？"
	translations[TraditionalChinese][KeySSHFormUnsaved] = "SSH 主機表單有尚未儲存的變更。"
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
