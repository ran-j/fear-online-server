package proudnet_test

import (
	"bytes"
	"testing"

	"go-service-template/internal/proudnet"

	"github.com/stretchr/testify/assert"
)

func TestEncodeVarInt(t *testing.T) {
	cases := map[int][]byte{
		0:   {0x00},
		63:  {0x3f},
		64:  {0xC0, 0x00}, // 0x40 in the final group would read as a sign bit
		127: {0xFF, 0x00},
		128: {0x80, 0x01},
	}
	for value, expected := range cases {
		assert.Equal(t, expected, proudnet.EncodeVarInt(value), "encode %d", value)
	}
}

func TestVarIntRoundTrip(t *testing.T) {
	for _, value := range []int{0, 1, 63, 64, 127, 128, 300, 1024, 65535, 1048576} {
		encoded := proudnet.EncodeVarInt(value)
		decoded, n, err := proudnet.DecodeVarInt(encoded, 0)
		assert.NoError(t, err, "value %d", value)
		assert.Equal(t, value, decoded, "value %d", value)
		assert.Equal(t, len(encoded), n, "value %d", value)
	}
}

func TestDecodeVarIntIncomplete(t *testing.T) {
	_, _, err := proudnet.DecodeVarInt([]byte{0x80}, 0)
	assert.ErrorIs(t, err, proudnet.ErrIncomplete)
}

func TestFrameRoundTrip(t *testing.T) {
	payload := []byte{0x75, 0xCD, 0x01, 0x02, 0x03}
	encoded := proudnet.EncodeFrame(proudnet.MarkerReliable, payload)

	frames, remaining, err := proudnet.ParseFrames(encoded)
	assert.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Len(t, frames, 1)
	assert.Equal(t, proudnet.MarkerReliable, frames[0].Marker)
	assert.True(t, bytes.Equal(payload, frames[0].Payload))
}

func TestParseFramesMultipleAndPartial(t *testing.T) {
	first := proudnet.EncodeFrame(proudnet.MarkerReliable, []byte{0xAA})
	second := proudnet.EncodeFrame(proudnet.MarkerUnreliable, []byte{0xBB, 0xCC})

	stream := append(append([]byte{}, first...), second...)
	stream = append(stream, first[:2]...) // a third frame, only partially arrived

	frames, remaining, err := proudnet.ParseFrames(stream)
	assert.NoError(t, err)
	assert.Len(t, frames, 2)
	assert.Equal(t, []byte{0xAA}, frames[0].Payload)
	assert.Equal(t, []byte{0xBB, 0xCC}, frames[1].Payload)
	assert.Equal(t, first[:2], remaining) // the leftover partial frame is preserved
}

func TestParseFramesUnknownMarker(t *testing.T) {
	_, _, err := proudnet.ParseFrames([]byte{0x00, 0x00, 0x01, 0xFF})
	assert.Error(t, err)
}
