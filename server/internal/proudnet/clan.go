package proudnet

import (
	"context"
	"fmt"
	"hash/crc32"
	"strings"
	"time"

	"go-service-template/internal/models"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"
)

const (
	rmiRequestSearchClanName uint16 = 0xA414
	rmiAnswerSearchNameOK    uint16 = 0xA61A
	rmiAnswerSearchNameFail  uint16 = 0xA61B

	rmiRequestCheckClanName uint16 = 0xA416
	rmiAnswerCheckNameOK    uint16 = 0xA61E
	rmiAnswerCheckNameFail  uint16 = 0xA61F

	rmiRequestCreateClan uint16 = 0xA417
	rmiAnswerCreateOK    uint16 = 0xA620
	rmiAnswerCreateFail  uint16 = 0xA621

	rmiRequestJoinClan uint16 = 0xA41E
	rmiAnswerJoinOK    uint16 = 0xA631
	rmiAnswerJoinFail  uint16 = 0xA632

	rmiRequestDeleteClan       uint16 = 0xA418
	rmiRequestChangeClanName   uint16 = 0xA419
	rmiRequestSearchClanMaster uint16 = 0xA415
	rmiRequestClanChat         uint16 = 0xA427
	rmiRequestLoginClanRecord  uint16 = 0xA428
	rmiSendClanInfo            uint16 = 0xA60B
	rmiNotifyUserClanInfo      uint16 = 0xA074
)

type Clan struct {
	clans    *services.ClanService
	players  *services.PlayerService
	logger   logger.Interface
	sessions *SessionRegistry
}

func NewClan(clans *services.ClanService, players *services.PlayerService, logger logger.Interface) *Clan {
	return &Clan{clans: clans, players: players, logger: logger}
}

func (c *Clan) Register(server *Server) {
	c.sessions = server.Sessions()
	server.Handle(rmiRequestSearchClanName, c.searchName)
	server.Handle(rmiRequestCheckClanName, c.checkName)
	server.Handle(rmiRequestCreateClan, c.create)
	server.Handle(rmiRequestJoinClan, c.requestJoin)
	server.Handle(rmiRequestClanChat, c.chat)
	c.registerActions(server)
	c.registerMark(server)

	// Still unmapped. The general chat block (0x9471-0x9474) is logged rather
	// than answered so the next message reveals its body.
	for _, id := range []uint16{
		rmiRequestSearchClanMaster,
		rmiRequestChangeClanName,
		rmiRequestLoginClanRecord,
		rmiRequestChatNotice,
	} {
		id := id
		server.Handle(id, func(session *Session, msg Message) error {
			c.logger.Info(fmt.Sprintf("clan: rmi 0x%04x body=%x len=%d", id, msg.Body, len(msg.Body)))
			return nil
		})
	}
}

func (c *Clan) create(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("create clan before authentication")
	}
	mark, markExtra, ok := parseClanMark(msg.Body)
	if !ok {
		return session.SendPlain(Message{ID: rmiAnswerCreateFail})
	}

	name, _, err := ReadProudString(msg.Body, clanMarkSize)
	if err != nil {
		return fmt.Errorf("read clan name: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clan, err := c.clans.Create(ctx, player.SteamID, player.Name, name, mark, markExtra)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: create rejected %q: %v", name, err))
		return session.SendPlain(Message{ID: rmiAnswerCreateFail})
	}
	if joined, ok := resolvePlayer(session); ok {
		joined.ClanName = name
		updatePlayer(session, joined)
	}

	c.logger.Info("clan: created " + name + " master=" + player.Name)
	if err := session.SendPlain(Message{ID: rmiAnswerCreateOK}); err != nil {
		return err
	}

	if err := session.SendPlain(Message{ID: rmiSendClanInfo, Body: appendClanInfo(nil, clan)}); err != nil {
		return err
	}
	master := makeClanMemberWire(clan.Members[0], player, clanMemberPresence{Online: true})
	if err := sendClanMemberList(session, []clanMemberWire{master}); err != nil {
		return err
	}
	if err := notifyClanMemberCount(session, clan); err != nil {
		return err
	}
	return sendUserClanInfo(session, clan)
}

func (c *Clan) requestJoin(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("join clan before authentication")
	}
	name, _, err := ReadProudString(msg.Body, 0)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: malformed join body=%x: %v", msg.Body, err))
		return session.SendPlain(Message{ID: rmiAnswerJoinFail})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clan, err := c.clans.RequestJoin(ctx, player, strings.TrimSpace(name))
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: join %q rejected for %s: %v", name, player.Name, err))
		return session.SendPlain(Message{ID: rmiAnswerJoinFail})
	}

	c.logger.Info(fmt.Sprintf("clan: %s applied to %q (%d pending)", player.Name, clan.Name, len(clan.Applicants)))
	if err := session.SendPlain(Message{ID: rmiAnswerJoinOK}); err != nil {
		return err
	}
	if err := sendRequestClanInfo(session, clan); err != nil {
		return err
	}
	c.notifyApplication(clan, player)
	return nil
}

func (c *Clan) notifyApplication(clan models.Clan, applicant models.Player) {
	pending, ok := lastApplicant(clan, applicant.SteamID)
	if !ok {
		return
	}
	for _, member := range clan.Members {
		if !models.CanReviewClanApplications(member.Rank) {
			continue
		}
		for _, target := range c.sessions.Sessions(member.SteamID) {
			if err := notifyClanRequest(target, pending, applicant); err != nil {
				c.logger.Error(fmt.Sprintf("clan: notify %s of %s's application: %v", member.Name, applicant.Name, err))
			}
		}
	}
}

func lastApplicant(clan models.Clan, steamID string) (models.ClanApplicant, bool) {
	for _, applicant := range clan.Applicants {
		if applicant.SteamID == steamID {
			return applicant, true
		}
	}
	return models.ClanApplicant{}, false
}

func (c *Clan) searchName(session *Session, msg Message) error {
	name, _, err := ReadProudString(msg.Body, 0)
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: malformed search body=%x: %v", msg.Body, err))
		return session.SendPlain(Message{ID: rmiAnswerSearchNameFail})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clans, err := c.clans.Search(ctx, strings.TrimSpace(name))
	if err != nil {
		c.logger.Info(fmt.Sprintf("clan: search %q failed: %v", name, err))
		return session.SendPlain(Message{ID: rmiAnswerSearchNameFail})
	}

	c.logger.Info(fmt.Sprintf("clan: search %q returned %d result(s)", name, len(clans)))
	return session.SendPlain(Message{ID: rmiAnswerSearchNameOK, Body: clanInfoList(clans)})
}

func (c *Clan) checkName(session *Session, msg Message) error {
	name, _, err := ReadProudString(msg.Body, 0)
	if err != nil {
		return fmt.Errorf("read clan name: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	available, err := c.clans.NameAvailable(ctx, name)
	if err != nil {
		return fmt.Errorf("check clan name %q: %w", name, err)
	}

	if !available {
		c.logger.Info("clan: name taken " + name)
		return session.SendPlain(Message{ID: rmiAnswerCheckNameFail})
	}
	c.logger.Info("clan: name available " + name)
	return session.SendPlain(Message{ID: rmiAnswerCheckNameOK})
}

// notifyUserClanInfoRow builds NotifyUserClanInfo (0xA074):
// [u16 user index][Proud string clan name][4 x u16 Clan::Mark].
func notifyUserClanInfoRow(userIndex uint16, clan models.Clan) []byte {
	body := appendUint16(nil, userIndex)
	body = WriteProudString(body, clan.Name)
	return appendClanMark(body, clan)
}

func sendUserClanInfo(session *Session, clan models.Clan) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("clan info for an unauthenticated connection")
	}
	return session.SendPlain(Message{
		ID:   rmiNotifyUserClanInfo,
		Body: notifyUserClanInfoRow(uint16(characterID(player.SteamID)), clan),
	})
}

func appendClanMark(dst []byte, clan models.Clan) []byte {
	for _, value := range clan.Mark {
		if value < 0 {
			value = 0
		}
		if value > 0xFFFF {
			value = 0xFFFF
		}
		dst = appendUint16(dst, uint16(value))
	}
	return appendUint16(dst, clan.MarkExtra)
}

func clanInfoList(clans []models.Clan) []byte {
	body := EncodeVarInt(len(clans))
	for _, clan := range clans {
		body = appendClanInfo(body, clan)
	}
	return body
}

func appendClanInfo(dst []byte, clan models.Clan) []byte {
	dst = appendUint32(dst, clanWireID(clan.Name))
	dst = WriteProudString(dst, clan.Name)
	dst = WriteProudString(dst, clanMasterName(clan))

	dst = append(dst, 0, clanLevel(clan))

	// 64-bit clan score/experience aggregate.
	dst = appendUint64(dst, 0)

	// Search-page record fields.
	for _, value := range []int32{
		clan.Records.Win,
		clan.Records.Draw,
		clan.Records.Lose,
		clan.Records.Kill,
		clan.Records.Death,
		0,
	} {
		dst = appendUint32(dst, nonNegativeUint32(value))
	}

	// Rank/presentation byte followed by Clan::Mark.
	dst = append(dst, 0)
	dst = appendClanMark(dst, clan)

	dst = append(dst, 0)
	dst = append(dst, makeClanMemberCounts(clan).row()...)

	// Notice and introduction strings.
	dst = WriteProudString(dst, "")
	dst = WriteProudString(dst, "")

	created := clan.CreatedAt.UTC()
	if created.IsZero() {
		created = time.Unix(0, 0).UTC()
	}
	dst = appendUint16(dst, uint16(created.Year()))
	dst = append(dst, byte(created.Month()), byte(created.Day()))
	return dst
}

func clanLevel(clan models.Clan) byte {
	if clan.Level == 0 {
		return 1
	}
	return clan.Level
}

func clanWireID(name string) uint32 {
	id := crc32.ChecksumIEEE([]byte(strings.ToLower(strings.TrimSpace(name))))
	if id == 0 {
		return 1
	}
	return id
}

func clanMasterName(clan models.Clan) string {
	for _, member := range clan.Members {
		if member.SteamID == clan.Master && member.Name != "" {
			return member.Name
		}
	}
	for _, member := range clan.Members {
		if member.Rank == 0 && member.Name != "" {
			return member.Name
		}
	}
	return clan.Master
}

func clampByte(value int) byte {
	if value < 0 {
		return 0
	}
	if value > 0xFF {
		return 0xFF
	}
	return byte(value)
}

func nonNegativeUint32(value int32) uint32 {
	if value < 0 {
		return 0
	}
	return uint32(value)
}
