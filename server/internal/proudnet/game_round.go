package proudnet

import "fmt"

const (
	rmiRequestCheckRespawn   uint16 = 0x908F
	rmiRequestRespawnInstant uint16 = 0x9090
	rmiRequestRespawn        uint16 = 0x9091
	rmiRequestTimeOut        uint16 = 0x9092
	rmiRequestDeath          uint16 = 0x9093
	rmiRequestDediUserPing   uint16 = 0x909C
	rmiRequestHitCount       uint16 = 0x9028
	rmiAnswerHitCountOK      uint16 = 0x905B

	rmiNotifyUserResultList uint16 = 0x9221
	rmiNotifyRoundEnd       uint16 = 0x922B
	rmiNotifyGameEnd        uint16 = 0x922C

	rmiAnswerCheckRespawnOK   uint16 = 0x923E
	rmiAnswerRespawnInstantOK uint16 = 0x9240
	rmiAnswerRespawnOK        uint16 = 0x9242
	rmiNotifyRespawn          uint16 = 0x9244
	rmiAnswerTimeOutOK        uint16 = 0x9245
	rmiAnswerDeathOK          uint16 = 0x9247
	rmiNotifyDeath            uint16 = 0x9249
	rmiAnswerDediUserPingOK   uint16 = 0x9261

	deathReportBytes = 10
)

// dediUserPing is a keep-alive the client sends every minute.
func (l *Lobby) dediUserPing(session *Session, _ Message) error {
	return session.SendPlain(Message{ID: rmiAnswerDediUserPingOK})
}

func (l *Lobby) hitCount(session *Session, _ Message) error {
	return session.SendPlain(Message{ID: rmiAnswerHitCountOK})
}

func (l *Lobby) checkRespawn(session *Session, _ Message) error {
	return session.SendPlain(Message{ID: rmiAnswerCheckRespawnOK, Body: []byte{l.seatOf(session)}})
}

func (l *Lobby) checkRespawnInstant(session *Session, _ Message) error {
	return session.SendPlain(Message{ID: rmiAnswerRespawnInstantOK, Body: []byte{l.seatOf(session)}})
}

func (l *Lobby) respawn(session *Session, _ Message) error {
	room := sessionRoom(session)
	if room == nil {
		return nil
	}

	seat := l.seatOf(session)
	if err := session.SendPlain(Message{ID: rmiAnswerRespawnOK, Body: []byte{seat}}); err != nil {
		return err
	}
	l.tellMatch(room, Message{ID: rmiNotifyRespawn, Body: []byte{seat}})
	return nil
}

func (l *Lobby) death(session *Session, msg Message) error {
	room := sessionRoom(session)
	if room == nil {
		return nil
	}
	if len(msg.Body) < deathReportBytes {
		l.logger.Info(fmt.Sprintf("game: short death report body=%x", msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerDeathOK})
	}

	killer, victim, assist := msg.Body[0], msg.Body[1], msg.Body[2]
	room.RecordDeath(uint32(killer), uint32(victim), uint32(assist))
	l.logger.Info(fmt.Sprintf("game: room %d killer=%d victim=%d assist=%d body=%x",
		room.Number, killer, victim, assist, msg.Body))

	if err := session.SendPlain(Message{ID: rmiAnswerDeathOK}); err != nil {
		return err
	}
	l.tellMatch(room, Message{ID: rmiNotifyDeath, Body: notifyDeathRow(msg.Body)})
	return nil
}

func (l *Lobby) timeOut(session *Session, _ Message) error {
	room := sessionRoom(session)
	if room == nil {
		return nil
	}
	if err := session.SendPlain(Message{ID: rmiAnswerTimeOutOK}); err != nil {
		return err
	}

	users := room.Members()
	l.tellMatch(room, Message{ID: rmiNotifyUserResultList, Body: userResultList(users)})

	round, last := room.FinishRound()
	if last {
		l.logger.Info(fmt.Sprintf("game: room %d finished after round %d", room.Number, round))
		l.tellMatch(room, Message{ID: rmiNotifyRoundEnd})
		l.tellMatch(room, Message{ID: rmiNotifyGameEnd, Body: []byte{winningTeam(users)}})
		return nil
	}

	l.logger.Info(fmt.Sprintf("game: room %d round %d over, %d next", room.Number, round, round+1))
	l.tellMatch(room, Message{ID: rmiNotifyRoundEnd})
	l.tellMatch(room, Message{ID: rmiNotifyRoundBegin, Body: []byte{round + 1}})
	return nil
}

func notifyDeathRow(report []byte) []byte {
	return []byte{report[0], report[1], report[2], report[7], report[8], report[9]}
}

// userResultList is varint(count) then each seat's Game::UserResult.
func userResultList(users []RoomUser) []byte {
	body := EncodeVarInt(len(users))
	for index, user := range users {
		body = appendUserResult(body, uint8(index+1), user)
	}
	return body
}

func appendUserResult(dst []byte, seat uint8, user RoomUser) []byte {
	dst = append(dst, seat, user.Team)
	for _, counter := range []uint16{
		user.Score.Deaths,  // wire +2, reaches ActionScript 7th
		user.Score.Assists, // wire +4, 8th
		user.Score.Kills,   // wire +6, 6th
		user.Score.Points,  // wire +8, 9th
		0,                  // wire +10, never seen used
	} {
		dst = appendUint16(dst, counter)
	}
	return append(dst, make([]byte, 7)...)
}

func winningTeam(users []RoomUser) byte {
	totals := map[uint8]uint16{}
	for _, user := range users {
		totals[user.Team] += user.Score.Kills
	}
	switch {
	case totals[1] > totals[2]:
		return 1
	case totals[2] > totals[1]:
		return 2
	default:
		return 0
	}
}

func (l *Lobby) seatOf(session *Session) byte {
	room := sessionRoom(session)
	if room == nil {
		return 1
	}
	return seatIndex(room.Members(), sessionRoomUser(session))
}
