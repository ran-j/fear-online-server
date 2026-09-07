package proudnet

import (
	"context"
	"sync"
	"time"
)

// SessionRegistry indexes authenticated ProudNet sessions by account ID.
// A player can have more than one live connection while moving between login,
// lobby and game flows, so each account maps to a set rather than one session.
type SessionRegistry struct {
	mu          sync.RWMutex
	bySteamID   map[string]map[*Session]struct{}
	bySessionID map[*Session]string
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		bySteamID:   make(map[string]map[*Session]struct{}),
		bySessionID: make(map[*Session]string),
	}
}

func (r *SessionRegistry) Bind(steamID string, session *Session) {
	if r == nil || steamID == "" || session == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if previous, ok := r.bySessionID[session]; ok {
		r.removeLocked(previous, session)
	}

	sessions := r.bySteamID[steamID]
	if sessions == nil {
		sessions = make(map[*Session]struct{})
		r.bySteamID[steamID] = sessions
	}
	sessions[session] = struct{}{}
	r.bySessionID[session] = steamID
}

func (r *SessionRegistry) Unbind(session *Session) {
	if r == nil || session == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	steamID, ok := r.bySessionID[session]
	if !ok {
		return
	}
	r.removeLocked(steamID, session)
}

func (r *SessionRegistry) Sessions(steamID string) []*Session {
	if r == nil || steamID == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	set := r.bySteamID[steamID]
	result := make([]*Session, 0, len(set))
	for session := range set {
		result = append(result, session)
	}
	return result
}

func (r *SessionRegistry) IsOnline(steamID string) bool {
	if r == nil || steamID == "" {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bySteamID[steamID]) != 0
}

func (r *SessionRegistry) SendPlain(steamID string, msg Message) error {
	var firstErr error
	for _, session := range r.Sessions(steamID) {
		if err := session.SendPlain(msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *SessionRegistry) removeLocked(steamID string, session *Session) {
	delete(r.bySessionID, session)
	set := r.bySteamID[steamID]
	delete(set, session)
	if len(set) == 0 {
		delete(r.bySteamID, steamID)
	}
}

type Presence struct {
	Channel uint8
	Lobby   uint8
	Room    uint16
	Online  bool
	InGame  bool
}

func (r *SessionRegistry) Presence(steamID string) Presence {
	presence := Presence{}
	for _, session := range r.Sessions(steamID) {
		presence.Online = true
		if channel := sessionChannel(session); channel != 0 {
			presence.Channel = channel
		}
		if room := sessionRoom(session); room != nil {
			presence.Room = room.Number
		}
	}
	return presence
}

func (r *SessionRegistry) Online() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.bySteamID))
	for steamID := range r.bySteamID {
		out = append(out, steamID)
	}
	return out
}

func context5s() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (r *SessionRegistry) InRoom(number uint16) []*Session {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	sessions := make([]*Session, 0, len(r.bySessionID))
	for session := range r.bySessionID {
		sessions = append(sessions, session)
	}
	r.mu.RUnlock()

	inRoom := sessions[:0]
	for _, session := range sessions {
		if room := sessionRoom(session); room != nil && room.Number == number {
			inRoom = append(inRoom, session)
		}
	}
	return inRoom
}
