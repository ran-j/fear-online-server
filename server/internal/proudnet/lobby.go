package proudnet

import (
	"fmt"

	"go-service-template/internal/catalog"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"
)

const (
	rmiRequestLobbyJoin  uint16 = 0x8AAD
	rmiAnswerLobbyJoin   uint16 = 0x8B11
	rmiNotifyOpenMapList uint16 = 0x8D2B

	rmiRequestLobbyLeave uint16 = 0x8AAE
	rmiAnswerLobbyLeave  uint16 = 0x8B13

	rmiRequestLobbyList uint16 = 0x8AAF
	rmiAnswerLobbyList  uint16 = 0x8B15
	rmiSendLobbyList    uint16 = 0x8B17

	rmiRequestRoomJoin uint16 = 0x8CA2

	rmiRequestRoomList        uint16 = 0x8AB0
	rmiRequestRefreshRoomList uint16 = 0x8AB3
	rmiAnswerRoomList         uint16 = 0x8B18
	rmiSendRoomList           uint16 = 0x8B1A

	rmiRequestClanRoomList uint16 = 0x8AB1
	rmiAnswerClanRoomList  uint16 = 0x8B22
	rmiSendClanRoomList    uint16 = 0x8B24

	rmiRequestLobbyUserList uint16 = 0x8AB2
	rmiAnswerLobbyUserList  uint16 = 0x8B28
	rmiSendLobbyUserList    uint16 = 0x8B2A
)

type Lobby struct {
	logger   logger.Interface
	catalog  *catalog.Catalog
	players  *services.PlayerService
	rooms    *RoomRegistry
	sessions *SessionRegistry
	gameHost string
	gamePort uint16
	tickets  *matchTickets
	matches  *matchSessions
}

func NewLobby(logger logger.Interface, gameCatalog *catalog.Catalog, players *services.PlayerService, rooms *RoomRegistry) *Lobby {
	return &Lobby{logger: logger, catalog: gameCatalog, players: players, rooms: rooms, tickets: newMatchTickets(), matches: newMatchSessions()}
}

func (l *Lobby) Register(server *Server) {
	l.sessions = server.Sessions()
	l.gameHost, l.gamePort = server.host, server.port
	server.Handle(rmiRequestLobbyJoin, l.join)
	server.Handle(rmiRequestLobbyLeave, l.leave)
	server.Handle(rmiRequestLobbyList, l.lobbyList)
	server.Handle(rmiRequestRoomList, l.roomList)
	server.Handle(rmiRequestClanRoomList, l.clanRoomList)
	server.Handle(rmiRequestLobbyUserList, l.userList)
	server.Handle(rmiRequestRoomCreate, l.createRoom)
	server.Handle(rmiRequestRoomLeave, l.leaveRoom)
	server.Handle(rmiRequestRoomReady, l.ready)

	server.Handle(rmiRequestRoomJoin, l.joinRoom)
	server.Handle(rmiRequestGameStart, l.gameStart)
	server.Handle(rmiAnswerConnectServer, l.connectServer)
	server.Handle(rmiRequestMatchStart, l.matchStart)
	server.Handle(rmiRequestLoadPercent, l.loadPercent)
	server.Handle(rmiRequestLoadComplete, l.loadComplete)
	server.Handle(rmiRequestTimeStart, l.timeStart)
	server.Handle(rmiRequestBeginRound, l.beginRound)
	server.Handle(rmiRequestCheckRespawn, l.checkRespawn)
	server.Handle(rmiRequestRespawnInstant, l.checkRespawnInstant)
	server.Handle(rmiRequestRespawn, l.respawn)
	server.Handle(rmiRequestDeath, l.death)
	server.Handle(rmiRequestTimeOut, l.timeOut)
	server.Handle(rmiRequestDediUserPing, l.dediUserPing)
	server.Handle(rmiRequestHitCount, l.hitCount)
	server.Handle(rmiRequestRoomGameLeave, l.matchLeave)
	server.Handle(rmiRequestGameLeave, l.matchLeave)
	server.Handle(rmiRequestUserEquip, l.userEquip)
	server.Handle(rmiRequestChangeWeapon, l.changeWeapon)
	server.Handle(rmiRequestRefreshRoomList, l.roomList)
}

func (l *Lobby) join(session *Session, msg Message) error {
	l.logger.Info(fmt.Sprintf("lobby: join body=%x", msg.Body))
	if err := session.SendPlain(Message{ID: rmiAnswerLobbyJoin, Body: []byte{1}}); err != nil {
		return err
	}
	return session.SendPlain(Message{ID: rmiNotifyOpenMapList, Body: l.openMapList()})
}

func (l *Lobby) leave(session *Session, _ Message) error {
	return session.SendPlain(Message{ID: rmiAnswerLobbyLeave})
}

func (l *Lobby) lobbyList(session *Session, _ Message) error {
	if err := session.SendPlain(Message{ID: rmiAnswerLobbyList}); err != nil {
		return err
	}
	return session.SendPlain(Message{ID: rmiSendLobbyList, Body: empty})
}

func (l *Lobby) roomList(session *Session, _ Message) error {
	if err := session.SendPlain(Message{ID: rmiAnswerRoomList}); err != nil {
		return err
	}

	rooms := l.rooms.List()
	body := EncodeVarInt(len(rooms))
	for _, room := range rooms {
		body = append(body, roomInfo(room)...)
	}
	l.logger.Info(fmt.Sprintf("lobby: advertising %d open room(s)", len(rooms)))
	return session.SendPlain(Message{ID: rmiSendRoomList, Body: body})
}

func (l *Lobby) clanRoomList(session *Session, _ Message) error {
	if err := session.SendPlain(Message{ID: rmiAnswerClanRoomList}); err != nil {
		return err
	}
	return session.SendPlain(Message{ID: rmiSendClanRoomList, Body: empty})
}

func (l *Lobby) userList(session *Session, _ Message) error {
	if err := session.SendPlain(Message{ID: rmiAnswerLobbyUserList}); err != nil {
		return err
	}
	return session.SendPlain(Message{ID: rmiSendLobbyUserList, Body: empty})
}

func (l *Lobby) openMapList() []byte {
	maps := l.catalog.OpenMaps()
	l.logger.Info(fmt.Sprintf("lobby: advertising %d open map(s)", len(maps)))

	body := EncodeVarInt(len(maps))
	for _, info := range maps {
		body = appendUint16(body, uint16(info.MapIndex))
		body = append(body, 1) // isOpen
	}
	return body
}
