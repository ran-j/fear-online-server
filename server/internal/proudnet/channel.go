package proudnet

import "fmt"

const (
	rmiRequestChannelList uint16 = 0x88B9
	rmiAnswerChannelList  uint16 = 0x891D
	rmiSendChannelList    uint16 = 0x891F

	rmiRequestChannelJoin uint16 = 0x88BA
	rmiAnswerChannelJoin  uint16 = 0x8920
)

// Channel is one advertised game server. Name is a string-database id (Loki.strdb), not display text.
type Channel struct {
	ServerID int32
	ID       uint8
	Kind     uint8
	Name     string
	Host     string
	Port     uint16
	MaxUsers uint16
}

func (c Channel) appendTo(dst []byte) []byte {
	dst = appendUint32(dst, uint32(c.ServerID))
	dst = append(dst, c.ID, c.Kind)
	dst = WriteProudString(dst, c.Name)
	dst = WriteProudString(dst, c.Host) // primary
	dst = appendUint16(dst, c.Port)
	dst = WriteProudString(dst, c.Host) // alternate
	dst = appendUint16(dst, c.Port)
	dst = appendUint16(dst, 0) // current users
	dst = appendUint16(dst, c.MaxUsers)
	return dst
}

func (h *Handlers) joinChannel(session *Session, _ Message) error {
	if len(h.channels) > 0 {
		setSessionChannel(session, h.channels[0].ID)
	}
	return session.SendPlain(Message{ID: rmiAnswerChannelJoin, Body: []byte{1}})
}

func (h *Handlers) requestChannelList(session *Session, _ Message) error {
	if err := session.SendPlain(Message{ID: rmiAnswerChannelList}); err != nil {
		return err
	}
	return h.sendChannelList(session)
}

func (h *Handlers) sendChannelList(session *Session) error {
	body := EncodeVarInt(len(h.channels))
	for _, channel := range h.channels {
		body = channel.appendTo(body)
	}
	h.logger.Info(fmt.Sprintf("login: advertising %d channel(s)", len(h.channels)))
	return session.SendPlain(Message{ID: rmiSendChannelList, Body: body})
}
