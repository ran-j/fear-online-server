package proudnet

import (
	"encoding/binary"
	"fmt"
)

const (
	MarkerReliable   uint16 = 0x5713
	MarkerUnreliable uint16 = 0x5813

	maxFrameLength = 1024 * 1024
)

// Frame is a single ProudNet message: a marker plus its payload.
type Frame struct {
	Marker  uint16
	Payload []byte
}

// followUp is a server push sent right after an answer.
type followUp struct {
	id   uint16
	body []byte
}

// EncodeFrame wraps payload in a ProudNet frame: marker (uint16 LE) + length
// (signed varint) + payload.
func EncodeFrame(marker uint16, payload []byte) []byte {
	length := EncodeVarInt(len(payload))
	frame := make([]byte, 2, 2+len(length)+len(payload))
	binary.LittleEndian.PutUint16(frame, marker)
	frame = append(frame, length...)
	return append(frame, payload...)
}

// ParseFrames consumes every complete frame at the front of buf, returning the
// frames and the unconsumed remainder (a partial frame awaiting more bytes). A
// non-nil error means the stream is malformed and the connection should drop.
func ParseFrames(buf []byte) (frames []Frame, remaining []byte, err error) {
	offset := 0
	for len(buf)-offset >= 3 {
		marker := binary.LittleEndian.Uint16(buf[offset:])
		if marker != MarkerReliable && marker != MarkerUnreliable {
			return frames, buf[offset:], fmt.Errorf("proudnet: unknown marker 0x%04x", marker)
		}

		length, n, verr := DecodeVarInt(buf, offset+2)
		if verr == ErrIncomplete {
			break
		}
		if verr != nil {
			return frames, buf[offset:], verr
		}
		if length < 0 || length > maxFrameLength {
			return frames, buf[offset:], fmt.Errorf("proudnet: invalid frame length %d", length)
		}

		payloadOffset := offset + 2 + n
		frameEnd := payloadOffset + length
		if len(buf) < frameEnd {
			break
		}
		frames = append(frames, Frame{Marker: marker, Payload: buf[payloadOffset:frameEnd]})
		offset = frameEnd
	}
	return frames, buf[offset:], nil
}
