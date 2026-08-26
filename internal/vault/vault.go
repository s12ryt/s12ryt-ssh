// Package vault implements the client-side encrypted profile vault.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"s12ryt-ssh/internal/config"

	"golang.org/x/crypto/argon2"
)

const (
	formatVersion = 1
	kdfMemory     = 32 * 1024
	kdfTime       = 1
	kdfThreads    = 1
	keySize       = 32
	saltSize      = 16
	identifierLen = 16
	recoveryLen   = 24
)

// ErrInvalidEnvelope means that a vault cannot be safely parsed or opened.
var ErrInvalidEnvelope = errors.New("vault: invalid encrypted envelope")

// Registration contains the public vault identity and the one-time recovery key.
type Registration struct {
	ID          string
	Name        string
	RecoveryKey string
}

type envelope struct {
	Version            int    `json:"version"`
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Salt               string `json:"salt"`
	Ciphertext         string `json:"ciphertext"`
	WrappedKey         string `json:"wrapped_key"`
	RecoverySalt       string `json:"recovery_salt"`
	RecoveryWrappedKey string `json:"recovery_wrapped_key"`
}

// Create generates a vault identity and encrypts all profiles with a random
// data-encryption key. The password and recovery key only wrap that key.
func Create(name, password string, profiles *config.Store) (Registration, []byte, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Registration{}, nil, fmt.Errorf("vault: name is required")
	}
	if password == "" {
		return Registration{}, nil, fmt.Errorf("vault: password is required")
	}
	if profiles == nil {
		profiles = &config.Store{}
	}

	id, err := newIdentifier()
	if err != nil {
		return Registration{}, nil, err
	}
	recoveryKey, err := randomText(recoveryLen)
	if err != nil {
		return Registration{}, nil, err
	}
	dek := make([]byte, keySize)
	if _, err := rand.Read(dek); err != nil {
		return Registration{}, nil, err
	}
	data, err := marshalPayload(profiles)
	if err != nil {
		return Registration{}, nil, err
	}
	env, err := sealEnvelope(id, name, password, recoveryKey, dek, data)
	if err != nil {
		return Registration{}, nil, err
	}
	encoded, err := json.Marshal(env)
	if err != nil {
		return Registration{}, nil, err
	}
	return Registration{ID: id, Name: name, RecoveryKey: recoveryKey}, encoded, nil
}

// Decrypt opens a vault using the registered name and password.
func Decrypt(data []byte, name, password string) (*config.Store, error) {
	env, err := parse(data)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" || password == "" {
		return nil, fmt.Errorf("%w: credentials are required", ErrInvalidEnvelope)
	}
	if env.Name != strings.TrimSpace(name) {
		return nil, fmt.Errorf("%w: credentials do not match", ErrInvalidEnvelope)
	}
	dek, err := unwrapKey(env.WrappedKey, env.Salt, env.Name, password, "password")
	if err != nil {
		return nil, err
	}
	payload, err := openPayload(env.Ciphertext, dek)
	if err != nil {
		return nil, err
	}
	var profiles config.Store
	if err := json.Unmarshal(payload, &profiles); err != nil {
		return nil, fmt.Errorf("%w: payload: %v", ErrInvalidEnvelope, err)
	}
	return &profiles, nil
}

// Update replaces the encrypted profile payload while preserving the vault
// identity and both credential wrappers, including the current recovery key.
func Update(data []byte, name, password string, profiles *config.Store) ([]byte, error) {
	env, err := parse(data)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" || password == "" {
		return nil, fmt.Errorf("%w: credentials are required", ErrInvalidEnvelope)
	}
	if env.Name != name {
		return nil, fmt.Errorf("%w: credentials do not match", ErrInvalidEnvelope)
	}
	dek, err := unwrapKey(env.WrappedKey, env.Salt, env.Name, password, "password")
	if err != nil {
		return nil, err
	}
	if profiles == nil {
		profiles = &config.Store{}
	}
	payload, err := marshalPayload(profiles)
	if err != nil {
		return nil, err
	}
	ciphertext, err := sealPayload(payload, dek)
	if err != nil {
		return nil, err
	}
	env.Ciphertext = encode(ciphertext)
	return json.Marshal(env)
}

// Recover opens a vault with its recovery key and re-encrypts it with a new
// name, password, and recovery key. The vault ID is intentionally preserved.
// Once the returned envelope replaces the remote record, the old recovery key
// can no longer open the current vault.
func Recover(data []byte, recoveryKey, newName, newPassword string) (Registration, []byte, error) {
	env, err := parse(data)
	if err != nil {
		return Registration{}, nil, err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || newPassword == "" || recoveryKey == "" {
		return Registration{}, nil, fmt.Errorf("%w: recovery credentials are required", ErrInvalidEnvelope)
	}
	dek, err := unwrapKey(env.RecoveryWrappedKey, env.RecoverySalt, env.ID, recoveryKey, "recovery")
	if err != nil {
		return Registration{}, nil, err
	}
	payload, err := openPayload(env.Ciphertext, dek)
	if err != nil {
		return Registration{}, nil, err
	}
	newRecoveryKey, err := randomText(recoveryLen)
	if err != nil {
		return Registration{}, nil, err
	}
	rotated, err := sealEnvelope(env.ID, newName, newPassword, newRecoveryKey, dek, payload)
	if err != nil {
		return Registration{}, nil, err
	}
	encoded, err := json.Marshal(rotated)
	if err != nil {
		return Registration{}, nil, err
	}
	return Registration{ID: env.ID, Name: newName, RecoveryKey: newRecoveryKey}, encoded, nil
}

func sealEnvelope(id, name, password, recoveryKey string, dek, payload []byte) (envelope, error) {
	salt, err := randomBytes(saltSize)
	if err != nil {
		return envelope{}, err
	}
	recoverySalt, err := randomBytes(saltSize)
	if err != nil {
		return envelope{}, err
	}
	wrapped, err := wrapKey(dek, deriveKey(name, password, salt, "password"))
	if err != nil {
		return envelope{}, err
	}
	recoveryWrapped, err := wrapKey(dek, deriveKey(id, recoveryKey, recoverySalt, "recovery"))
	if err != nil {
		return envelope{}, err
	}
	ciphertext, err := sealPayload(payload, dek)
	if err != nil {
		return envelope{}, err
	}
	return envelope{
		Version:            formatVersion,
		ID:                 id,
		Name:               name,
		Salt:               encode(salt),
		Ciphertext:         encode(ciphertext),
		WrappedKey:         encode(wrapped),
		RecoverySalt:       encode(recoverySalt),
		RecoveryWrappedKey: encode(recoveryWrapped),
	}, nil
}

func parse(data []byte) (envelope, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return envelope{}, fmt.Errorf("%w: json: %v", ErrInvalidEnvelope, err)
	}
	if env.Version != formatVersion || env.ID == "" || env.Name == "" || env.Salt == "" || env.Ciphertext == "" || env.WrappedKey == "" || env.RecoverySalt == "" || env.RecoveryWrappedKey == "" {
		return envelope{}, fmt.Errorf("%w: missing or unsupported fields", ErrInvalidEnvelope)
	}
	return env, nil
}

func marshalPayload(profiles *config.Store) ([]byte, error) {
	return json.Marshal(profiles)
}

func deriveKey(identity, secret string, salt []byte, purpose string) []byte {
	input := []byte(purpose + "\x00" + identity + "\x00" + secret)
	return argon2.IDKey(input, salt, kdfTime, kdfMemory, kdfThreads, keySize)
}

func wrapKey(key, wrappingKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(wrappingKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := randomBytes(gcm.NonceSize())
	if err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, key, nil)...), nil
}

func unwrapKey(encoded, encodedSalt, identity, secret, purpose string) ([]byte, error) {
	salt, err := decode(encodedSalt)
	if err != nil {
		return nil, fmt.Errorf("%w: salt: %v", ErrInvalidEnvelope, err)
	}
	wrapped, err := decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: wrapped key: %v", ErrInvalidEnvelope, err)
	}
	if len(wrapped) < 12 {
		return nil, fmt.Errorf("%w: wrapped key is too short", ErrInvalidEnvelope)
	}
	block, err := aes.NewCipher(deriveKey(identity, secret, salt, purpose))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ciphertext := wrapped[:gcm.NonceSize()], wrapped[gcm.NonceSize():]
	key, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: credentials do not match", ErrInvalidEnvelope)
	}
	return key, nil
}

func sealPayload(payload, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := randomBytes(gcm.NonceSize())
	if err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, payload, nil)...), nil
}

func openPayload(encoded string, key []byte) ([]byte, error) {
	sealed, err := decode(encoded)
	if err != nil || len(sealed) < 12 {
		return nil, fmt.Errorf("%w: payload is invalid", ErrInvalidEnvelope)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	payload, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("%w: payload authentication failed", ErrInvalidEnvelope)
	}
	return payload, nil
}

func newIdentifier() (string, error) {
	b, err := randomBytes(identifierLen)
	if err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(b[:4]), hex.EncodeToString(b[4:6]), hex.EncodeToString(b[6:8]), hex.EncodeToString(b[8:10]), hex.EncodeToString(b[10:])), nil
}

func randomText(n int) (string, error) {
	b, err := randomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func encode(b []byte) string { return base64.RawStdEncoding.EncodeToString(b) }

func decode(s string) ([]byte, error) { return base64.RawStdEncoding.DecodeString(s) }
