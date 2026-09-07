package proudnet

import "fmt"

// TODO TEMPORARY no need to exmplaim
func (l *Lobby) soloStartOnReady(session *Session, room *Room, state uint8) {
	if state != wireStateReady || sessionRoomUser(session) != room.LeaderID {
		return
	}
	l.logger.Info(fmt.Sprintf("lobby: room %d leader ready, standing in for the START button", room.Number))
	if err := l.beginMatch(session, room); err != nil {
		l.logger.Error(fmt.Sprintf("lobby: solo start for room %d: %v", room.Number, err))
	}
}
