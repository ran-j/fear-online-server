package proudnet

import "go-service-template/internal/models"

const (
	rmiSendMemberList    uint16 = 0xA60F
	rmiNotifyMemberInfo  uint16 = 0xA610
	rmiNotifyMemberCount uint16 = 0xA642
)

type clanMemberPresence struct {
	Channel byte
	Lobby   byte
	Room    uint16
	Online  bool
	InGame  bool
}

type clanMemberWire struct {
	MemberID uint16
	Name     string
	Level    byte

	PvP        clanMemberRecordWire
	ClanRecord clanMemberRecordWire

	Channel  byte
	Lobby    byte
	Room     uint16
	Position byte
	Online   bool
	InGame   bool
}
type clanMemberRecordWire struct {
	Win   uint32
	Lose  uint32
	Draw  uint32
	Kill  uint32
	Death uint32
	Extra uint32
}

func makeClanMemberWire(member models.ClanMember, player models.Player, presence clanMemberPresence) clanMemberWire {
	name := member.Name
	if name == "" {
		name = player.Name
	}

	draw := player.Records.PvP.Draw
	if draw == 0 && player.Records.PvP.Games >= player.Records.PvP.Win+player.Records.PvP.Lose {
		draw = player.Records.PvP.Games - player.Records.PvP.Win - player.Records.PvP.Lose
	}

	return clanMemberWire{
		MemberID: member.MemberID,
		Name:     name,
		Level:    levelByte(player),
		PvP: clanMemberRecordWire{
			Win:   player.Records.PvP.Win,
			Lose:  player.Records.PvP.Lose,
			Draw:  draw,
			Kill:  player.Records.PvP.Kill,
			Death: player.Records.PvP.Death,
			Extra: player.Records.PvP.Headshot,
		},
		ClanRecord: clanMemberRecordWire{
			Win:   nonNegativeUint32(member.Records.Win),
			Lose:  nonNegativeUint32(member.Records.Lose),
			Draw:  nonNegativeUint32(member.Records.Draw),
			Kill:  nonNegativeUint32(member.Records.Kill),
			Death: nonNegativeUint32(member.Records.Death),
		},
		Channel:  presence.Channel,
		Lobby:    presence.Lobby,
		Room:     presence.Room,
		Position: member.Rank,
		Online:   presence.Online,
		InGame:   presence.InGame,
	}
}

// appendClanMember serializes Common::Standard::Clan::Member:
//
//	[u16 memberId]
//	[ProudString name]
//	[u8 level]
//	[6 x u32 PvP record: win, lose, draw, kill, death, extra]
//	[6 x u32 clan record: win, lose, draw, kill, death, extra]
//	[u8 channel]
//	[u8 lobby]
//	[u16 room]
//	[u8 position]
//	[u8 online]
//	[u8 inGame]
func appendClanMember(dst []byte, member clanMemberWire) []byte {
	dst = appendUint16(dst, member.MemberID)
	dst = WriteProudString(dst, member.Name)
	dst = append(dst, member.Level)
	dst = appendClanMemberRecord(dst, member.PvP)
	dst = appendClanMemberRecord(dst, member.ClanRecord)
	dst = append(dst, member.Channel, member.Lobby)
	dst = appendUint16(dst, member.Room)
	dst = append(dst, member.Position, boolByte(member.Online), boolByte(member.InGame))
	return dst
}

func appendClanMemberRecord(dst []byte, record clanMemberRecordWire) []byte {
	for _, value := range []uint32{
		record.Win,
		record.Lose,
		record.Draw,
		record.Kill,
		record.Death,
		record.Extra,
	} {
		dst = appendUint32(dst, value)
	}
	return dst
}

func clanMemberList(members []clanMemberWire) []byte {
	body := EncodeVarInt(len(members))
	for _, member := range members {
		body = appendClanMember(body, member)
	}
	return body
}

func sendClanMemberList(session *Session, members []clanMemberWire) error {
	return session.SendPlain(Message{ID: rmiSendMemberList, Body: clanMemberList(members)})
}

func notifyClanMember(session *Session, member clanMemberWire) error {
	return session.SendPlain(Message{ID: rmiNotifyMemberInfo, Body: appendClanMember(nil, member)})
}

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}

type clanMemberCounts struct {
	CurrentTotal byte
	Managers     byte
	Members      byte
	Associates   byte
	Capacity     byte
}

func makeClanMemberCounts(clan models.Clan) clanMemberCounts {
	counts := clanMemberCounts{CurrentTotal: clampByte(len(clan.Members))}
	for _, member := range clan.Members {
		switch member.Rank {
		case models.ClanRankManager:
			counts.Managers = incrementByte(counts.Managers)
		case models.ClanRankMember:
			counts.Members = incrementByte(counts.Members)
		case models.ClanRankAssociate:
			counts.Associates = incrementByte(counts.Associates)
		}
	}

	capacity := clan.MemberCapacity
	if capacity == 0 {
		capacity = models.DefaultClanMemberCapacity
	}
	if counts.CurrentTotal > capacity {
		capacity = counts.CurrentTotal
	}
	counts.Capacity = capacity
	return counts
}

func (counts clanMemberCounts) row() []byte {
	return []byte{
		counts.CurrentTotal,
		counts.Managers,
		counts.Members,
		counts.Associates,
		counts.Capacity,
	}
}

func notifyClanMemberCount(session *Session, clan models.Clan) error {
	return session.SendPlain(Message{ID: rmiNotifyMemberCount, Body: makeClanMemberCounts(clan).row()})
}

func incrementByte(value byte) byte {
	if value == 0xFF {
		return value
	}
	return value + 1
}
