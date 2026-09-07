package proudnet

import (
	"fmt"
	"strings"
)

// 0xA427 RequestClanChat [ProudString text] -> 0xA64D / 0xA64E [u32 reason]
// 0xA64F NotifyClanChat  [ProudString speaker][ProudString text]
const (
	rmiAnswerClanChatOK   uint16 = 0xA64D
	rmiAnswerClanChatFail uint16 = 0xA64E
	rmiNotifyClanChat     uint16 = 0xA64F

	rmiRequestChatAll    uint16 = 0x9471
	rmiRequestChatUser   uint16 = 0x9473
	rmiRequestChatNotice uint16 = 0x9474

	maxChatLength = 200
)

func clanChatRow(speaker, text string) []byte {
	return WriteProudString(WriteProudString(nil, speaker), text)
}

func (c *Clan) chat(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("clan chat before authentication")
	}

	text, _, err := ReadProudString(msg.Body, 0)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: malformed chat body=%x: %v", msg.Body, err))
		return c.refuseChat(session)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return c.refuseChat(session)
	}
	if len(text) > maxChatLength {
		text = text[:maxChatLength]
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, found, err := c.clans.Find(ctx, player.ClanName)
	if err != nil || !found {
		c.logger.Info(fmt.Sprintf("clan: %s talked with no clan behind them", player.Name))
		return c.refuseChat(session)
	}

	if err := session.SendPlain(Message{ID: rmiAnswerClanChatOK}); err != nil {
		return err
	}
	c.logger.Info(fmt.Sprintf("clan chat [%s] %s: %s", clan.Name, player.Name, text))
	c.broadcast(clan, Message{ID: rmiNotifyClanChat, Body: clanChatRow(player.Name, text)})
	return nil
}

func (c *Clan) refuseChat(session *Session) error {
	return session.SendPlain(Message{ID: rmiAnswerClanChatFail, Body: appendUint32(nil, 0)})
}
