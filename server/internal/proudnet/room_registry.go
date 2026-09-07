package proudnet

import (
	"sort"
	"sync"
)

// RoomRegistry is the server's list of live rooms. Rooms used to exist only
// inside the session that created them, which made a room unreachable to
// anybody else — an invitation could name a room number nothing could resolve.
type RoomRegistry struct {
	mu       sync.RWMutex
	rooms    map[uint16]*Room
	lastRoom uint16
}

func NewRoomRegistry() *RoomRegistry {
	return &RoomRegistry{rooms: make(map[uint16]*Room)}
}

// Open assigns the next free number and publishes the room.
func (r *RoomRegistry) Open(room *Room) uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()

	for {
		r.lastRoom++
		if r.lastRoom == 0 {
			r.lastRoom = 1
		}
		if _, taken := r.rooms[r.lastRoom]; !taken {
			break
		}
	}
	room.Number = r.lastRoom
	r.rooms[room.Number] = room
	return room.Number
}

func (r *RoomRegistry) Find(number uint16) (*Room, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.rooms[number]
	return room, ok
}

func (r *RoomRegistry) Close(number uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rooms, number)
}

// List returns the open rooms in number order, so the lobby listing is stable
// between refreshes rather than following Go's map iteration.
func (r *RoomRegistry) List() []*Room {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rooms := make([]*Room, 0, len(r.rooms))
	for _, room := range r.rooms {
		rooms = append(rooms, room)
	}
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].Number < rooms[j].Number })
	return rooms
}
