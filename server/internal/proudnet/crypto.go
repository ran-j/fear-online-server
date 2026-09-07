package proudnet

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SHA-1 is dictated by the client's ProudNet OAEP.
	"crypto/x509"
	"fmt"
)

// KeyPair is the server's per-process RSA-1024 key. The public half (SPKI DER)
// is advertised to the client during the handshake; the client encrypts the
// session key against it and the private half decrypts it.
type KeyPair struct {
	private   *rsa.PrivateKey
	PublicDER []byte
}

func GenerateKeyPair() (*KeyPair, error) {
	private, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil {
		return nil, err
	}
	return &KeyPair{private: private, PublicDER: publicDER}, nil
}

// DecryptSessionKey RSA-OAEP(SHA-1) decrypts the client's session key. ProudNet
// may transmit the ciphertext byte-reversed, so both orders are attempted.
func (k *KeyPair) DecryptSessionKey(ciphertext []byte) ([]byte, error) {
	if key, err := rsa.DecryptOAEP(sha1.New(), nil, k.private, ciphertext, nil); err == nil {
		return key, nil
	}
	key, err := rsa.DecryptOAEP(sha1.New(), nil, k.private, reverseBytes(ciphertext), nil)
	if err != nil {
		return nil, fmt.Errorf("proudnet: session key decrypt failed in both byte orders: %w", err)
	}
	return key, nil
}

// AESECBEncrypt encrypts with AES-ECB and no padding, zero-padding the plaintext
// up to a 16-byte multiple.
func AESECBEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, ((len(plaintext)+15)/16)*16)
	copy(out, plaintext)
	for i := 0; i < len(out); i += 16 {
		block.Encrypt(out[i:i+16], out[i:i+16])
	}
	return out, nil
}

// AESECBDecrypt decrypts AES-ECB with no padding; the ciphertext must be a
// 16-byte multiple.
func AESECBDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%16 != 0 {
		return nil, fmt.Errorf("proudnet: ciphertext length %d is not a multiple of 16", len(ciphertext))
	}
	out := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += 16 {
		block.Decrypt(out[i:i+16], ciphertext[i:i+16])
	}
	return out, nil
}

func reverseBytes(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}
