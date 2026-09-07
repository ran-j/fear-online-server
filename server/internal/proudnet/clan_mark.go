package proudnet

import (
	"encoding/binary"
	"fmt"

	"go-service-template/internal/models"
	"go-service-template/internal/services"
)

// 0xA429 RequestCheckMark  [4 x u16 mark] -> 0xA653 [u8 bool] / 0xA654
// 0xA42A RequestChangeMark [4 x u16 mark] -> 0xA655 / 0xA656
// 0xA657 NotifyClanMark    [4 x u16 mark]
const (
	rmiRequestCheckMark  uint16 = 0xA429
	rmiAnswerCheckMarkOK uint16 = 0xA653
	rmiAnswerCheckMarkNo uint16 = 0xA654

	rmiRequestChangeMark  uint16 = 0xA42A
	rmiAnswerChangeMarkOK uint16 = 0xA655
	rmiAnswerChangeMarkNo uint16 = 0xA656

	rmiNotifyClanMark uint16 = 0xA657

	clanMarkSize = 8
)

func checkMarkVerdict(available bool) []byte {
	return []byte{boolByte(available)}
}

func (c *Clan) registerMark(server *Server) {
	server.Handle(rmiRequestCheckMark, c.checkMark)
	server.Handle(rmiRequestChangeMark, c.changeMark)
}

func (c *Clan) checkMark(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("check clan mark before authentication")
	}
	mark, _, ok := parseClanMark(msg.Body)
	if !ok {
		c.logger.Info(fmt.Sprintf("clan: malformed mark body=%x", msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerCheckMarkNo})
	}

	ctx, cancel := c.context()
	defer cancel()
	available, err := c.clans.MarkAvailable(ctx, player, mark)
	if err != nil {
		c.logger.Error(fmt.Sprintf("clan: check mark %v: %v", mark, err))
		return session.SendPlain(Message{ID: rmiAnswerCheckMarkNo})
	}

	c.logger.Info(fmt.Sprintf("clan: mark %v available=%t for %s", mark, available, player.Name))
	return session.SendPlain(Message{ID: rmiAnswerCheckMarkOK, Body: checkMarkVerdict(available)})
}

func (c *Clan) changeMark(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("change clan mark before authentication")
	}
	mark, markExtra, ok := parseClanMark(msg.Body)
	if !ok {
		c.logger.Info(fmt.Sprintf("clan: malformed mark body=%x", msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerChangeMarkNo})
	}
	ctx, cancel := c.context()
	defer cancel()

	if current, found, err := c.clans.Find(ctx, player.ClanName); err == nil && found &&
		current.Mark == mark && current.MarkExtra == markExtra {
		c.logger.Info(fmt.Sprintf("clan: %q already wears %v, nothing spent", current.Name, mark))
		return session.SendPlain(Message{ID: rmiAnswerChangeMarkOK})
	}

	if _, owns := c.players.FindConsumable(player, services.FunctionClanMarkChange); !owns {
		c.logger.Info(fmt.Sprintf("clan: %s has no Clan Mark Change item", player.Name))
		return session.SendPlain(Message{ID: rmiAnswerChangeMarkNo})
	}

	clan, err := c.clans.ChangeMark(ctx, player, mark, markExtra)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not repaint %q: %v", player.Name, player.ClanName, err))
		return session.SendPlain(Message{ID: rmiAnswerChangeMarkNo})
	}

	c.logger.Info(fmt.Sprintf("clan: %s repainted %q to %v", player.Name, clan.Name, mark))
	if err := session.SendPlain(Message{ID: rmiAnswerChangeMarkOK}); err != nil {
		return err
	}

	c.broadcast(clan, Message{ID: rmiNotifyClanMark, Body: appendClanMark(nil, clan)})
	return c.spendMarkChange(session, player)
}

func (c *Clan) spendMarkChange(session *Session, player models.Player) error {
	ctx, cancel := c.context()
	defer cancel()
	updated, spent, err := c.players.SpendConsumable(ctx, player.SteamID, services.FunctionClanMarkChange)
	if err != nil {
		c.logger.Error(fmt.Sprintf("clan: spend mark change for %s: %v", player.Name, err))
		return nil
	}
	updatePlayer(session, updated)

	if spent.UsedUp {
		return session.SendPlain(Message{ID: rmiSendDeleteItem, Body: appendUint16(nil, spent.Serial)})
	}
	return session.SendPlain(Message{ID: rmiSendItemValue, Body: itemValueRow(spent.Remaining)})
}

func parseClanMark(body []byte) ([3]int32, uint16, bool) {
	if len(body) < clanMarkSize {
		return [3]int32{}, 0, false
	}
	mark := [3]int32{
		int32(binary.LittleEndian.Uint16(body[0:2])),
		int32(binary.LittleEndian.Uint16(body[2:4])),
		int32(binary.LittleEndian.Uint16(body[4:6])),
	}
	return mark, binary.LittleEndian.Uint16(body[6:8]), true
}
