package proudnet

import (
	"encoding/hex"
	"sync"

	"go-service-template/internal/models"
)

type sessionData struct {
	mu       sync.RWMutex
	player   models.Player
	room     *Room
	roomUser uint32
	channel  uint8
}

func bindPlayer(session *Session, player models.Player) {
	session.State = &sessionData{player: player}
	if session.server != nil {
		session.server.sessions.Bind(player.SteamID, session)
	}
}

func sessionOf(session *Session) (*sessionData, bool) {
	data, ok := session.State.(*sessionData)
	return data, ok
}

func resolvePlayer(session *Session) (models.Player, bool) {
	data, ok := sessionOf(session)
	if !ok {
		return models.Player{}, false
	}
	data.mu.RLock()
	defer data.mu.RUnlock()
	return data.player, true
}

func updatePlayer(session *Session, player models.Player) {
	data, ok := sessionOf(session)
	if !ok {
		return
	}
	data.mu.Lock()
	defer data.mu.Unlock()
	data.player = player
}

func sessionRoom(session *Session) *Room {
	data, ok := sessionOf(session)
	if !ok {
		return nil
	}
	data.mu.RLock()
	defer data.mu.RUnlock()
	return data.room
}

func sessionChannel(session *Session) uint8 {
	data, ok := sessionOf(session)
	if !ok {
		return 0
	}
	data.mu.RLock()
	defer data.mu.RUnlock()
	return data.channel
}

func setSessionChannel(session *Session, channel uint8) {
	data, ok := sessionOf(session)
	if !ok {
		return
	}
	data.mu.Lock()
	defer data.mu.Unlock()
	data.channel = channel
}

func sessionRoomUser(session *Session) uint32 {
	state, ok := session.State.(*sessionData)
	if !ok {
		return 0
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.roomUser
}

func setSessionRoomUser(session *Session, id uint32) {
	state, ok := session.State.(*sessionData)
	if !ok {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.roomUser = id
}

func setSessionRoom(session *Session, room *Room) {
	data, ok := sessionOf(session)
	if !ok {
		return
	}
	data.mu.Lock()
	defer data.mu.Unlock()
	data.room = room
}

type Credentials struct {
	mu      sync.Mutex
	byToken map[string]models.Player
}

func NewCredentials() *Credentials {
	return &Credentials{byToken: make(map[string]models.Player)}
}

// Issue binds a token (the login connection's 16-byte guid) to an account.
func (c *Credentials) Issue(token []byte, player models.Player) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byToken[hex.EncodeToString(token)] = player
}

func (c *Credentials) Redeem(token []byte) (models.Player, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	player, ok := c.byToken[hex.EncodeToString(token)]
	return player, ok
}
