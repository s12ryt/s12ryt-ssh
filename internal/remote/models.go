package remote

import (
	"fmt"
)

// Operation is one server-authorized action on an assigned connection.
type Operation string

// Account is the non-sensitive authenticated account identity.
type Account struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// TokenPair contains opaque session tokens returned by the authentication service.
type TokenPair struct {
	AccessToken      string  `json:"accessToken"`
	AccessExpiresAt  int64   `json:"accessExpiresAt"`
	RefreshToken     string  `json:"refreshToken"`
	RefreshExpiresAt int64   `json:"refreshExpiresAt"`
	Account          Account `json:"account"`
	SessionID        string  `json:"sessionId"`
}

// Resource describes one assigned connection in the /resources response.
type Resource struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Enabled    bool        `json:"enabled"`
	Operations []Operation `json:"operations"`
}

// ResourcesOverview is the /resources response: assigned connections plus the account SSH switch.
type ResourcesOverview struct {
	Resources  []Resource `json:"resources"`
	SSHEnabled bool       `json:"sshEnabled"`
}

// SSHAuthMethod identifies which stored credential a host connection should use.
type SSHAuthMethod string

const (
	SSHAuthPassword   SSHAuthMethod = "password"
	SSHAuthPrivateKey SSHAuthMethod = "private_key"
)

// SSHTerminalFont identifies a synchronized terminal typeface choice.
type SSHTerminalFont string

const (
	SSHTerminalFontBuiltin SSHTerminalFont = "builtin-mono"
	SSHTerminalFontSystem  SSHTerminalFont = "system-mono"
)

// SSHTerminalAppearance is the complete account-level terminal appearance.
type SSHTerminalAppearance struct {
	Font       SSHTerminalFont `json:"font"`
	FontSize   float32         `json:"fontSize"`
	Foreground string          `json:"foreground"`
	Background string          `json:"background"`
}

// SSHTerminalAppearanceOverride contains only host-specific appearance values.
type SSHTerminalAppearanceOverride struct {
	Font       SSHTerminalFont `json:"font,omitempty"`
	FontSize   float32         `json:"fontSize,omitempty"`
	Foreground string          `json:"foreground,omitempty"`
	Background string          `json:"background,omitempty"`
}

// SSHWorkspacePreferences contains synchronized account-level SSH UI defaults.
type SSHWorkspacePreferences struct {
	TerminalAppearance SSHTerminalAppearance `json:"terminalAppearance"`
	Version            int                   `json:"version"`
	UpdatedAt          int64                 `json:"updatedAt"`
}

// SSHWorkspacePreferencesInput updates synchronized account defaults.
type SSHWorkspacePreferencesInput struct {
	TerminalAppearance SSHTerminalAppearance `json:"terminalAppearance"`
}

// SSHConnectionSettings contains the persisted connection and session defaults for a host.
type SSHConnectionSettings struct {
	TCPTimeoutMS          int                            `json:"tcpTimeoutMs"`
	SSHHandshakeTimeoutMS int                            `json:"sshHandshakeTimeoutMs"`
	PTYTimeoutMS          int                            `json:"ptyTimeoutMs"`
	KeepaliveIntervalMS   int                            `json:"keepaliveIntervalMs"`
	FailureCount          int                            `json:"failureCount"`
	IdleTimeoutMS         int                            `json:"idleTimeoutMs"`
	Compression           bool                           `json:"compression"`
	StartupCommand        string                         `json:"startupCommand"`
	InitialDirectory      string                         `json:"initialDirectory"`
	Environment           map[string]string              `json:"environment"`
	AutoReconnect         bool                           `json:"autoReconnect"`
	TerminalAppearance    *SSHTerminalAppearanceOverride `json:"terminalAppearance,omitempty"`
}

// SSHHost describes one self-managed SSH host without credentials.
type SSHHost struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	Host               string                `json:"host"`
	Port               int                   `json:"port"`
	Username           string                `json:"username"`
	HasPassword        bool                  `json:"hasPassword"`
	HasPrivateKey      bool                  `json:"hasPrivateKey"`
	HasKeyPassphrase   bool                  `json:"hasKeyPassphrase"`
	TrustedFingerprint string                `json:"trustedFingerprint"`
	Enabled            bool                  `json:"enabled"`
	Favorite           bool                  `json:"favorite"`
	GroupPath          string                `json:"groupPath"`
	Tags               []string              `json:"tags"`
	SortOrder          int                   `json:"sortOrder"`
	AuthMethod         SSHAuthMethod         `json:"authMethod"`
	Settings           SSHConnectionSettings `json:"settings"`
	Version            int                   `json:"version"`
	CreatedAt          int64                 `json:"createdAt"`
	UpdatedAt          int64                 `json:"updatedAt"`
}

// SSHHostInput is the create/update body. Empty credential fields keep the stored secret.
type SSHHostInput struct {
	Name                    string                 `json:"name"`
	Host                    string                 `json:"host"`
	Port                    int                    `json:"port"`
	Username                string                 `json:"username"`
	Password                string                 `json:"password,omitempty"`
	PrivateKey              string                 `json:"privateKey,omitempty"`
	KeyPassphrase           string                 `json:"keyPassphrase,omitempty"`
	TrustedFingerprint      string                 `json:"trustedFingerprint,omitempty"`
	Enabled                 bool                   `json:"enabled,omitempty"`
	Favorite                bool                   `json:"favorite,omitempty"`
	GroupPath               string                 `json:"groupPath,omitempty"`
	Tags                    []string               `json:"tags,omitempty"`
	SortOrder               int                    `json:"sortOrder,omitempty"`
	AuthMethod              SSHAuthMethod          `json:"authMethod,omitempty"`
	Settings                *SSHConnectionSettings `json:"settings,omitempty"`
	ClearTerminalAppearance bool                   `json:"clearTerminalAppearance,omitempty"`
}

// SSHHostCredentials is the one-time credential issuance for a saved host.
type SSHHostCredentials struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	Host               string                `json:"host"`
	Port               int                   `json:"port"`
	Username           string                `json:"username"`
	Password           string                `json:"password"`
	PrivateKey         string                `json:"privateKey"`
	KeyPassphrase      string                `json:"keyPassphrase"`
	TrustedFingerprint string                `json:"trustedFingerprint"`
	AuthMethod         SSHAuthMethod         `json:"authMethod"`
	Settings           SSHConnectionSettings `json:"settings"`
	Version            int                   `json:"version"`
}

// SSHHostFingerprintSource identifies how a host key became trusted.
type SSHHostFingerprintSource string

const (
	SSHHostFingerprintTOFU   SSHHostFingerprintSource = "tofu"
	SSHHostFingerprintManual SSHHostFingerprintSource = "manual"
)

// SSHHostFingerprint is an immutable observation in a host's trust history.
type SSHHostFingerprint struct {
	ID          string                   `json:"id"`
	HostID      string                   `json:"hostId"`
	Algorithm   string                   `json:"algorithm"`
	Fingerprint string                   `json:"fingerprint"`
	Source      SSHHostFingerprintSource `json:"source"`
	Active      bool                     `json:"active"`
	ObservedAt  int64                    `json:"observedAt"`
	RetiredAt   *int64                   `json:"retiredAt"`
}

// SSHCommandSnippet describes a saved command without exposing its secret values.
type SSHCommandSnippet struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Variables   []string `json:"variables"`
	SecretNames []string `json:"secretNames"`
	Enabled     bool     `json:"enabled"`
	Version     int      `json:"version"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}

// SSHCommandSnippetInput is the create/update payload for a saved command.
type SSHCommandSnippetInput struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Variables []string          `json:"variables,omitempty"`
	Secrets   map[string]string `json:"secrets,omitempty"`
	Enabled   *bool             `json:"enabled,omitempty"`
}

// SSHKeyIdentity describes a reusable private-key identity without secret material.
type SSHKeyIdentity struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	PublicKey     string `json:"publicKey"`
	Fingerprint   string `json:"fingerprint"`
	HasPassphrase bool   `json:"hasPassphrase"`
	Enabled       bool   `json:"enabled"`
	Version       int    `json:"version"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// SSHKeyIdentityInput is the create/update payload for a reusable key identity.
type SSHKeyIdentityInput struct {
	Name                string `json:"name"`
	PublicKey           string `json:"publicKey,omitempty"`
	Fingerprint         string `json:"fingerprint,omitempty"`
	PrivateKey          string `json:"privateKey,omitempty"`
	KeyPassphrase       string `json:"keyPassphrase,omitempty"`
	Enabled             *bool  `json:"enabled,omitempty"`
	ClearSecretMaterial bool   `json:"clearSecretMaterial,omitempty"`
}

// SSHKeyIdentitySecrets contains private material returned only by the secrets endpoint.
type SSHKeyIdentitySecrets struct {
	PrivateKey    string `json:"privateKey"`
	KeyPassphrase string `json:"keyPassphrase"`
}

// SSHSessionHistoryStatus identifies one observable stage in an SSH connection lifecycle.
type SSHSessionHistoryStatus string

const (
	SSHSessionConnecting SSHSessionHistoryStatus = "connecting"
	SSHSessionConnected  SSHSessionHistoryStatus = "connected"
	SSHSessionFailed     SSHSessionHistoryStatus = "failed"
	SSHSessionClosed     SSHSessionHistoryStatus = "closed"
)

// SSHSessionHistory contains connection metadata without terminal commands or output.
type SSHSessionHistory struct {
	ID           string                  `json:"id"`
	HostID       string                  `json:"hostId"`
	HostName     string                  `json:"hostName"`
	Status       SSHSessionHistoryStatus `json:"status"`
	LatencyMS    int                     `json:"latencyMs"`
	ErrorMessage string                  `json:"errorMessage"`
	StartedAt    int64                   `json:"startedAt"`
	EndedAt      *int64                  `json:"endedAt"`
}

// SSHSessionHistoryInput creates a connection lifecycle record.
type SSHSessionHistoryInput struct {
	HostID       string                  `json:"hostId"`
	Status       SSHSessionHistoryStatus `json:"status"`
	LatencyMS    *int                    `json:"latencyMs,omitempty"`
	ErrorMessage *string                 `json:"errorMessage,omitempty"`
}

// SSHSessionHistoryUpdate changes only the supplied lifecycle metadata.
type SSHSessionHistoryUpdate struct {
	Status       SSHSessionHistoryStatus `json:"status"`
	LatencyMS    *int                    `json:"latencyMs,omitempty"`
	ErrorMessage *string                 `json:"errorMessage,omitempty"`
}

// SSHWorkspaceExportRequest controls whether a server-side workspace export includes encrypted secrets.
type SSHWorkspaceExportRequest struct {
	IncludeSecrets bool   `json:"includeSecrets"`
	Password       string `json:"password,omitempty"`
}

// SSHWorkspaceImportKind identifies one category in a workspace package.
type SSHWorkspaceImportKind string

const (
	SSHWorkspaceImportHost    SSHWorkspaceImportKind = "host"
	SSHWorkspaceImportTunnel  SSHWorkspaceImportKind = "tunnel"
	SSHWorkspaceImportSnippet SSHWorkspaceImportKind = "snippet"
	SSHWorkspaceImportKey     SSHWorkspaceImportKind = "key"
)

// SSHWorkspaceImportDecision resolves a case-insensitive name conflict.
type SSHWorkspaceImportDecision string

const (
	SSHWorkspaceImportOverwrite SSHWorkspaceImportDecision = "overwrite"
	SSHWorkspaceImportSkip      SSHWorkspaceImportDecision = "skip"
	SSHWorkspaceImportCopy      SSHWorkspaceImportDecision = "copy"
)

// SSHWorkspaceImportResolution is one explicit conflict decision.
type SSHWorkspaceImportResolution struct {
	Kind   SSHWorkspaceImportKind     `json:"kind"`
	Name   string                     `json:"name"`
	Action SSHWorkspaceImportDecision `json:"action"`
}

// SSHWorkspaceImportRequest contains the opaque package and its conflict decisions.
type SSHWorkspaceImportRequest struct {
	Package     string                         `json:"package"`
	Password    string                         `json:"password,omitempty"`
	Resolutions []SSHWorkspaceImportResolution `json:"resolutions"`
}

// SSHWorkspaceResourceCounts summarizes package metadata without exposing payload contents.
type SSHWorkspaceResourceCounts struct {
	Hosts    int `json:"hosts"`
	Tunnels  int `json:"tunnels"`
	Snippets int `json:"snippets"`
	Keys     int `json:"keys"`
}

// SSHWorkspaceImportConflict is one previewed case-insensitive name collision.
type SSHWorkspaceImportConflict struct {
	Kind     SSHWorkspaceImportKind `json:"kind"`
	Name     string                 `json:"name"`
	Conflict bool                   `json:"conflict"`
}

// SSHWorkspaceImportPreview contains only counts and conflict metadata.
type SSHWorkspaceImportPreview struct {
	IncludesSecrets bool                         `json:"includesSecrets"`
	Counts          SSHWorkspaceResourceCounts   `json:"counts"`
	Conflicts       []SSHWorkspaceImportConflict `json:"conflicts"`
}

// SSHWorkspaceImportPlanItem describes one applied create, overwrite, copy, or skip action.
type SSHWorkspaceImportPlanItem struct {
	Kind       SSHWorkspaceImportKind     `json:"kind"`
	Name       string                     `json:"name"`
	Action     SSHWorkspaceImportDecision `json:"action"`
	TargetName string                     `json:"targetName"`
}

// SSHWorkspaceImportApplyCounts summarizes the completed atomic transaction.
type SSHWorkspaceImportApplyCounts struct {
	Created     int `json:"created"`
	Overwritten int `json:"overwritten"`
	Copied      int `json:"copied"`
	Skipped     int `json:"skipped"`
}

// SSHWorkspaceImportResult is the server's secret-free import result.
type SSHWorkspaceImportResult struct {
	IncludesSecrets bool                          `json:"includesSecrets"`
	Counts          SSHWorkspaceImportApplyCounts `json:"counts"`
	Items           []SSHWorkspaceImportPlanItem  `json:"items"`
}

// SSHTunnelType identifies the direction of an SSH port-forwarding rule.
type SSHTunnelType string

const (
	SSHTunnelLocal   SSHTunnelType = "local"
	SSHTunnelRemote  SSHTunnelType = "remote"
	SSHTunnelDynamic SSHTunnelType = "dynamic_socks"
)

// SSHTunnelRule is a saved forwarding rule and its current runtime counters.
type SSHTunnelRule struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	HostID           string        `json:"hostId"`
	Type             SSHTunnelType `json:"type"`
	ListenHost       string        `json:"listenHost"`
	ListenPort       int           `json:"listenPort"`
	TargetHost       string        `json:"targetHost"`
	TargetPort       int           `json:"targetPort"`
	Enabled          bool          `json:"enabled"`
	AutoStart        bool          `json:"autoStart"`
	Running          bool          `json:"running"`
	TrafficUpBytes   int64         `json:"trafficUpBytes"`
	TrafficDownBytes int64         `json:"trafficDownBytes"`
	Version          int           `json:"version"`
	CreatedAt        int64         `json:"createdAt"`
	UpdatedAt        int64         `json:"updatedAt"`
}

// SSHTunnelInput is the persisted rule payload.
type SSHTunnelInput struct {
	Name       string        `json:"name"`
	HostID     string        `json:"hostId"`
	Type       SSHTunnelType `json:"type"`
	ListenHost string        `json:"listenHost"`
	ListenPort int           `json:"listenPort"`
	TargetHost string        `json:"targetHost,omitempty"`
	TargetPort int           `json:"targetPort,omitempty"`
	Enabled    bool          `json:"enabled"`
	AutoStart  bool          `json:"autoStart"`
}

// SSHTunnelRuntimeUpdate reports local forwarding state without changing the saved configuration version.
type SSHTunnelRuntimeUpdate struct {
	Running          bool  `json:"running"`
	TrafficUpBytes   int64 `json:"trafficUpBytes"`
	TrafficDownBytes int64 `json:"trafficDownBytes"`
}

// APIError preserves the service's stable error code and diagnostic message.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return fmt.Sprintf("remote: HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("remote: %s: %s", e.Code, e.Message)
}
