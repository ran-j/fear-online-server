package proudnet

import "errors"

// ErrIncomplete means the buffer does not yet hold a full value; the caller
// should wait for more bytes rather than treating it as malformed.
var ErrIncomplete = errors.New("proudnet: incomplete")

var errVarIntTooLong = errors.New("proudnet: varint too long")

// EncodeVarInt writes ProudNet's signed varint: 7-bit little-endian groups with
// 0x80 as the continuation flag. When the final group has bit 0x40 set (which
// DecodeVarInt reads as a sign bit) an extra group is emitted so non-negative
// values round-trip.
func EncodeVarInt(value int) []byte {
	remaining := uint32(value)
	out := make([]byte, 0, 5)
	for remaining>>7 != 0 {
		out = append(out, byte(remaining&0x7f)|0x80)
		remaining >>= 7
	}
	if remaining&0x40 != 0 {
		out = append(out, byte(remaining&0x7f)|0x80)
		remaining = 0
	}
	return append(out, byte(remaining&0x7f))
}

// DecodeVarInt reads a signed varint at offset. It returns ErrIncomplete when
// the buffer runs out mid-value.
func DecodeVarInt(buf []byte, offset int) (value int, bytesRead int, err error) {
	var acc int32
	var shift uint
	for i := 0; i < 5; i++ {
		if offset+i >= len(buf) {
			return 0, 0, ErrIncomplete
		}
		b := buf[offset+i]
		acc |= int32(b&0x7f) << shift
		if b&0x80 == 0 {
			if b&0x40 != 0 {
				acc = ^acc
			}
			return int(acc), i + 1, nil
		}
		shift += 7
	}
	return 0, 0, errVarIntTooLong
}
