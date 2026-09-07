package proudnet

import (
	"encoding/binary"
	"fmt"
	"sync"

	"go-service-template/internal/models"
)

const (
	rmiRequestRoomCreate uint16 = 0x8CA1
	rmiRequestRoomLeave  uint16 = 0x8CA3

	rmiNotifyOpenMapInfo uint16 = 0x8D2C
	rmiNotifyRoomInfo    uint16 = 0x8D05
	rmiNotifyRoomMapInfo uint16 = 0x8D06
	rmiSendRoomUserList  uint16 = 0x8D0A
	rmiNotifyTeamList    uint16 = 0x8D10
	rmiNotifyRoomLeader  uint16 = 0x8D08
	rmiAnswerRoomCreate  uint16 = 0x8D12
	rmiAnswerRoomLeave   uint16 = 0x8D16

	rmiAnswerRoomJoinOK   uint16 = 0x8D14
	rmiAnswerRoomJoinFail uint16 = 0x8D15
	rmiNotifyRoomUserInfo uint16 = 0x8D0B
	rmiNotifyTeamInfo     uint16 = 0x8D11

	rmiRequestRoomReady uint16 = 0x8CA6
	rmiAnswerRoomReady  uint16 = 0x8D1C
	rmiNotifyStateInfo  uint16 = 0x8D0E

	wireStateWait  uint8 = 0
	wireStateReady uint8 = 2
)

type Room struct {
	mu sync.RWMutex

	Number   uint16
	Title    string
	Password string
	MapIndex uint32
	MapName  string
	Host     string

	MaxUsers         uint8
	DisplayModeIndex uint8
	ObjectiveLimit   uint8
	RoundLimit       uint8

	RoomType     uint8
	Public       bool
	State        uint8
	TeamBalance  bool
	Instruction  bool
	DeathChat    bool
	PerkUse      bool
	ThirdViewUse bool

	LeaderID uint32
	Users    []RoomUser
	Ready    bool
	Round    uint8
}

func (r *Room) NextRound() uint8 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Round == 0 {
		r.Round = 1
	}
	return r.Round
}

func (r *Room) AddUser(user RoomUser) (RoomUser, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Users) >= int(r.MaxUsers) {
		return RoomUser{}, false
	}
	highest := uint32(0)
	for _, seated := range r.Users {
		if seated.ID > highest {
			highest = seated.ID
		}
	}
	user.ID = highest + 1
	r.Users = append(r.Users, user)
	return user, true
}

func (r *Room) RemoveUser(id uint32) (RoomUser, bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, user := range r.Users {
		if user.ID != id {
			continue
		}
		r.Users = append(r.Users[:i:i], r.Users[i+1:]...)
		return user, true, len(r.Users) == 0
	}
	return RoomUser{}, false, len(r.Users) == 0
}

func (r *Room) ToggleReady(userID uint32) (uint8, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Users {
		if r.Users[i].ID != userID {
			continue
		}
		if r.Users[i].WireState == wireStateReady {
			r.Users[i].WireState = wireStateWait
		} else {
			r.Users[i].WireState = wireStateReady
		}
		r.Ready = r.allReadyLocked()
		return r.Users[i].WireState, true
	}
	return wireStateWait, false
}

func (r *Room) allReadyLocked() bool {
	for _, user := range r.Users {
		if user.ID != uint32(r.LeaderID) && user.WireState != wireStateReady {
			return false
		}
	}
	return true
}

// Member finds a seat by the id the room knows it by.
func (r *Room) Member(id uint32) (RoomUser, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.Users {
		if user.ID == id {
			return user, true
		}
	}
	return RoomUser{}, false
}

func (r *Room) Members() []RoomUser {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]RoomUser(nil), r.Users...)
}

func (r *Room) NextTeam() uint8 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	first := 0
	for _, user := range r.Users {
		if user.Team == 1 {
			first++
		}
	}
	if first*2 <= len(r.Users) {
		return 1
	}
	return 2
}

func (r *Room) IsFull() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Users) >= int(r.MaxUsers)
}

type RoomUser struct {
	ID        uint32
	Name      string
	Level     uint8
	Team      uint8
	WireState uint8

	SteamID  string
	ClanName string
	Clan     models.Clan

	Score Score
}

type Score struct {
	Kills   uint16
	Deaths  uint16
	Assists uint16
	Points  uint16
}

func (r *Room) RecordDeath(killer, victim, assist uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Users {
		switch id := r.Users[i].ID; {
		case id == victim:
			r.Users[i].Score.Deaths++
		case id == killer && killer != victim:
			r.Users[i].Score.Kills++
		case id == assist && assist != 0:
			r.Users[i].Score.Assists++
		}
	}
}

func (r *Room) FinishRound() (uint8, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Round == 0 {
		r.Round = 1
	}
	played := r.Round
	if r.RoundLimit != 0 && played >= r.RoundLimit {
		return played, true
	}
	r.Round = played + 1
	return played, false
}

func (l *Lobby) createRoom(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("room create before authentication")
	}

	room, err := l.parseCreate(msg.Body, player)
	if err != nil {
		return err
	}
	l.rooms.Open(room)
	setSessionRoom(session, room)
	setSessionRoomUser(session, room.Users[0].ID)
	// TODO loggin this flags because idk what they are yet
	l.logger.Info(fmt.Sprintf("lobby: room %d created title=%q map=%s(%d) users=%d/%d password=%t public=%t state=%d",
		room.Number, room.Title, room.MapName, room.MapIndex, len(room.Users), room.MaxUsers, room.Password != "", room.Public, room.State))

	state := []followUp{
		{rmiNotifyOpenMapInfo, openMapInfo(room)},
		{rmiNotifyRoomInfo, roomInfo(room)},
		{rmiNotifyRoomMapInfo, roomMapInfo(room)},
		{rmiSendRoomUserList, roomUserList(room.Members())},
		{rmiNotifyTeamList, teamList(room.Members())},
		{rmiNotifyRoomLeader, appendUint32(nil, room.LeaderID)},
	}
	for _, push := range state {
		if err := session.SendPlain(Message{ID: push.id, Body: push.body}); err != nil {
			return err
		}
	}
	return session.SendPlain(Message{ID: rmiAnswerRoomCreate})
}

func (l *Lobby) joinRoom(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("room join before authentication")
	}
	if len(msg.Body) < 2 {
		l.logger.Info(fmt.Sprintf("lobby: malformed room join body=%x", msg.Body))
		return l.refuseJoin(session)
	}
	number := binary.LittleEndian.Uint16(msg.Body)
	password, _, err := ReadProudString(msg.Body, 2)
	if err != nil {
		l.logger.Info(fmt.Sprintf("lobby: malformed room join body=%x: %v", msg.Body, err))
		return l.refuseJoin(session)
	}

	room, found := l.rooms.Find(number)
	if !found {
		l.logger.Info(fmt.Sprintf("lobby: %s asked for room %d, which is not open", player.Name, number))
		return l.refuseJoin(session)
	}
	if room.Password != "" && room.Password != password {
		l.logger.Info(fmt.Sprintf("lobby: %s gave the wrong password for room %d", player.Name, number))
		return l.refuseJoin(session)
	}

	user, seated := room.AddUser(RoomUser{
		Name:     player.Name,
		Team:     room.NextTeam(),
		Level:    levelByte(player),
		SteamID:  player.SteamID,
		ClanName: player.ClanName,
	})
	if !seated {
		l.logger.Info(fmt.Sprintf("lobby: room %d is full, %s stays out", number, player.Name))
		return l.refuseJoin(session)
	}
	setSessionRoom(session, room)
	setSessionRoomUser(session, user.ID)
	l.logger.Info(fmt.Sprintf("lobby: %s joined room %d as user %d (%d/%d)", player.Name, number, user.ID, len(room.Members()), room.MaxUsers))

	for _, push := range []followUp{
		{rmiNotifyOpenMapInfo, openMapInfo(room)},
		{rmiNotifyRoomInfo, roomInfo(room)},
		{rmiNotifyRoomMapInfo, roomMapInfo(room)},
		{rmiSendRoomUserList, roomUserList(room.Members())},
		{rmiNotifyTeamList, teamList(room.Members())},
		{rmiNotifyRoomLeader, appendUint32(nil, room.LeaderID)},
	} {
		if err := session.SendPlain(Message{ID: push.id, Body: push.body}); err != nil {
			return err
		}
	}
	if err := session.SendPlain(Message{ID: rmiAnswerRoomJoinOK}); err != nil {
		return err
	}

	l.broadcastRoomExcept(room, session, Message{ID: rmiNotifyRoomUserInfo, Body: appendRoomUser(nil, user)})
	l.broadcastRoomExcept(room, session, Message{ID: rmiNotifyTeamInfo, Body: append(appendUint32(nil, user.ID), user.Team)})
	return nil
}

func (l *Lobby) refuseJoin(session *Session) error {
	return session.SendPlain(Message{ID: rmiAnswerRoomJoinFail, Body: appendUint32(nil, 0)})
}

func (l *Lobby) leaveRoom(session *Session, _ Message) error {
	if room := sessionRoom(session); room != nil {
		_, _, empty := room.RemoveUser(sessionRoomUser(session))
		if empty {
			l.rooms.Close(room.Number)
			l.logger.Info(fmt.Sprintf("lobby: room %d closed, last player left", room.Number))
		} else {
			l.broadcastRoom(room, Message{ID: rmiSendRoomUserList, Body: roomUserList(room.Members())})
		}
	}
	setSessionRoom(session, nil)
	return session.SendPlain(Message{ID: rmiAnswerRoomLeave})
}

func (l *Lobby) broadcastRoom(room *Room, msg Message) {
	l.broadcastRoomExcept(room, nil, msg)
}

func (l *Lobby) broadcastRoomExcept(room *Room, skip *Session, msg Message) {
	for _, session := range l.sessions.InRoom(room.Number) {
		if session == skip {
			continue
		}
		if err := session.SendPlain(msg); err != nil {
			l.logger.Error(fmt.Sprintf("lobby: push 0x%04x to room %d: %v", msg.ID, room.Number, err))
		}
	}
}

func (l *Lobby) ready(session *Session, _ Message) error {
	room := sessionRoom(session)
	if room == nil {
		return fmt.Errorf("ready without a room")
	}

	userID := sessionRoomUser(session)
	state, seated := room.ToggleReady(userID)
	if !seated {
		return fmt.Errorf("ready from someone not seated in room %d", room.Number)
	}

	if err := session.SendPlain(Message{ID: rmiAnswerRoomReady}); err != nil {
		return err
	}
	l.broadcastRoom(room, Message{
		ID:   rmiNotifyStateInfo,
		Body: append(appendUint32(nil, userID), state),
	})
	l.logger.Info(fmt.Sprintf("lobby: room %d user %d ready=%t (all=%t)", room.Number, userID, state == wireStateReady, room.Ready))
	l.soloStartOnReady(session, room, state) // TEMPORARY: see solo_start_hack.go
	return nil
}

func (l *Lobby) parseCreate(body []byte, player models.Player) (*Room, error) {
	title, next, err := ReadProudString(body, 0)
	if err != nil {
		return nil, fmt.Errorf("read room title: %w", err)
	}
	password, next, err := ReadProudString(body, next)
	if err != nil {
		return nil, fmt.Errorf("read room password: %w", err)
	}

	fixed := body[next:]
	if len(fixed) < 13 {
		return nil, fmt.Errorf("room create fixed block too short: %d", len(fixed))
	}
	maxUsers := fixed[2]
	if maxUsers == 0 {
		maxUsers = 16
	}

	mapIndex := uint32(binary.LittleEndian.Uint16(fixed[0:2]))
	room := &Room{
		Title:            title,
		Password:         password,
		MapIndex:         mapIndex,
		MapName:          l.catalog.Maps[mapIndex].MapName,
		MaxUsers:         maxUsers,
		DisplayModeIndex: fixed[3],
		ObjectiveLimit:   fixed[4],
		RoundLimit:       fixed[5],
	}
	room.Public = fixed[8] != 0
	room.State = fixed[9]
	room.TeamBalance = fixed[10] != 0
	room.Instruction = fixed[11] != 0
	room.DeathChat = fixed[12] != 0
	leader, seated := room.AddUser(RoomUser{
		Name: player.Name, Team: 1,
		Level: levelByte(player), SteamID: player.SteamID, ClanName: player.ClanName,
	})
	if !seated {
		return nil, fmt.Errorf("room created with no room for its own creator")
	}
	room.LeaderID = leader.ID
	return room, nil
}

func openMapInfo(room *Room) []byte {
	return append(appendUint16(nil, uint16(room.MapIndex)), 1)
}

func roomMapInfo(room *Room) []byte {
	body := appendUint16(nil, uint16(room.MapIndex))
	return append(body, room.DisplayModeIndex, room.RoundLimit, room.ObjectiveLimit)
}

func roomInfo(room *Room) []byte {
	users := room.Members()
	body := appendUint16(nil, room.Number)
	body = WriteProudString(body, room.Title)
	body = WriteProudString(body, room.Password)
	body = WriteProudString(body, room.MapName)
	body = appendUint32(body, room.LeaderID)
	body = append(body, roomMapInfo(room)...)
	return append(body,
		clampByte(len(users)), room.MaxUsers, room.RoomType,
		boolByte(room.Public), room.State,
		boolByte(room.TeamBalance), boolByte(room.Instruction),
		boolByte(room.DeathChat), boolByte(room.PerkUse), boolByte(room.ThirdViewUse),
	)
}

func roomUserList(users []RoomUser) []byte {
	body := EncodeVarInt(len(users))
	for _, user := range users {
		body = appendRoomUser(body, user)
	}
	return body
}

func appendRoomUser(dst []byte, user RoomUser) []byte {
	dst = appendUint32(dst, user.ID)
	dst = WriteProudString(dst, user.Name)
	dst = WriteProudString(dst, user.ClanName)
	dst = appendClanMark(dst, user.Clan)
	return append(dst, user.Level, user.Team, user.WireState, boolByte(user.ClanName != ""))
}

func teamList(users []RoomUser) []byte {
	body := EncodeVarInt(len(users))
	for _, user := range users {
		body = appendUint32(body, user.ID)
		body = append(body, user.Team)
	}
	return body
}
