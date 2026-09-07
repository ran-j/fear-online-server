package proudnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"go-service-template/internal/models"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"
)

// The friend subsystem. Layouts recovered from the client deserializers
// FUN_100c9eb0 (Friend::Info) and FUN_100c9be0 (Friend::Request):
//
//	0xA02A RequestFriend [ProudString name] -> 0xA05F / 0xA060 [u32 reason]
//	0xA02B RequestAccept [u16 id]           -> 0xA062 [u16 id] / 0xA063 [u32]
//	0xA02C RequestReject [u16 id]           -> 0xA065 [u16 id] / 0xA066 [u32]
//	0xA02D RequestDelete [u16 id]           -> 0xA067 [u16 id] / 0xA068 [u32]
//
//	0xA05D SendFriendList        list of Friend::Info
//	0xA05E SendRequestFriendList list of Friend::Request
//	0xA061 SendRequestFriendInfo one Friend::Request
//	0xA064 SendFriendInfo        one Friend::Info
//	0xA069 SendDeleteFriend      [u16 id]
const (
	rmiFriendRequestLogin uint16 = 0xA029
	rmiFriendAnswerLogin  uint16 = 0xA05B

	rmiRequestFriend          uint16 = 0xA02A
	rmiAnswerFriendAskOK      uint16 = 0xA05F
	rmiAnswerFriendAskFail    uint16 = 0xA060
	rmiRequestFriendAccept    uint16 = 0xA02B
	rmiAnswerFriendAcceptOK   uint16 = 0xA062
	rmiAnswerFriendAcceptFail uint16 = 0xA063
	rmiRequestFriendReject    uint16 = 0xA02C
	rmiAnswerFriendRejectOK   uint16 = 0xA065
	rmiAnswerFriendRejectFail uint16 = 0xA066
	rmiRequestFriendDelete    uint16 = 0xA02D
	rmiAnswerFriendDeleteOK   uint16 = 0xA067
	rmiAnswerFriendDeleteFail uint16 = 0xA068

	rmiSendFriendList        uint16 = 0xA05D
	rmiSendRequestFriendList uint16 = 0xA05E
	rmiSendRequestFriendInfo uint16 = 0xA061
	rmiSendFriendInfo        uint16 = 0xA064
	rmiSendDeleteFriend      uint16 = 0xA069

	rmiRequestFriendInvite  uint16 = 0xA02E
	rmiAnswerFriendInviteOK uint16 = 0xA06A
	rmiAnswerFriendInviteNo uint16 = 0xA06B
	rmiSendFriendInvite     uint16 = 0xA06C
)

type Friends struct {
	friends  *services.FriendService
	clans    *services.ClanService
	logger   logger.Interface
	sessions *SessionRegistry
}

func NewFriends(friends *services.FriendService, clans *services.ClanService, logger logger.Interface) *Friends {
	return &Friends{friends: friends, clans: clans, logger: logger}
}

func (f *Friends) Register(server *Server) {
	f.sessions = server.Sessions()
	server.Handle(rmiFriendRequestLogin, f.login)
	server.Handle(rmiRequestFriend, f.request)
	server.Handle(rmiRequestFriendAccept, f.accept)
	server.Handle(rmiRequestFriendReject, f.reject)
	server.Handle(rmiRequestFriendDelete, f.delete)
	server.Handle(rmiRequestFriendInvite, f.invite)
}

// login answers the social screen's own login and hands over both lists.
func (f *Friends) login(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("friend login without an authenticated account")
	}
	if err := session.SendPlain(Message{ID: rmiFriendAnswerLogin}); err != nil {
		return err
	}

	ctx, cancel := f.context()
	defer cancel()
	current, accounts, err := f.friends.Lists(ctx, player.SteamID)
	if err != nil {
		f.logger.Error(fmt.Sprintf("friend: load lists of %s: %v", player.Name, err))
		current = player
	}

	if err := session.SendPlain(Message{ID: rmiSendFriendList, Body: f.friendList(current, accounts)}); err != nil {
		return err
	}
	return session.SendPlain(Message{ID: rmiSendRequestFriendList, Body: friendRequestList(current, accounts)})
}

func (f *Friends) request(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("friend request without an authenticated account")
	}
	name, _, err := ReadProudString(msg.Body, 0)
	if err != nil {
		f.logger.Info(fmt.Sprintf("friend: malformed request body=%x: %v", msg.Body, err))
		return f.refuse(session, rmiAnswerFriendAskFail)
	}

	ctx, cancel := f.context()
	defer cancel()
	target, request, err := f.friends.Request(ctx, player.SteamID, name)
	if err != nil {
		f.logger.Info(fmt.Sprintf("friend: %s could not add %q: %v", player.Name, name, err))
		return f.refuse(session, rmiAnswerFriendAskFail)
	}

	f.logger.Info(fmt.Sprintf("friend: %s asked %s", player.Name, target.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerFriendAskOK}); err != nil {
		return err
	}
	f.push(target.SteamID, Message{
		ID:   rmiSendRequestFriendInfo,
		Body: appendFriendRequest(nil, request, player),
	})
	return nil
}

func (f *Friends) accept(session *Session, msg Message) error {
	player, requestID, ok := f.actorAndID(session, msg)
	if !ok {
		return f.refuse(session, rmiAnswerFriendAcceptFail)
	}

	ctx, cancel := f.context()
	defer cancel()
	mine, theirs, sender, err := f.friends.Accept(ctx, player.SteamID, requestID)
	if err != nil {
		f.logger.Info(fmt.Sprintf("friend: %s could not accept %d: %v", player.Name, requestID, err))
		return f.refuse(session, rmiAnswerFriendAcceptFail)
	}

	f.logger.Info(fmt.Sprintf("friend: %s and %s are now friends", player.Name, sender.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerFriendAcceptOK, Body: appendUint16(nil, requestID)}); err != nil {
		return err
	}

	// Each side gets the row describing the other.
	if err := session.SendPlain(Message{ID: rmiSendFriendInfo, Body: f.friendRow(mine, sender)}); err != nil {
		return err
	}
	f.push(sender.SteamID, Message{ID: rmiSendFriendInfo, Body: f.friendRow(theirs, player)})
	return nil
}

func (f *Friends) reject(session *Session, msg Message) error {
	player, requestID, ok := f.actorAndID(session, msg)
	if !ok {
		return f.refuse(session, rmiAnswerFriendRejectFail)
	}

	ctx, cancel := f.context()
	defer cancel()
	if _, err := f.friends.Reject(ctx, player.SteamID, requestID); err != nil {
		f.logger.Info(fmt.Sprintf("friend: %s could not reject %d: %v", player.Name, requestID, err))
		return f.refuse(session, rmiAnswerFriendRejectFail)
	}

	f.logger.Info(fmt.Sprintf("friend: %s turned down request %d", player.Name, requestID))
	return session.SendPlain(Message{ID: rmiAnswerFriendRejectOK, Body: appendUint16(nil, requestID)})
}

func (f *Friends) delete(session *Session, msg Message) error {
	player, friendID, ok := f.actorAndID(session, msg)
	if !ok {
		return f.refuse(session, rmiAnswerFriendDeleteFail)
	}

	ctx, cancel := f.context()
	defer cancel()
	friend, mirrored, err := f.friends.Delete(ctx, player.SteamID, friendID)
	if err != nil {
		f.logger.Info(fmt.Sprintf("friend: %s could not remove %d: %v", player.Name, friendID, err))
		return f.refuse(session, rmiAnswerFriendDeleteFail)
	}

	f.logger.Info(fmt.Sprintf("friend: %s removed friend %d", player.Name, friendID))
	if err := session.SendPlain(Message{ID: rmiAnswerFriendDeleteOK, Body: appendUint16(nil, friendID)}); err != nil {
		return err
	}
	if mirrored.FriendID != 0 {
		f.push(friend.SteamID, Message{ID: rmiSendDeleteFriend, Body: appendUint16(nil, mirrored.FriendID)})
	}
	return nil
}

func (f *Friends) invite(session *Session, msg Message) error {
	player, friendID, ok := f.actorAndID(session, msg)
	if !ok {
		return f.refuse(session, rmiAnswerFriendInviteNo)
	}
	room := sessionRoom(session)
	if room == nil {
		f.logger.Info(fmt.Sprintf("friend: %s invited %d from outside a room", player.Name, friendID))
		return f.refuse(session, rmiAnswerFriendInviteNo)
	}

	ctx, cancel := f.context()
	defer cancel()
	target, inviterID, err := f.friends.InviteTarget(ctx, player.SteamID, friendID)
	if err != nil {
		f.logger.Info(fmt.Sprintf("friend: %s could not invite %d: %v", player.Name, friendID, err))
		return f.refuse(session, rmiAnswerFriendInviteNo)
	}
	if !f.sessions.IsOnline(target.SteamID) {
		f.logger.Info(fmt.Sprintf("friend: %s invited %s, who is offline", player.Name, target.Name))
		return f.refuse(session, rmiAnswerFriendInviteNo)
	}

	f.logger.Info(fmt.Sprintf("friend: %s invited %s to room %d", player.Name, target.Name, room.Number))
	if err := session.SendPlain(Message{ID: rmiAnswerFriendInviteOK, Body: appendUint16(nil, friendID)}); err != nil {
		return err
	}
	f.push(target.SteamID, Message{
		ID:   rmiSendFriendInvite,
		Body: append(appendUint16(nil, inviterID), roomInvite(room)...),
	})
	return nil
}

// roomInvite serializes Common::Standard::Room::Invite:
//
//	[u16 roomNumber][u8 roomKind][Room::MapInfo][ProudString title]
//	[u8 popupFlag][u8 tailFlag]
//
// roomKind and popupFlag both reach the invitation popup — the client derives a
// bool from `roomKind == 2` and passes popupFlag straight through — so neither
// is padding. We have no room kind of our own yet, and tailFlag never reaches
// the popup at all, so those go out as zero.
//
// TODO: roomKind should carry the room's type once rooms have one; sending zero
// pins every invitation to the `roomKind != 2` branch.
func roomInvite(room *Room) []byte {
	body := appendUint16(nil, room.Number)
	body = append(body, 0)
	body = append(body, roomMapInfo(room)...)
	body = WriteProudString(body, room.Title)
	return append(body, 1, 0)
}

// appendFriendInfo serializes Common::Standard::Friend::Info:
//
//	[u16 id][ProudString name][ProudString clanName][Clan::Mark 4 x u16]
//	[u8 isClan][u8 level][u8 channel][u8 lobby][u16 room][u8 online][u8 inGame]
//
// The byte right after the mark is isClan, not level: the tail carries one more
// field than Clan::Member does, which is why the two structs differ in shape.
func appendFriendInfo(dst []byte, friend models.Friend, account models.Player, clan models.Clan, presence Presence) []byte {
	dst = appendUint16(dst, friend.FriendID)
	dst = WriteProudString(dst, account.Name)
	dst = WriteProudString(dst, clan.Name)
	dst = appendClanMark(dst, clan)
	dst = append(dst, boolByte(clan.Name != ""), levelByte(account), presence.Channel, presence.Lobby)
	dst = appendUint16(dst, presence.Room)
	return append(dst, boolByte(presence.Online), boolByte(presence.InGame))
}

// appendFriendRequest serializes Common::Standard::Friend::Request:
// [u16 id][ProudString name][u8 level].
func appendFriendRequest(dst []byte, request models.FriendRequest, account models.Player) []byte {
	dst = appendUint16(dst, request.RequestID)
	dst = WriteProudString(dst, account.Name)
	return append(dst, levelByte(account))
}

func (f *Friends) friendList(player models.Player, accounts map[string]models.Player) []byte {
	body := EncodeVarInt(len(player.Friends))
	for _, friend := range player.Friends {
		body = append(body, f.friendRow(friend, accounts[friend.SteamID])...)
	}
	return body
}

func friendRequestList(player models.Player, accounts map[string]models.Player) []byte {
	body := EncodeVarInt(len(player.FriendRequests))
	for _, request := range player.FriendRequests {
		body = appendFriendRequest(body, request, accounts[request.SteamID])
	}
	return body
}

func (f *Friends) friendRow(friend models.Friend, account models.Player) []byte {
	var clan models.Clan
	if account.ClanName != "" && f.clans != nil {
		ctx, cancel := f.context()
		defer cancel()
		if found, ok, err := f.clans.Find(ctx, account.ClanName); err == nil && ok {
			clan = found
		}
	}
	return appendFriendInfo(nil, friend, account, clan, f.sessions.Presence(friend.SteamID))
}

func (f *Friends) actorAndID(session *Session, msg Message) (models.Player, uint16, bool) {
	player, ok := resolvePlayer(session)
	if !ok {
		return models.Player{}, 0, false
	}
	if len(msg.Body) < 2 {
		f.logger.Info(fmt.Sprintf("friend: rmi body too short for an id: %x", msg.Body))
		return models.Player{}, 0, false
	}
	return player, binary.LittleEndian.Uint16(msg.Body), true
}

// refuse answers a failure id. Its u32 is an error code whose enum is not
// mapped yet
func (f *Friends) refuse(session *Session, id uint16) error {
	return session.SendPlain(Message{ID: id, Body: appendUint32(nil, 0)})
}

func (f *Friends) push(steamID string, msg Message) {
	if err := f.sessions.SendPlain(steamID, msg); err != nil {
		f.logger.Error(fmt.Sprintf("friend: push 0x%04x to %s: %v", msg.ID, steamID, err))
	}
}

func (f *Friends) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
