package proudnet

import (
	"encoding/binary"
	"fmt"

	"go-service-template/internal/models"
)

// TODO not working yet
// In-match equipment. The hangar loadout does not travel into the game on its
// own, and 0x909D is a *query*, not an announcement: it carries a single byte,
// the index of the player being asked about, and the server is the authority
// that answers with that player's set.
//
//	0x909D RequestUserEquip [u8]        -> 0x9263 [u8] / 0x9264 [u32 reason]
//	0x9223 NotifyUserEquipList          list of Game::UserEquip
//	0x9224 NotifyUserEquipInfo          one Game::UserEquip
//	0x921F NotifyTeamEquipList          list of Game::TeamEquip
//
// Game::UserEquip is 77 bytes, read by FUN_100CD420. Its constructor installs
// the nested types by name, so the blocks are not a guess:
//
//	[u8 gameUserIndex][Game::Weapon 5 x u32][Game::Custom 6 x u32]
//	[Game::Perk 3 x u32][5 x u32]
//
// Game::TeamEquip is the same shape without weapons and customs:
//
//	[u8 gameUserIndex][Game::Perk 3 x u32][5 x u32]
const (
	rmiRequestUserEquip    uint16 = 0x909D
	rmiAnswerUserEquipOK   uint16 = 0x9263
	rmiAnswerUserEquipFail uint16 = 0x9264
	rmiNotifyUserEquipList uint16 = 0x9223
	rmiNotifyUserEquipInfo uint16 = 0x9224
	rmiNotifyTeamEquipList uint16 = 0x921F
)

type gameUserEquip struct {
	UserIndex uint8
	Weapon    [5]uint32
	Custom    [6]uint32
	Perk      [3]uint32
	Unkn      [5]uint32 // unmapped, sent as zero
}

func (e gameUserEquip) row() []byte {
	body := []byte{e.UserIndex}
	for _, block := range [][]uint32{e.Weapon[:], e.Custom[:], e.Perk[:], e.Unkn[:]} {
		for _, value := range block {
			body = appendUint32(body, value)
		}
	}
	return body
}

func buildUserEquip(index uint8, player models.Player) gameUserEquip {
	equip := gameUserEquip{UserIndex: index}

	indexBySerial := make(map[uint16]uint32, len(player.Inventory))
	for _, item := range player.Inventory {
		indexBySerial[item.Serial] = item.ItemIndex
	}

	for _, equipped := range player.Loadout {
		if equipped.Slot >= models.SlotWeaponFirst && equipped.Slot <= models.SlotWeaponLast {
			equip.Weapon[equipped.Slot-models.SlotWeaponFirst] = indexBySerial[equipped.Serial]
		}
	}
	for _, mounted := range player.Attachments {
		switch {
		case mounted.Slot >= models.SlotCustomPartFirst && mounted.Slot <= models.SlotCustomPartLast:
			equip.Custom[mounted.Slot-models.SlotCustomPartFirst] = indexBySerial[mounted.ChildSerial]
		case mounted.Slot >= models.SlotPerkFirst && mounted.Slot <= models.SlotPerkLast:
			equip.Perk[mounted.Slot-models.SlotPerkFirst] = indexBySerial[mounted.ChildSerial]
		}
	}
	return equip
}

func (l *Lobby) userEquip(session *Session, msg Message) error {
	room := sessionRoom(session)
	if room == nil {
		return session.SendPlain(Message{ID: rmiAnswerUserEquipFail, Body: appendUint32(nil, 0)})
	}
	if len(msg.Body) < 1 {
		l.logger.Info(fmt.Sprintf("lobby: malformed user equip body=%x", msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerUserEquipFail, Body: appendUint32(nil, 0)})
	}

	asked := msg.Body[0]
	seat, found := room.Member(uint32(asked))
	if !found {
		l.logger.Info(fmt.Sprintf("lobby: room %d has no game user %d", room.Number, asked))
		return session.SendPlain(Message{ID: rmiAnswerUserEquipFail, Body: appendUint32(nil, 0)})
	}

	ctx, cancel := context5s()
	defer cancel()
	accounts, err := l.players.FindMany(ctx, []string{seat.SteamID})
	if err != nil {
		l.logger.Error(fmt.Sprintf("lobby: load equipment of %s: %v", seat.Name, err))
		return session.SendPlain(Message{ID: rmiAnswerUserEquipFail, Body: appendUint32(nil, 0)})
	}

	equip := buildUserEquip(asked, accounts[seat.SteamID])
	l.logger.Info(fmt.Sprintf("lobby: room %d sending %s's set weapons=%v perks=%v",
		room.Number, seat.Name, equip.Weapon, equip.Perk))
	if err := session.SendPlain(Message{ID: rmiAnswerUserEquipOK, Body: []byte{asked}}); err != nil {
		return err
	}
	return session.SendPlain(Message{ID: rmiNotifyUserEquipInfo, Body: equip.row()})
}

// Changing weapons mid-match. Pressing the loadout key in game sends the whole
// new set at once, and until this was handled the server kept serving the hangar
// loadout while the player carried something else.
//
//	0x909A RequestChangeWeapon [u8][u16 x5] -> 0x925B [u8] / 0x925C [u8 reason]
//	0x925D NotifyUserWeaponInfo [u8][Game::Weapon][Game::Custom]
const (
	rmiRequestChangeWeapon    uint16 = 0x909A
	rmiAnswerChangeWeaponOK   uint16 = 0x925B
	rmiAnswerChangeWeaponFail uint16 = 0x925C
	rmiNotifyUserWeaponInfo   uint16 = 0x925D

	changeWeaponSlots = 5
	changeWeaponBytes = 1 + changeWeaponSlots*2
)

func (l *Lobby) changeWeapon(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("weapon change before authentication")
	}
	if len(msg.Body) < changeWeaponBytes {
		l.logger.Info(fmt.Sprintf("lobby: malformed weapon change body=%x", msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerChangeWeaponFail, Body: []byte{0}})
	}

	serials := make([]uint16, changeWeaponSlots)
	for slot := range serials {
		serials[slot] = binary.LittleEndian.Uint16(msg.Body[1+slot*2:])
	}

	ctx, cancel := context5s()
	defer cancel()
	updated, err := l.players.ChangeWeapons(ctx, player.SteamID, serials)
	if err != nil {
		l.logger.Info(fmt.Sprintf("lobby: %s cannot carry %v: %v", player.Name, serials, err))
		return session.SendPlain(Message{ID: rmiAnswerChangeWeaponFail, Body: []byte{0}})
	}
	updatePlayer(session, updated)

	l.logger.Info(fmt.Sprintf("lobby: %s changed weapons to %v (body=%x)", player.Name, serials, msg.Body))
	if err := session.SendPlain(Message{ID: rmiAnswerChangeWeaponOK, Body: []byte{msg.Body[0]}}); err != nil {
		return err
	}
	if room := sessionRoom(session); room != nil {
		equip := buildUserEquip(l.seatOf(session), updated)
		l.tellMatch(room, Message{ID: rmiNotifyUserWeaponInfo, Body: equip.row()[:1+(5+6)*4]})
	}
	return nil
}
