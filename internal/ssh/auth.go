package ssh

import (
	"errors"
	"os"

	gossh "golang.org/x/crypto/ssh"
)

var errNotConnected = errors.New("ssh: not connected")

// loadKeyAuth reads a private key file and returns an auth method.
// If passphrase is non-empty it is used to decrypt password-protected keys.
func loadKeyAuth(keyPath, passphrase string) (gossh.AuthMethod, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return keyDataAuth(data, passphrase)
}

// keyDataAuth parses an in-memory private key and returns an auth method.
// If passphrase is non-empty it is used to decrypt password-protected keys.
func keyDataAuth(data []byte, passphrase string) (gossh.AuthMethod, error) {
	var signer gossh.Signer
	var err error
	if passphrase != "" {
		signer, err = gossh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
	} else {
		signer, err = gossh.ParsePrivateKey(data)
	}
	if err != nil {
		return nil, err
	}
	return gossh.PublicKeys(signer), nil
}
