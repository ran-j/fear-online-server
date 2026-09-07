package proudnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptedEnvelopeRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	original := Message{ID: 0x75CD, Body: []byte{0xAA, 0xBB, 0xCC}}

	payload, err := encodeEncrypted(key, 7, original)
	assert.NoError(t, err)
	assert.Equal(t, envelopeEncrypted, payload[0])

	decoded, count, err := decodeEncrypted(key, payload)
	assert.NoError(t, err)
	assert.Equal(t, uint16(7), count)
	assert.Equal(t, original.ID, decoded.ID)
	// AES-ECB zero-pads to a block; the real length is known from the RMI schema.
	assert.Equal(t, original.Body, decoded.Body[:len(original.Body)])
}

func TestPlainEnvelopeRoundTrip(t *testing.T) {
	original := Message{ID: 0xFA01, Body: []byte{0x01}}

	decoded, ok := decodePlain(encodePlain(original))
	assert.True(t, ok)
	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.Body, decoded.Body)
	assert.True(t, decoded.IsInternal())
}

func TestDecodeEncryptedRejectsWrongEnvelope(t *testing.T) {
	_, _, err := decodeEncrypted([]byte("0123456789abcdef"), []byte{envelopePlain, 0x00})
	assert.Error(t, err)
}
