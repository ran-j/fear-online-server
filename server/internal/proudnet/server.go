package proudnet

import (
	"errors"
	"fmt"
	"net"

	"go-service-template/pkg/logger"
)

type Handler func(session *Session, msg Message) error

type Server struct {
	logger   logger.Interface
	keyPair  *KeyPair
	host     string
	port     uint16
	listener net.Listener
	handlers map[uint16]Handler
	sessions *SessionRegistry
	rooms    *RoomRegistry
}

func (s *Server) UseRooms(rooms *RoomRegistry) {
	s.rooms = rooms
}

func NewServer(logger logger.Interface, keyPair *KeyPair, host string, port uint16) *Server {
	return &Server{
		logger:   logger,
		keyPair:  keyPair,
		host:     host,
		port:     port,
		handlers: make(map[uint16]Handler),
		sessions: NewSessionRegistry(),
	}
}

func (s *Server) Handle(rmiID uint16, handler Handler) {
	s.handlers[rmiID] = handler
}

func (s *Server) Sessions() *SessionRegistry {
	return s.sessions
}

func (s *Server) Listen(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener
	go s.acceptLoop()
	return nil
}

func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	s.logger.Info("proudnet: client connected " + remote)
	defer s.logger.Info("proudnet: client disconnected " + remote)

	session := &Session{conn: conn, server: s}
	defer s.releaseSession(session)
	var pending []byte
	readBuffer := make([]byte, 4096)

	for {
		n, err := conn.Read(readBuffer)
		if err != nil {
			return
		}
		pending = append(pending, readBuffer[:n]...)

		frames, remaining, parseErr := ParseFrames(pending)
		if parseErr != nil {
			s.logger.Warn("proudnet: parse error from " + remote + ": " + parseErr.Error())
			return
		}
		pending = remaining

		for _, frame := range frames {
			if err := s.handleFrame(session, frame); err != nil {
				s.logger.Error("proudnet: " + err.Error())
				return
			}
		}
	}
}

func (s *Server) handleFrame(session *Session, frame Frame) error {
	switch {
	case IsClientHello(frame):
		if err := session.sendReliable(BuildConnectionParams(s.keyPair.PublicDER)); err != nil {
			return fmt.Errorf("write connection params: %w", err)
		}
		s.logger.Info("proudnet: replied to client hello with connection params + public key")
		return nil

	case frameType(frame) == msgCryptoSetup:
		sessionKey, err := ParseCryptoSetup(s.keyPair, frame.Payload)
		if err != nil {
			return fmt.Errorf("crypto setup: %w", err)
		}
		session.sessionKey = sessionKey
		session.established = true
		s.logger.Info(fmt.Sprintf("proudnet: session established (%d-byte key)", len(sessionKey)))
		if err := session.sendReliable(serverAuthSuccess()); err != nil {
			return fmt.Errorf("write server auth success: %w", err)
		}
		return nil

	case frameType(frame) == msgClientConnect:
		connect, err := parseClientConnect(frame.Payload)
		if err != nil {
			return fmt.Errorf("client connect: %w", err)
		}
		session.connectGUID = connect.guid
		reply := buildServerConnectSuccess(connect, s.host, s.port)
		if err := session.sendReliable(reply); err != nil {
			return fmt.Errorf("write server connect success: %w", err)
		}
		s.logger.Info("proudnet: replied NotifyServerConnectSuccess")
		return nil

	case frameType(frame) == msgHeartbeat:
		if err := session.sendReliable(frame.Payload); err != nil {
			return fmt.Errorf("echo heartbeat: %w", err)
		}
		return nil

	case frameType(frame) == envelopeEncrypted:
		return s.handleEncrypted(session, frame)

	case frameType(frame) == envelopePlain:
		s.handlePlain(session, frame)
		return nil

	default:
		s.logger.Debug(fmt.Sprintf("proudnet: unhandled frame type 0x%02x len=%d", frameType(frame), len(frame.Payload)))
		return nil
	}
}

func (s *Server) handleEncrypted(session *Session, frame Frame) error {
	if !session.established {
		return errors.New("encrypted rmi before session established")
	}
	msg, count, err := decodeEncrypted(session.sessionKey, frame.Payload)
	if err != nil {
		return fmt.Errorf("decode encrypted rmi: %w", err)
	}
	s.logger.Info(fmt.Sprintf("proudnet: RX encrypted rmi 0x%04x count=%d len=%d", msg.ID, count, len(msg.Body)))
	s.dispatch(session, msg)
	return nil
}

func (s *Server) handlePlain(session *Session, frame Frame) {
	msg, ok := decodePlain(frame.Payload)
	if !ok {
		s.logger.Debug("proudnet: malformed plain rmi")
		return
	}
	if msg.IsInternal() {
		s.logger.Debug(fmt.Sprintf("proudnet: RX internal control 0x%04x", msg.ID))
		return
	}
	s.dispatch(session, msg)
}

func (s *Server) dispatch(session *Session, msg Message) {
	handler, ok := s.handlers[msg.ID]
	if !ok {
		s.logger.Info(fmt.Sprintf("proudnet: no handler for rmi 0x%04x len=%d", msg.ID, len(msg.Body)))
		return
	}
	if err := handler(session, msg); err != nil {
		s.logger.Error(fmt.Sprintf("proudnet: handler 0x%04x: %v", msg.ID, err))
	}
}

func frameType(frame Frame) byte {
	if len(frame.Payload) == 0 {
		return 0
	}
	return frame.Payload[0]
}

func (s *Server) releaseSession(session *Session) {
	if room := sessionRoom(session); room != nil && s.rooms != nil {
		if _, _, empty := room.RemoveUser(sessionRoomUser(session)); empty {
			s.rooms.Close(room.Number)
			s.logger.Info(fmt.Sprintf("proudnet: room %d closed, its last player dropped", room.Number))
		}
		setSessionRoom(session, nil)
	}
	s.sessions.Unbind(session)
}
