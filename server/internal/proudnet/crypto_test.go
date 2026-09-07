package proudnet_test

import (
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // matches the protocol under test.
	"crypto/x509"
	"testing"

	"go-service-template/internal/proudnet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encryptForServer(t *testing.T, publicDER, sessionKey []byte) []byte {
	t.Helper()
	parsed, err := x509.ParsePKIXPublicKey(publicDER)
	require.NoError(t, err)
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), randReader{}, parsed.(*rsa.PublicKey), sessionKey, nil)
	require.NoError(t, err)
	return ciphertext
}

func TestDecryptSessionKey(t *testing.T) {
	keyPair, err := proudnet.GenerateKeyPair()
	require.NoError(t, err)

	sessionKey := []byte("0123456789abcdef") // 16 bytes, an AES-128 key

	t.Run("as-is byte order", func(t *testing.T) {
		ciphertext := encryptForServer(t, keyPair.PublicDER, sessionKey)
		decrypted, err := keyPair.DecryptSessionKey(ciphertext)
		assert.NoError(t, err)
		assert.Equal(t, sessionKey, decrypted)
	})

	t.Run("reversed byte order", func(t *testing.T) {
		ciphertext := encryptForServer(t, keyPair.PublicDER, sessionKey)
		reversed := make([]byte, len(ciphertext))
		for i := range ciphertext {
			reversed[len(ciphertext)-1-i] = ciphertext[i]
		}
		decrypted, err := keyPair.DecryptSessionKey(reversed)
		assert.NoError(t, err)
		assert.Equal(t, sessionKey, decrypted)
	})
}

func TestAESECBRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := []byte("proudnet rmi body, not a block multiple")

	ciphertext, err := proudnet.AESECBEncrypt(key, plaintext)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ciphertext)%16)

	decrypted, err := proudnet.AESECBDecrypt(key, ciphertext)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted[:len(plaintext)]) // trailing zero padding stripped
}

func TestAESECBDecryptRejectsUnaligned(t *testing.T) {
	_, err := proudnet.AESECBDecrypt([]byte("0123456789abcdef"), []byte{0x01, 0x02, 0x03})
	assert.Error(t, err)
}

type randReader struct{}

func (randReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(i*7 + 1)
	}
	return len(p), nil
}
