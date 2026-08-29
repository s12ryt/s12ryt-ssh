package remote

import (
	"fmt"
	"io"
)

// Operation is one server-authorized action on an assigned connection.
type Operation string

const (
	OperationS3Read    Operation = "s3.read"
	OperationS3Write   Operation = "s3.write"
	OperationS3Delete  Operation = "s3.delete"
	OperationSQLTables Operation = "sql.tables"
	OperationSQLQuery  Operation = "sql.query"
	OperationSQLExec   Operation = "sql.exec"
)

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

// Resource describes one assigned S3 or SQL connection without credentials.
type Resource struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Enabled    bool        `json:"enabled"`
	Operations []Operation `json:"operations"`
}

// Allows reports whether the server assigned operation to this resource.
func (r Resource) Allows(operation Operation) bool {
	for _, assigned := range r.Operations {
		if assigned == operation {
			return true
		}
	}
	return false
}

// ResourcesOverview is the /resources response: assigned connections plus the account SSH switch.
type ResourcesOverview struct {
	Resources  []Resource `json:"resources"`
	SSHEnabled bool       `json:"sshEnabled"`
}

// SSHHost describes one self-managed SSH host without credentials.
type SSHHost struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	HasPassword        bool   `json:"hasPassword"`
	HasPrivateKey      bool   `json:"hasPrivateKey"`
	HasKeyPassphrase   bool   `json:"hasKeyPassphrase"`
	TrustedFingerprint string `json:"trustedFingerprint"`
	CreatedAt          int64  `json:"createdAt"`
	UpdatedAt          int64  `json:"updatedAt"`
}

// SSHHostInput is the create/update body. Empty credential fields keep the stored secret.
type SSHHostInput struct {
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password,omitempty"`
	PrivateKey         string `json:"privateKey,omitempty"`
	KeyPassphrase      string `json:"keyPassphrase,omitempty"`
	TrustedFingerprint string `json:"trustedFingerprint,omitempty"`
}

// SSHHostCredentials is the one-time credential issuance for a saved host.
type SSHHostCredentials struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	PrivateKey         string `json:"privateKey"`
	KeyPassphrase      string `json:"keyPassphrase"`
	TrustedFingerprint string `json:"trustedFingerprint"`
}

// S3Object is non-sensitive object metadata returned by the proxy.
type S3Object struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified,omitempty"`
}

// UploadResult reports how many bytes the proxy accepted.
type UploadResult struct {
	Bytes int64 `json:"bytes"`
}

// Download is a streaming object response. Callers must close Body.
type Download struct {
	Body          io.ReadCloser
	ContentLength int64
	ContentType   string
}

// SQLQueryResult contains the bounded rows returned by a read-only query.
type SQLQueryResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
}

// SQLExecResult contains metadata from a statement with side effects.
type SQLExecResult struct {
	RowsAffected int64  `json:"rowsAffected"`
	LastInsertID string `json:"lastInsertId,omitempty"`
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
