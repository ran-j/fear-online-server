package proudnet

import (
	"io"
	"net"
	"sync"
)

// Session is the per-connection ProudNet state: the transport, the negotiated
// session key and the outgoing encryption counter.
type Session struct {
	conn         net.Conn
	server       *Server
	sendMu       sync.Mutex
	sessionKey   []byte
	established  bool
	encryptCount uint16

	// connectGUID is the 16-byte client guid from the 0x08 connect message
	connectGUID []byte

	// State carries per-connection application data (e.g. the logged-in
	// account), set and read by the RMI handlers.
	State interface{}
}

// Established reports whether the crypto handshake completed.
func (s *Session) Established() bool { return s.established }

// SendEncrypted encrypts and sends an application RMI, advancing the counter
// (which wraps at 16 bits, matching the client).
func (s *Session) SendEncrypted(msg Message) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	payload, err := encodeEncrypted(s.sessionKey, s.encryptCount, msg)
	if err != nil {
		return err
	}
	s.encryptCount++
	return s.writeReliableLocked(payload)
}

// SendPlain sends a plaintext application RMI. The write lock is shared with
// encrypted and control frames so asynchronous pushes cannot interleave bytes.
func (s *Session) SendPlain(msg Message) error {
	return s.sendReliable(encodePlain(msg))
}

func (s *Session) sendReliable(payload []byte) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.writeReliableLocked(payload)
}

func (s *Session) writeReliableLocked(payload []byte) error {
	frame := EncodeFrame(MarkerReliable, payload)
	for len(frame) != 0 {
		written, err := s.conn.Write(frame)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrUnexpectedEOF
		}
		frame = frame[written:]
	}
	return nil
}
