package proudnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// IsClientHello reports whether frame is the ProudNet connection probe the
// client opens with (type 0x32 + one of two magic values).
func IsClientHello(frame Frame) bool {
	if frame.Marker != MarkerReliable || len(frame.Payload) < 5 || frame.Payload[0] != 0x32 {
		return false
	}
	magic := binary.LittleEndian.Uint32(frame.Payload[1:])
	return magic == 0x4000000f || magic == 0x40000010
}

// BuildConnectionParams builds the server's reply to the client hello: a type
// 0x05 CNetConnectionParam block (Fast encryption, 128-bit key, no RC4) followed
// by the server's public key as a ProudNet byte array. The fixed byte layout is
// what the client's parser expects.
func BuildConnectionParams(publicKeyDER []byte) []byte {
	buf := new(bytes.Buffer)
	buf.WriteByte(0x05) // message type
	buf.WriteByte(0x00) // leading bool

	buf.WriteByte(0x00)     // protocol version
	putUint32(buf, 0x10000) // param[1]
	putUint32(buf, 0x10000) // param[2]
	putUint32(buf, 60000)   // param[3] timeout
	buf.WriteByte(0x00)     // param[4]
	putUint32(buf, 0x00)    // param[5]
	buf.WriteByte(0x01)     // param[6] encryption method = Fast
	putUint32(buf, 0x80)    // param[7] first key length = 128 bits
	putUint32(buf, 0x00)    // param[8] second key length = 0 (no RC4)
	buf.WriteByte(0x00)     // 0x24
	buf.WriteByte(0x01)     // 0x25
	buf.WriteByte(0x01)     // 0x26
	buf.WriteByte(0x00)     // 0x27
	buf.WriteByte(0x01)     // 0x2c
	buf.WriteByte(0x00)     // 0x2d
	buf.WriteByte(0x00)     // 0x2e
	putUint32(buf, 0x00)    // param[10]

	buf.Write(EncodeVarInt(len(publicKeyDER)))
	buf.Write(publicKeyDER)
	return buf.Bytes()
}

// ParseCryptoSetup reads the client's 0x06 message — an RSA-encrypted session
// key followed by an AES-encrypted proof key — and returns the negotiated
// AES-128 session key used for every encrypted RMI afterwards.
func ParseCryptoSetup(keyPair *KeyPair, payload []byte) ([]byte, error) {
	rsaKey, next, err := ReadByteArray(payload, 1)
	if err != nil {
		return nil, err
	}
	secondKey, _, err := ReadByteArray(payload, next)
	if err != nil {
		return nil, err
	}
	sessionKey, err := keyPair.DecryptSessionKey(rsaKey)
	if err != nil {
		return nil, err
	}
	if _, err := AESECBDecrypt(sessionKey, secondKey); err != nil {
		return nil, fmt.Errorf("proudnet: proof key decrypt failed: %w", err)
	}
	return sessionKey, nil
}

// ReadByteArray reads a ProudNet byte array (varint length + raw bytes) at
// offset, returning the data and the offset just past it.
func ReadByteArray(buf []byte, offset int) (data []byte, next int, err error) {
	length, n, err := DecodeVarInt(buf, offset)
	if err != nil {
		return nil, 0, err
	}
	start := offset + n
	end := start + length
	if length < 0 || end > len(buf) {
		return nil, 0, fmt.Errorf("proudnet: invalid byte array length %d", length)
	}
	return buf[start:end], end, nil
}

func putUint32(buf *bytes.Buffer, value uint32) {
	var scratch [4]byte
	binary.LittleEndian.PutUint32(scratch[:], value)
	buf.Write(scratch[:])
}
