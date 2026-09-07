package proudnet

import (
	"encoding/binary"
	"fmt"
	"net"
)

const (
	rmiRequestGameStart    uint16 = 0x8CA7
	rmiAnswerGameStartOK   uint16 = 0x8D1E
	rmiAnswerGameStartFail uint16 = 0x8D1F
	rmiNotifyGameStart     uint16 = 0x9057
	rmiNotifyHostServer    uint16 = 0x921C
	rmiNotifyRoomGame      uint16 = 0x8D09

	lithTechPort uint16 = 27889
	userKeyKind  uint8  = 1
)

func notifyGameStartRow(host string, port uint16, userID uint32, ticket uint16) []byte {
	body := WriteProudString(nil, host)
	body = appendUint16(body, port)
	body = appendUint32(body, userID)
	body = append(body, userKeyKind)

	key := make([]byte, guidBytes)
	binary.LittleEndian.PutUint16(key, ticket)
	return append(body, key...)
}

func notifyHostServerRow(host string, port uint16, isHost, intrude bool) []byte {
	body := WriteProudString(nil, host)
	body = appendUint16(body, port)
	return append(body, boolByte(isHost), boolByte(intrude))
}

func (l *Lobby) gameStart(session *Session, _ Message) error {
	room := sessionRoom(session)
	if room == nil {
		l.logger.Info("lobby: game start without a room")
		return session.SendPlain(Message{ID: rmiAnswerGameStartFail, Body: appendUint32(nil, 0)})
	}
	if sessionRoomUser(session) != room.LeaderID {
		l.logger.Info(fmt.Sprintf("lobby: room %d start refused, only the leader starts", room.Number))
		return session.SendPlain(Message{ID: rmiAnswerGameStartFail, Body: appendUint32(nil, 0)})
	}
	return l.beginMatch(session, room)
}

func (l *Lobby) beginMatch(leader *Session, room *Room) error {
	host, ok := hostAddress(leader)
	if !ok {
		l.logger.Error(fmt.Sprintf("lobby: room %d has no usable host address", room.Number))
		return leader.SendPlain(Message{ID: rmiAnswerGameStartFail, Body: appendUint32(nil, 0)})
	}
	room.Host = host

	l.broadcastRoom(room, Message{ID: rmiNotifyRoomGame, Body: []byte{1}})
	l.logger.Info(fmt.Sprintf("lobby: room %d starting %s, host %s:%d, game server %s:%d",
		room.Number, room.MapName, host, lithTechPort, l.gameHost, l.gamePort))

	for _, member := range l.sessions.InRoom(room.Number) {
		seat := sessionRoomUser(member)
		ticket := l.tickets.issue(room, seat)
		if err := member.SendPlain(Message{ID: rmiAnswerGameStartOK}); err != nil {
			l.logger.Error(fmt.Sprintf("lobby: start ack for room %d: %v", room.Number, err))
			continue
		}
		if err := member.SendPlain(Message{
			ID:   rmiNotifyGameStart,
			Body: notifyGameStartRow(l.gameHost, l.gamePort, seat, ticket),
		}); err != nil {
			l.logger.Error(fmt.Sprintf("lobby: game start for room %d: %v", room.Number, err))
		}
	}
	return nil
}

func hostAddress(session *Session) (string, bool) {
	addr, ok := session.conn.RemoteAddr().(*net.TCPAddr)
	if !ok || addr.IP == nil {
		return "", false
	}
	return addr.IP.String(), true
}
