package proudnet_test

import (
	"testing"

	"go-service-template/internal/proudnet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsClientHello(t *testing.T) {
	hello := proudnet.Frame{Marker: proudnet.MarkerReliable, Payload: []byte{0x32, 0x0f, 0x00, 0x00, 0x40}}
	assert.True(t, proudnet.IsClientHello(hello))

	wrongType := proudnet.Frame{Marker: proudnet.MarkerReliable, Payload: []byte{0x05, 0x0f, 0x00, 0x00, 0x40}}
	assert.False(t, proudnet.IsClientHello(wrongType))

	wrongMarker := proudnet.Frame{Marker: proudnet.MarkerUnreliable, Payload: []byte{0x32, 0x0f, 0x00, 0x00, 0x40}}
	assert.False(t, proudnet.IsClientHello(wrongMarker))
}

func TestBuildConnectionParamsCarriesPublicKey(t *testing.T) {
	keyPair, err := proudnet.GenerateKeyPair()
	require.NoError(t, err)

	params := proudnet.BuildConnectionParams(keyPair.PublicDER)
	assert.Equal(t, byte(0x05), params[0])

	// The public key is the trailing byte array; read its length prefix back.
	length, n, err := proudnet.DecodeVarInt(params, len(params)-len(keyPair.PublicDER)-varIntLen(len(keyPair.PublicDER)))
	assert.NoError(t, err)
	assert.Equal(t, len(keyPair.PublicDER), length)
	assert.Equal(t, varIntLen(len(keyPair.PublicDER)), n)
}

func TestReadByteArrayRoundTrip(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	encoded := append(proudnet.EncodeVarInt(len(data)), data...)

	read, next, err := proudnet.ReadByteArray(encoded, 0)
	assert.NoError(t, err)
	assert.Equal(t, data, read)
	assert.Equal(t, len(encoded), next)
}

func TestParseCryptoSetup(t *testing.T) {
	keyPair, err := proudnet.GenerateKeyPair()
	require.NoError(t, err)

	sessionKey := []byte("0123456789abcdef")
	rsaCiphertext := encryptForServer(t, keyPair.PublicDER, sessionKey)
	proof, err := proudnet.AESECBEncrypt(sessionKey, []byte("proof block"))
	require.NoError(t, err)

	payload := []byte{0x06}
	payload = append(payload, proudnet.EncodeVarInt(len(rsaCiphertext))...)
	payload = append(payload, rsaCiphertext...)
	payload = append(payload, proudnet.EncodeVarInt(len(proof))...)
	payload = append(payload, proof...)

	negotiated, err := proudnet.ParseCryptoSetup(keyPair, payload)
	assert.NoError(t, err)
	assert.Equal(t, sessionKey, negotiated)
}

func varIntLen(value int) int {
	return len(proudnet.EncodeVarInt(value))
}
