package plex

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

// Vault uses a separate persistent key. A lost or malformed existing key fails closed.
type Vault struct {
	aead     cipher.AEAD
	ClientID string
}

func OpenVault(dir string, encryptedRowsExist bool) (*Vault, error) {
	secrets := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secrets, 0700); err != nil {
		return nil, errors.New("Unable to initialize encryption directory")
	}
	key, err := persistentRandom(filepath.Join(secrets, "plex-key-v1"), 32, !encryptedRowsExist)
	if err != nil {
		return nil, err
	}
	client, err := persistentRandom(filepath.Join(secrets, "plex-client-id"), 16, true)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("Invalid encryption key")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("Unable to initialize encryption")
	}
	return &Vault{aead: aead, ClientID: hex.EncodeToString(client)}, nil
}
func persistentRandom(path string, n int, allowCreate bool) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && allowCreate {
		b = make([]byte, n)
		if _, err = rand.Read(b); err != nil {
			return nil, errors.New("Unable to generate encryption material")
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, errors.New("Unable to create encryption material")
		}
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return nil, errors.New("Unable to persist encryption material")
		}
	} else if err != nil {
		return nil, errors.New("Encryption material is missing or unreadable; restore the matching config backup")
	}
	if len(b) != n {
		return nil, errors.New("Encryption material is invalid; restore the matching config backup")
	}
	if err = os.Chmod(path, 0600); err != nil {
		return nil, errors.New("Unable to restrict encryption material permissions")
	}
	return b, nil
}
func (v *Vault) Encrypt(serverID, token string) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, errors.New("Unable to encrypt credential")
	}
	return v.aead.Seal(nonce, nonce, []byte(token), []byte("plex:v1:"+serverID)), nil
}
func (v *Vault) Decrypt(serverID string, version int, value []byte) (string, error) {
	if version != 1 || len(value) < v.aead.NonceSize() {
		return "", errors.New("Credential key version or ciphertext is invalid")
	}
	n := v.aead.NonceSize()
	plain, err := v.aead.Open(nil, value[:n], value[n:], []byte("plex:v1:"+serverID))
	if err != nil {
		return "", errors.New("Credential cannot be decrypted; restore the matching key")
	}
	return string(plain), nil
}
