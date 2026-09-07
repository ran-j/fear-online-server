package proudnet

import "go-service-template/internal/models"

const (
	rmiSendRequestList      uint16 = 0xA616
	rmiNotifyRequestInfo    uint16 = 0xA617
	rmiNotifyDeleteRequest  uint16 = 0xA618
	rmiSendClearRequest     uint16 = 0xA619
	rmiSendRequestClanInfo  uint16 = 0xA60D
	rmiSendClearRequestInfo uint16 = 0xA60E
)

// appendClanRequest serializes Common::Standard::Clan::Request, the row behind
// the master's applicant table:
//
//	[u16 applicantId]
//	[ProudString name]
//	[u8 level]
//	[6 x u32 PvP record: win, lose, draw, kill, death, extra]
//	[u16 year][u8 month][u8 day]
func appendClanRequest(dst []byte, applicant models.ClanApplicant, player models.Player) []byte {
	dst = appendUint16(dst, applicant.ApplicantID)
	dst = WriteProudString(dst, applicant.Name)
	dst = append(dst, clampByte(int(applicant.Level)))

	pvp := player.Records.PvP
	dst = appendClanMemberRecord(dst, clanMemberRecordWire{
		Win:   pvp.Win,
		Lose:  pvp.Lose,
		Draw:  pvp.Draw,
		Kill:  pvp.Kill,
		Death: pvp.Death,
		Extra: pvp.Headshot,
	})

	appliedAt := applicant.AppliedAt.UTC()
	dst = appendUint16(dst, uint16(appliedAt.Year()))
	return append(dst, byte(appliedAt.Month()), byte(appliedAt.Day()))
}

func clanRequestList(applicants []models.ClanApplicant, accounts map[string]models.Player) []byte {
	body := EncodeVarInt(len(applicants))
	for _, applicant := range applicants {
		body = appendClanRequest(body, applicant, accounts[applicant.SteamID])
	}
	return body
}

func notifyClanRequest(session *Session, applicant models.ClanApplicant, player models.Player) error {
	return session.SendPlain(Message{ID: rmiNotifyRequestInfo, Body: appendClanRequest(nil, applicant, player)})
}

func notifyDeleteClanRequest(session *Session, applicantID uint16) error {
	return session.SendPlain(Message{ID: rmiNotifyDeleteRequest, Body: appendUint16(nil, applicantID)})
}

func appendRequestClanInfo(dst []byte, clan models.Clan) []byte {
	dst = WriteProudString(dst, clan.Name)
	dst = appendUint32(dst, clanWireID(clan.Name))
	return appendUint32(dst, packClanMark(clan))
}

func packClanMark(clan models.Clan) uint32 {
	mark := func(index int, bits uint32) uint32 {
		value := clan.Mark[index]
		if value < 0 {
			return 0
		}
		if uint32(value) > bits {
			return bits
		}
		return uint32(value)
	}
	return mark(0, 0xFFFF) | mark(1, 0xFF)<<16 | mark(2, 0xFF)<<24
}

func sendRequestClanInfo(session *Session, clan models.Clan) error {
	return session.SendPlain(Message{ID: rmiSendRequestClanInfo, Body: appendRequestClanInfo(nil, clan)})
}

func sendClearRequestInfo(session *Session) error {
	return session.SendPlain(Message{ID: rmiSendClearRequestInfo})
}
