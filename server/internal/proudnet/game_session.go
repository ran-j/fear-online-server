package proudnet

import (
	"encoding/binary"
	"fmt"
	"sync"

	"go-service-template/internal/models"
)

const (
	rmiAnswerConnectServer  uint16 = 0x9025
	rmiRequestMatchStart    uint16 = 0x9089
	rmiAnswerMatchStartOK   uint16 = 0x9232
	rmiAnswerMatchStartFail uint16 = 0x9233

	rmiNotifyMapInfo          uint16 = 0x9219
	rmiNotifyGameInfo         uint16 = 0x921A
	rmiNotifyInGameItem       uint16 = 0x921B
	rmiNotifyGameUserList     uint16 = 0x921D
	rmiNotifyUserFunctionList uint16 = 0x9225

	userKeyBytes = 21
	guidBytes    = 16
)

type matchSeat struct {
	ticket uint16
	userID uint32
}

type matchTickets struct {
	mu     sync.Mutex
	next   uint16
	bySeat map[matchSeat]*Room
}

func newMatchTickets() *matchTickets {
	return &matchTickets{bySeat: make(map[matchSeat]*Room)}
}

// issue reserves a place in a match and returns the id to advertise.
func (t *matchTickets) issue(room *Room, userID uint32) uint16 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	t.bySeat[matchSeat{ticket: t.next, userID: userID}] = room
	return t.next
}

func (t *matchTickets) redeem(ticket uint16, userID uint32) (*Room, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	room, ok := t.bySeat[matchSeat{ticket: ticket, userID: userID}]
	return room, ok
}

func (l *Lobby) connectServer(session *Session, _ Message) error {
	l.logger.Info("game: client reports its game connection is up")
	return nil
}

func (l *Lobby) matchStart(session *Session, msg Message) error {
	if len(msg.Body) < userKeyBytes {
		l.logger.Info(fmt.Sprintf("game: match start too short body=%x", msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerMatchStartFail, Body: appendUint32(nil, 0)})
	}

	userID := binary.LittleEndian.Uint32(msg.Body[0:4])
	ticket := binary.LittleEndian.Uint16(msg.Body[5:7])
	room, ok := l.tickets.redeem(ticket, userID)
	if !ok {
		l.logger.Info(fmt.Sprintf("game: no match waiting for ticket %d seat %d (body=%x)", ticket, userID, msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerMatchStartFail, Body: appendUint32(nil, 0)})
	}

	seat, seated := room.Member(userID)
	if !seated {
		l.logger.Info(fmt.Sprintf("game: room %d no longer seats user %d", room.Number, userID))
		return session.SendPlain(Message{ID: rmiAnswerMatchStartFail, Body: appendUint32(nil, 0)})
	}
	bindMatchSession(session, l.loadSeat(seat), room, userID)
	l.matches.join(room.Number, session)

	users := room.Members()
	l.logger.Info(fmt.Sprintf("game: room %d user %d joined the match on %s, %d in the snapshot",
		room.Number, userID, room.MapName, len(users)))

	for _, out := range l.matchSnapshot(room, users, userID) {
		if err := session.SendPlain(out); err != nil {
			return err
		}
	}
	return session.SendPlain(Message{ID: rmiAnswerMatchStartOK})
}

func bindMatchSession(session *Session, player models.Player, room *Room, userID uint32) {
	session.State = &sessionData{player: player, room: room, roomUser: userID}
}

func (l *Lobby) loadSeat(seat RoomUser) models.Player {
	ctx, cancel := context5s()
	defer cancel()

	accounts, err := l.players.FindMany(ctx, []string{seat.SteamID})
	if err != nil {
		l.logger.Error(fmt.Sprintf("game: load account of %s: %v", seat.Name, err))
		return models.Player{}
	}
	return accounts[seat.SteamID]
}

func (l *Lobby) matchSnapshot(room *Room, users []RoomUser, viewer uint32) []Message {
	viewerSeat, _ := room.Member(viewer)
	equipList := EncodeVarInt(len(users))
	for index, user := range users {
		equipList = append(equipList, l.matchEquip(uint8(index+1), user).row()...)
	}

	return []Message{
		{ID: rmiNotifyMapInfo, Body: append(roomMapInfo(room), 0)},
		{ID: rmiNotifyGameInfo, Body: []byte{seatIndex(users, viewer), seatIndex(users, room.LeaderID), viewerSeat.Team, viewerSeat.Team}},
		{ID: rmiNotifyInGameItem, Body: make([]byte, 3)},
		{ID: rmiNotifyGameUserList, Body: gameUserList(users)},
		{ID: rmiNotifyTeamEquipList, Body: EncodeVarInt(0)},
		{ID: rmiNotifyUserEquipList, Body: equipList},
		{ID: rmiNotifyUserFunctionList, Body: EncodeVarInt(0)},
		{ID: rmiNotifyHostServer, Body: notifyHostServerRow(room.Host, lithTechPort, viewer == room.LeaderID, false)},
	}
}

func (l *Lobby) matchEquip(index uint8, user RoomUser) gameUserEquip {
	return buildUserEquip(index, l.loadSeat(user))
}

// gameUserList is varint(count) then each seat as Common::Standard::Game::User.
func gameUserList(users []RoomUser) []byte {
	body := EncodeVarInt(len(users))
	for index, user := range users {
		body = appendGameUser(body, uint8(index+1), user)
	}
	return body
}

func appendGameUser(dst []byte, seat uint8, user RoomUser) []byte {
	dst = append(dst, seat)
	dst = WriteProudString(dst, user.Name)
	dst = WriteProudString(dst, user.ClanName)
	dst = appendClanMark(dst, user.Clan)
	dst = append(dst, boolByte(user.ClanName != ""), user.Team)
	dst = appendGameUserScore(dst, user.Score)
	return append(dst, 0, 0)
}

func appendGameUserScore(dst []byte, score Score) []byte {
	for _, counter := range []uint16{score.Kills, score.Deaths, score.Assists, score.Points} {
		dst = appendUint16(dst, counter)
	}
	return dst
}

const (
	rmiRequestLoadPercent     uint16 = 0x908B
	rmiRequestLoadComplete    uint16 = 0x908C
	rmiAnswerLoadPercentOK    uint16 = 0x9236
	rmiAnswerLoadCompleteOK   uint16 = 0x9238
	rmiNotifyLoadPercent      uint16 = 0x922F
	rmiNotifyUserLoadComplete uint16 = 0x9230
	rmiNotifyLoadComplete     uint16 = 0x9231
	rmiRequestRoomGameLeave   uint16 = 0x9027
	rmiRequestGameLeave       uint16 = 0x90A2
)

type matchSessions struct {
	mu     sync.Mutex
	byRoom map[uint16][]*Session
	loaded map[*Session]bool
}

func newMatchSessions() *matchSessions {
	return &matchSessions{byRoom: make(map[uint16][]*Session), loaded: make(map[*Session]bool)}
}

func (m *matchSessions) join(room uint16, session *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byRoom[room] = append(m.byRoom[room], session)
}

func (m *matchSessions) in(room uint16) []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*Session(nil), m.byRoom[room]...)
}

// markLoaded records one arrival and reports whether the match is now complete.
func (m *matchSessions) markLoaded(room uint16, session *Session) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loaded[session] = true
	for _, member := range m.byRoom[room] {
		if !m.loaded[member] {
			return false
		}
	}
	return true
}

func (m *matchSessions) leave(room uint16, session *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.loaded, session)

	rest := m.byRoom[room][:0]
	for _, member := range m.byRoom[room] {
		if member != session {
			rest = append(rest, member)
		}
	}
	if len(rest) == 0 {
		delete(m.byRoom, room)
		return
	}
	m.byRoom[room] = rest
}

// loadPercent relays one player's progress bar to the rest of the match.
func (l *Lobby) loadPercent(session *Session, msg Message) error {
	room := sessionRoom(session)
	if room == nil || len(msg.Body) < 1 {
		return nil
	}

	if err := session.SendPlain(Message{ID: rmiAnswerLoadPercentOK}); err != nil {
		return err
	}
	seat := seatIndex(room.Members(), sessionRoomUser(session))
	l.tellMatch(room, Message{ID: rmiNotifyLoadPercent, Body: []byte{seat, msg.Body[0]}})
	return nil
}

func (l *Lobby) loadComplete(session *Session, _ Message) error {
	room := sessionRoom(session)
	if room == nil {
		return nil
	}

	if err := session.SendPlain(Message{ID: rmiAnswerLoadCompleteOK}); err != nil {
		return err
	}
	seat := seatIndex(room.Members(), sessionRoomUser(session))
	l.tellMatch(room, Message{ID: rmiNotifyUserLoadComplete, Body: []byte{seat}})

	if l.matches.markLoaded(room.Number, session) {
		l.logger.Info(fmt.Sprintf("game: room %d fully loaded, the match is on", room.Number))
		l.tellMatch(room, Message{ID: rmiNotifyLoadComplete})
	}
	return nil
}

func (l *Lobby) matchLeave(session *Session, _ Message) error {
	room := sessionRoom(session)
	if room == nil {
		return nil
	}
	l.logger.Info(fmt.Sprintf("game: room %d user %d left the match", room.Number, sessionRoomUser(session)))
	l.matches.leave(room.Number, session)
	return nil
}

func (l *Lobby) tellMatch(room *Room, msg Message) {
	for _, member := range l.matches.in(room.Number) {
		if err := member.SendPlain(msg); err != nil {
			l.logger.Error(fmt.Sprintf("game: push 0x%04x to room %d: %v", msg.ID, room.Number, err))
		}
	}
}

func seatIndex(users []RoomUser, userID uint32) uint8 {
	for index, user := range users {
		if user.ID == userID {
			return uint8(index + 1)
		}
	}
	return 1
}

const (
	rmiRequestTimeStart     uint16 = 0x908D
	rmiRequestBeginRound    uint16 = 0x908E
	rmiAnswerTimeStartOK    uint16 = 0x923A
	rmiAnswerBeginRoundOK   uint16 = 0x923C
	rmiAnswerBeginRoundFail uint16 = 0x923D
	rmiNotifyRoundBegin     uint16 = 0x922A
)

func (l *Lobby) timeStart(session *Session, _ Message) error {
	return session.SendPlain(Message{ID: rmiAnswerTimeStartOK})
}

func (l *Lobby) beginRound(session *Session, _ Message) error {
	room := sessionRoom(session)
	if room == nil {
		return session.SendPlain(Message{ID: rmiAnswerBeginRoundFail})
	}

	if err := session.SendPlain(Message{ID: rmiAnswerBeginRoundOK}); err != nil {
		return err
	}
	round := room.NextRound()
	l.logger.Info(fmt.Sprintf("game: room %d round %d begins", room.Number, round))
	l.tellMatch(room, Message{ID: rmiNotifyRoundBegin, Body: []byte{round}})
	return nil
}
