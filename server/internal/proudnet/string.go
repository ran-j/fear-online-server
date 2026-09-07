package proudnet

import (
	"errors"
	"unicode/utf16"
)

// WriteProudString appends a ProudNet UTF-16 string: a varint character count
// followed by the UTF-16LE bytes.
func WriteProudString(dst []byte, value string) []byte {
	units := utf16.Encode([]rune(value))
	dst = append(dst, EncodeVarInt(len(units))...)
	for _, unit := range units {
		dst = append(dst, byte(unit), byte(unit>>8))
	}
	return dst
}

// ReadProudString reads a ProudNet UTF-16 string at offset, returning the value
// and the offset just past it.
func ReadProudString(buf []byte, offset int) (value string, next int, err error) {
	charLen, n, err := DecodeVarInt(buf, offset)
	if err != nil {
		return "", 0, err
	}
	start := offset + n
	end := start + charLen*2
	if charLen < 0 || end > len(buf) {
		return "", 0, errors.New("proudnet: invalid utf-16 string length")
	}
	units := make([]uint16, charLen)
	for i := 0; i < charLen; i++ {
		units[i] = uint16(buf[start+i*2]) | uint16(buf[start+i*2+1])<<8
	}
	return string(utf16.Decode(units)), end, nil
}
