package proudnet

import "errors"

// Connection-phase message types, read from the first byte of a frame payload.
const (
	msgCryptoSetup       byte = 0x06 // client: RSA/AES session key exchange
	msgServerAuthSuccess byte = 0x07 // server: handshake authenticated
	msgClientConnect     byte = 0x08 // client: connect (token + guid)
	msgServerConnectOK   byte = 0x0B // server: NotifyServerConnectSuccess
	msgHeartbeat         byte = 0x1C // client<->server: time-sync, echoed back

	serverHostID uint32 = 0x32
)

// serverAuthSuccess is the single-byte message the server sends right after the
// crypto handshake to signal the connection is authenticated. The client waits
// on it before continuing.
func serverAuthSuccess() []byte {
	return []byte{msgServerAuthSuccess}
}

type clientConnect struct {
	token []byte
	guid  []byte // 16 bytes
}

func parseClientConnect(payload []byte) (clientConnect, error) {
	token, next, err := ReadByteArray(payload, 1)
	if err != nil {
		return clientConnect{}, err
	}
	if next+16 > len(payload) {
		return clientConnect{}, errors.New("proudnet: 0x08 missing 16-byte client guid")
	}
	return clientConnect{token: token, guid: payload[next : next+16]}, nil
}

// buildServerConnectSuccess is the 0x0B reply: it echoes the client's guid/token
// and advertises the server's host id and address.
func buildServerConnectSuccess(connect clientConnect, host string, port uint16) []byte {
	out := []byte{msgServerConnectOK}
	out = appendUint32(out, serverHostID)
	out = append(out, connect.guid...)
	out = append(out, EncodeVarInt(len(connect.token))...)
	out = append(out, connect.token...)
	out = WriteProudString(out, host)
	out = appendUint16(out, port)
	return out
}
