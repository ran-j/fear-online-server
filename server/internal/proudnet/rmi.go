package proudnet

import (
	"encoding/binary"
	"errors"
)

const (
	envelopeEncrypted byte = 0x26
	envelopePlain     byte = 0x01
	contextApp        byte = 0x01

	// internalRMIFloor is the id at/above which an RMI is a ProudNet control
	// message (0xFA01 heartbeat, 0xFA0D, ...) rather than an application call.
	internalRMIFloor uint16 = 0xFA00
)

// Message is an application RMI: an id and its serialized parameters.
type Message struct {
	ID   uint16
	Body []byte
}

// IsInternal reports whether the message is a ProudNet control RMI.
func (m Message) IsInternal() bool { return m.ID >= internalRMIFloor }

// decodeEncrypted decrypts a 0x26 envelope, returning the RMI and the client's
// running decrypt counter. Plaintext layout: uint16 count, context byte, uint16
// id, params.
func decodeEncrypted(sessionKey, payload []byte) (msg Message, count uint16, err error) {
	if len(payload) == 0 || payload[0] != envelopeEncrypted {
		return Message{}, 0, errors.New("proudnet: not an encrypted envelope")
	}
	plain, err := AESECBDecrypt(sessionKey, payload[1:])
	if err != nil {
		return Message{}, 0, err
	}
	if len(plain) < 5 {
		return Message{}, 0, errors.New("proudnet: encrypted rmi too short")
	}
	count = binary.LittleEndian.Uint16(plain)
	id := binary.LittleEndian.Uint16(plain[3:])
	return Message{ID: id, Body: plain[5:]}, count, nil
}

// encodeEncrypted builds a 0x26 envelope for msg with the server's counter.
func encodeEncrypted(sessionKey []byte, count uint16, msg Message) ([]byte, error) {
	plain := make([]byte, 0, 5+len(msg.Body))
	plain = appendUint16(plain, count)
	plain = append(plain, contextApp)
	plain = appendUint16(plain, msg.ID)
	plain = append(plain, msg.Body...)

	encrypted, err := AESECBEncrypt(sessionKey, plain)
	if err != nil {
		return nil, err
	}
	return append([]byte{envelopeEncrypted}, encrypted...), nil
}

// decodePlain reads a plaintext 0x01 RMI (internal control or app fallback).
func decodePlain(payload []byte) (Message, bool) {
	if len(payload) < 3 || payload[0] != envelopePlain {
		return Message{}, false
	}
	return Message{ID: binary.LittleEndian.Uint16(payload[1:]), Body: payload[3:]}, true
}

// encodePlain builds a plaintext 0x01 RMI.
func encodePlain(msg Message) []byte {
	out := make([]byte, 0, 3+len(msg.Body))
	out = append(out, envelopePlain)
	out = appendUint16(out, msg.ID)
	return append(out, msg.Body...)
}
