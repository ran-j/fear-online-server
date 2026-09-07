package proudnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"go-service-template/internal/models"
)

// Membership management. Every request carries at most a member or applicant id
// and a position byte; the answers are bare acks, and the state actually lands
// on the other clients through the notifications below.
//
//	0xA41F RequestJoinCancel      (no body)                  -> 0xA633 / 0xA634
//	0xA420 RequestAcceptMember    [u16 applicantId]          -> 0xA635 / 0xA636
//	0xA421 RequestRejectMember    [u16 applicantId]          -> 0xA637 / 0xA638
//	0xA422 RequestKickOutMember   [u16 memberId]             -> 0xA639 / 0xA63A
//	0xA423 RequestGetOut          (no body)                  -> 0xA63B / 0xA63C
//	0xA424 RequestChangeMaster    [u16 memberId]             -> 0xA63D / 0xA63E
//	0xA425 RequestChangePosition  [u16 memberId][u8 position]-> 0xA63F / 0xA640
const (
	rmiRequestJoinCancel     uint16 = 0xA41F
	rmiAnswerJoinCancelOK    uint16 = 0xA633
	rmiAnswerJoinCancelFail  uint16 = 0xA634
	rmiRequestAcceptMember   uint16 = 0xA420
	rmiAnswerAcceptMemberOK  uint16 = 0xA635
	rmiAnswerAcceptFail      uint16 = 0xA636
	rmiRequestRejectMember   uint16 = 0xA421
	rmiAnswerRejectMemberOK  uint16 = 0xA637
	rmiAnswerRejectFail      uint16 = 0xA638
	rmiRequestKickOutMember  uint16 = 0xA422
	rmiAnswerKickOutOK       uint16 = 0xA639
	rmiAnswerKickOutFail     uint16 = 0xA63A
	rmiRequestGetOut         uint16 = 0xA423
	rmiAnswerGetOutOK        uint16 = 0xA63B
	rmiAnswerGetOutFail      uint16 = 0xA63C
	rmiRequestChangeMaster   uint16 = 0xA424
	rmiAnswerChangeMasterOK  uint16 = 0xA63D
	rmiAnswerChangeMasterNo  uint16 = 0xA63E
	rmiRequestChangePosition uint16 = 0xA425
	rmiAnswerChangePosOK     uint16 = 0xA63F
	rmiAnswerChangePosFail   uint16 = 0xA640

	// 0xA418 RequestDelete also arrives with no body.
	rmiAnswerDeleteOK   uint16 = 0xA622
	rmiAnswerDeleteFail uint16 = 0xA623

	rmiSendClearClanInfo    uint16 = 0xA60C
	rmiNotifyDeleteMember   uint16 = 0xA614
	rmiSendClearMember      uint16 = 0xA615
	rmiNotifyMemberPosition uint16 = 0xA641
)

func (c *Clan) registerActions(server *Server) {
	server.Handle(rmiRequestDeleteClan, c.deleteClan)
	server.Handle(rmiRequestJoinCancel, c.cancelApplication)
	server.Handle(rmiRequestAcceptMember, c.acceptMember)
	server.Handle(rmiRequestRejectMember, c.rejectMember)
	server.Handle(rmiRequestKickOutMember, c.kickMember)
	server.Handle(rmiRequestGetOut, c.leaveClan)
	server.Handle(rmiRequestChangeMaster, c.changeMaster)
	server.Handle(rmiRequestChangePosition, c.changePosition)
}

func (c *Clan) deleteClan(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("delete clan before authentication")
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, err := c.clans.Disband(ctx, player)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not delete %q: %v", player.Name, player.ClanName, err))
		return session.SendPlain(Message{ID: rmiAnswerDeleteFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s deleted %q (%d member(s), %d pending)", player.Name, clan.Name, len(clan.Members), len(clan.Applicants)))
	if err := session.SendPlain(Message{ID: rmiAnswerDeleteOK}); err != nil {
		return err
	}

	for _, member := range clan.Members {
		c.setSessionClan(member.SteamID, "")
	}
	c.broadcast(clan,
		Message{ID: rmiSendClearMember},
		Message{ID: rmiSendClearClanInfo},
	)
	// Anyone still queued is waiting on a clan that no longer exists.
	for _, applicant := range clan.Applicants {
		c.sendTo(applicant.SteamID, Message{ID: rmiSendClearRequestInfo})
	}
	return nil
}

func (c *Clan) acceptMember(session *Session, msg Message) error {
	player, applicantID, ok := c.actorAndID(session, msg)
	if !ok {
		return session.SendPlain(Message{ID: rmiAnswerAcceptFail})
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, member, err := c.clans.AcceptApplication(ctx, player, applicantID)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not accept applicant %d: %v", player.Name, applicantID, err))
		return session.SendPlain(Message{ID: rmiAnswerAcceptFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s accepted %s into %q", player.Name, member.Name, clan.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerAcceptMemberOK}); err != nil {
		return err
	}

	c.broadcastReviewers(clan, Message{ID: rmiNotifyDeleteRequest, Body: appendUint16(nil, applicantID)})
	c.broadcast(clan, Message{ID: rmiNotifyMemberInfo, Body: appendClanMember(nil, c.memberWire(clan, member))})
	c.broadcast(clan, Message{ID: rmiNotifyMemberCount, Body: makeClanMemberCounts(clan).row()})
	c.welcome(clan, member)
	return nil
}

func (c *Clan) rejectMember(session *Session, msg Message) error {
	player, applicantID, ok := c.actorAndID(session, msg)
	if !ok {
		return session.SendPlain(Message{ID: rmiAnswerRejectFail})
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, applicant, err := c.clans.RejectApplication(ctx, player, applicantID)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not reject applicant %d: %v", player.Name, applicantID, err))
		return session.SendPlain(Message{ID: rmiAnswerRejectFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s rejected %s from %q", player.Name, applicant.Name, clan.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerRejectMemberOK}); err != nil {
		return err
	}

	c.broadcastReviewers(clan, Message{ID: rmiNotifyDeleteRequest, Body: appendUint16(nil, applicantID)})
	c.sendTo(applicant.SteamID, Message{ID: rmiSendClearRequestInfo})
	return nil
}

func (c *Clan) cancelApplication(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("cancel clan application before authentication")
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, applicant, err := c.clans.CancelApplication(ctx, player)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not cancel their application: %v", player.Name, err))
		return session.SendPlain(Message{ID: rmiAnswerJoinCancelFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s withdrew their application to %q", player.Name, clan.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerJoinCancelOK}); err != nil {
		return err
	}
	if err := session.SendPlain(Message{ID: rmiSendClearRequestInfo}); err != nil {
		return err
	}

	c.broadcastReviewers(clan, Message{ID: rmiNotifyDeleteRequest, Body: appendUint16(nil, applicant.ApplicantID)})
	return nil
}

func (c *Clan) kickMember(session *Session, msg Message) error {
	player, memberID, ok := c.actorAndID(session, msg)
	if !ok {
		return session.SendPlain(Message{ID: rmiAnswerKickOutFail})
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, member, err := c.clans.KickMember(ctx, player, memberID)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not kick member %d: %v", player.Name, memberID, err))
		return session.SendPlain(Message{ID: rmiAnswerKickOutFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s kicked %s out of %q", player.Name, member.Name, clan.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerKickOutOK}); err != nil {
		return err
	}

	c.announceDeparture(clan, member)
	return nil
}

func (c *Clan) leaveClan(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("leave clan before authentication")
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, member, err := c.clans.Leave(ctx, player)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not leave: %v", player.Name, err))
		return session.SendPlain(Message{ID: rmiAnswerGetOutFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s left %q", player.Name, clan.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerGetOutOK}); err != nil {
		return err
	}

	c.announceDeparture(clan, member)
	return nil
}

func (c *Clan) changePosition(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("change clan position before authentication")
	}
	if len(msg.Body) < 3 {
		c.logger.Info(fmt.Sprintf("clan: malformed change position body=%x", msg.Body))
		return session.SendPlain(Message{ID: rmiAnswerChangePosFail})
	}
	memberID := binary.LittleEndian.Uint16(msg.Body)
	rank := msg.Body[2]

	ctx, cancel := c.context()
	defer cancel()
	clan, member, err := c.clans.ChangePosition(ctx, player, memberID, rank)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not set member %d to position %d: %v", player.Name, memberID, rank, err))
		return session.SendPlain(Message{ID: rmiAnswerChangePosFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s set %s to position %d in %q", player.Name, member.Name, rank, clan.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerChangePosOK}); err != nil {
		return err
	}

	c.broadcast(clan, Message{ID: rmiNotifyMemberPosition, Body: memberPositionRow(member)})
	c.broadcast(clan, Message{ID: rmiNotifyMemberCount, Body: makeClanMemberCounts(clan).row()})
	return nil
}

func (c *Clan) changeMaster(session *Session, msg Message) error {
	player, memberID, ok := c.actorAndID(session, msg)
	if !ok {
		return session.SendPlain(Message{ID: rmiAnswerChangeMasterNo})
	}

	ctx, cancel := c.context()
	defer cancel()
	clan, successor, previous, err := c.clans.ChangeMaster(ctx, player, memberID)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: %s could not hand %q to member %d: %v", player.Name, player.ClanName, memberID, err))
		return session.SendPlain(Message{ID: rmiAnswerChangeMasterNo})
	}

	c.logger.Info(fmt.Sprintf("clan: %s handed %q over to %s", player.Name, clan.Name, successor.Name))
	if err := session.SendPlain(Message{ID: rmiAnswerChangeMasterOK}); err != nil {
		return err
	}

	c.broadcast(clan, Message{ID: rmiNotifyMemberPosition, Body: memberPositionRow(successor)})
	c.broadcast(clan, Message{ID: rmiNotifyMemberPosition, Body: memberPositionRow(previous)})
	c.broadcast(clan, Message{ID: rmiNotifyMemberCount, Body: makeClanMemberCounts(clan).row()})
	return nil
}

func (c *Clan) welcome(clan models.Clan, member models.ClanMember) {
	c.setSessionClan(member.SteamID, clan.Name)
	c.sendTo(member.SteamID,
		Message{ID: rmiSendClearRequestInfo},
		Message{ID: rmiSendClanInfo, Body: appendClanInfo(nil, clan)},
		Message{ID: rmiSendMemberList, Body: clanMemberList(c.roster(clan))},
		Message{ID: rmiNotifyMemberCount, Body: makeClanMemberCounts(clan).row()},
		Message{ID: rmiNotifyUserClanInfo, Body: notifyUserClanInfoRow(uint16(characterID(member.SteamID)), clan)},
	)
}

func (c *Clan) announceDeparture(clan models.Clan, member models.ClanMember) {
	c.broadcast(clan, Message{ID: rmiNotifyDeleteMember, Body: appendUint16(nil, member.MemberID)})
	c.broadcast(clan, Message{ID: rmiNotifyMemberCount, Body: makeClanMemberCounts(clan).row()})

	c.setSessionClan(member.SteamID, "")
	c.sendTo(member.SteamID,
		Message{ID: rmiSendClearMember},
		Message{ID: rmiSendClearClanInfo},
	)
}

func memberPositionRow(member models.ClanMember) []byte {
	body := appendUint16(nil, member.MemberID)
	return appendUint32(body, uint32(member.Rank))
}

func (c *Clan) actorAndID(session *Session, msg Message) (models.Player, uint16, bool) {
	player, ok := resolvePlayer(session)
	if !ok {
		return models.Player{}, 0, false
	}
	if len(msg.Body) < 2 {
		c.logger.Info(fmt.Sprintf("clan: rmi body too short for an id: %x", msg.Body))
		return models.Player{}, 0, false
	}
	return player, binary.LittleEndian.Uint16(msg.Body), true
}

func (c *Clan) broadcast(clan models.Clan, msgs ...Message) {
	for _, member := range clan.Members {
		c.sendTo(member.SteamID, msgs...)
	}
}

func (c *Clan) broadcastReviewers(clan models.Clan, msgs ...Message) {
	for _, member := range clan.Members {
		if models.CanReviewClanApplications(member.Rank) {
			c.sendTo(member.SteamID, msgs...)
		}
	}
}

func (c *Clan) sendTo(steamID string, msgs ...Message) {
	for _, session := range c.sessions.Sessions(steamID) {
		for _, msg := range msgs {
			if err := session.SendPlain(msg); err != nil {
				c.logger.Error(fmt.Sprintf("clan: push 0x%04x to %s: %v", msg.ID, steamID, err))
				break
			}
		}
	}
}

func (c *Clan) setSessionClan(steamID, clanName string) {
	for _, session := range c.sessions.Sessions(steamID) {
		if player, ok := resolvePlayer(session); ok {
			player.ClanName = clanName
			updatePlayer(session, player)
		}
	}
}

func (c *Clan) roster(clan models.Clan) []clanMemberWire {
	steamIDs := make([]string, 0, len(clan.Members))
	for _, member := range clan.Members {
		steamIDs = append(steamIDs, member.SteamID)
	}

	ctx, cancel := c.context()
	defer cancel()
	accounts, err := c.players.FindMany(ctx, steamIDs)
	if err != nil {
		c.logger.Error(fmt.Sprintf("clan: load roster of %q: %v", clan.Name, err))
		accounts = nil
	}

	rows := make([]clanMemberWire, 0, len(clan.Members))
	for _, member := range clan.Members {
		presence := clanMemberPresence{Online: c.sessions.IsOnline(member.SteamID)}
		rows = append(rows, makeClanMemberWire(member, accounts[member.SteamID], presence))
	}
	return rows
}

func (c *Clan) memberWire(clan models.Clan, member models.ClanMember) clanMemberWire {
	ctx, cancel := c.context()
	defer cancel()
	accounts, err := c.players.FindMany(ctx, []string{member.SteamID})
	if err != nil {
		c.logger.Error(fmt.Sprintf("clan: load %s of %q: %v", member.Name, clan.Name, err))
	}
	presence := clanMemberPresence{Online: c.sessions.IsOnline(member.SteamID)}
	return makeClanMemberWire(member, accounts[member.SteamID], presence)
}

func (c *Clan) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
