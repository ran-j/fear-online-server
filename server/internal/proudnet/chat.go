package proudnet

import (
	"fmt"
	"strings"

	"go-service-template/internal/models"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"
)

// The general chat block. Every request ends with a bool the client reads from a
// UI toggle; the notify carries the speaker plus a type byte.
//
//	0x9471 RequestAll    [ProudString text][u8 flag]            -> 0x94DA / 0x94DB [u32]
//	0x9472 RequestClan   [ProudString text][u8 flag]            -> 0x94DC / 0x94DD [u32]
//	0x9473 RequestUser   [ProudString to][ProudString text][u8]  -> 0x94DE / 0x94DF [u32]
//	0x94D5 NotifyChat    [ProudString speaker][ProudString text][u8 type][u8 flag]
const (
	rmiRequestChatClan uint16 = 0x9472

	rmiAnswerChatAllOK    uint16 = 0x94DA
	rmiAnswerChatAllFail  uint16 = 0x94DB
	rmiAnswerChatClanOK   uint16 = 0x94DC
	rmiAnswerChatClanFail uint16 = 0x94DD
	rmiAnswerChatUserOK   uint16 = 0x94DE
	rmiAnswerChatUserFail uint16 = 0x94DF

	rmiNotifyChat uint16 = 0x94D5

	// chatTypeGM is the only value read straight out of the client, used by the
	// notice flow. The rest of the enum is unmapped, so ordinary lines go out as
	// zero rather than a guessed category.
	chatTypeDefault byte = 0
	chatTypeGM      byte = 6
)

type Chat struct {
	players  *services.PlayerService
	clans    *services.ClanService
	logger   logger.Interface
	sessions *SessionRegistry
}

func NewChat(players *services.PlayerService, clans *services.ClanService, logger logger.Interface) *Chat {
	return &Chat{players: players, clans: clans, logger: logger}
}

func (c *Chat) Register(server *Server) {
	c.sessions = server.Sessions()
	server.Handle(rmiRequestChatAll, c.all)
	server.Handle(rmiRequestChatClan, c.clan)
	server.Handle(rmiRequestChatUser, c.whisper)
}

// notifyChatRow builds Common::Standard::Chat::Info.
func notifyChatRow(speaker, text string, chatType, flag byte) []byte {
	body := WriteProudString(nil, speaker)
	body = WriteProudString(body, text)
	return append(body, chatType, flag)
}

// all broadcasts to everyone connected. Scope is the whole server for now
func (c *Chat) all(session *Session, msg Message) error {
	player, text, flag, ok := c.parse(session, msg, 0)
	if !ok {
		return c.refuse(session, rmiAnswerChatAllFail)
	}

	if err := session.SendPlain(Message{ID: rmiAnswerChatAllOK}); err != nil {
		return err
	}
	c.logger.Info(fmt.Sprintf("chat [all] %s: %s", player.Name, text))

	row := notifyChatRow(player.Name, text, chatTypeDefault, flag)
	for _, steamID := range c.sessions.Online() {
		c.push(steamID, Message{ID: rmiNotifyChat, Body: row})
	}
	return nil
}

// clan is the chat block's own route into clan chat, separate from 0xA427.
func (c *Chat) clan(session *Session, msg Message) error {
	player, text, flag, ok := c.parse(session, msg, 0)
	if !ok {
		return c.refuse(session, rmiAnswerChatClanFail)
	}
	if player.ClanName == "" {
		return c.refuse(session, rmiAnswerChatClanFail)
	}

	ctx, cancel := context5s()
	defer cancel()
	clan, found, err := c.clans.Find(ctx, player.ClanName)
	if err != nil || !found {
		return c.refuse(session, rmiAnswerChatClanFail)
	}

	if err := session.SendPlain(Message{ID: rmiAnswerChatClanOK}); err != nil {
		return err
	}
	c.logger.Info(fmt.Sprintf("chat [%s] %s: %s", clan.Name, player.Name, text))

	row := notifyChatRow(player.Name, text, chatTypeDefault, flag)
	for _, member := range clan.Members {
		c.push(member.SteamID, Message{ID: rmiNotifyChat, Body: row})
	}
	return nil
}

func (c *Chat) whisper(session *Session, msg Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("whisper before authentication")
	}
	target, next, err := ReadProudString(msg.Body, 0)
	if err != nil {
		c.logger.Info(fmt.Sprintf("chat: malformed whisper body=%x: %v", msg.Body, err))
		return c.refuse(session, rmiAnswerChatUserFail)
	}
	text, flag, ok := readChatLine(msg.Body, next)
	if !ok {
		return c.refuse(session, rmiAnswerChatUserFail)
	}

	ctx, cancel := context5s()
	defer cancel()
	recipient, found, err := c.players.FindByName(ctx, strings.TrimSpace(target))
	if err != nil || !found || !c.sessions.IsOnline(recipient.SteamID) {
		c.logger.Info(fmt.Sprintf("chat: %s whispered %q, who is not around", player.Name, target))
		return c.refuse(session, rmiAnswerChatUserFail)
	}

	if err := session.SendPlain(Message{ID: rmiAnswerChatUserOK}); err != nil {
		return err
	}
	c.logger.Info(fmt.Sprintf("chat [whisper] %s -> %s: %s", player.Name, recipient.Name, text))

	row := notifyChatRow(player.Name, text, chatTypeDefault, flag)
	c.push(recipient.SteamID, Message{ID: rmiNotifyChat, Body: row})
	return session.SendPlain(Message{ID: rmiNotifyChat, Body: row})
}

// parse reads the shared [ProudString text][u8 flag] tail every chat request
// ends with.
func (c *Chat) parse(session *Session, msg Message, offset int) (models.Player, string, byte, bool) {
	player, ok := resolvePlayer(session)
	if !ok {
		return models.Player{}, "", 0, false
	}
	text, flag, ok := readChatLine(msg.Body, offset)
	if !ok {
		c.logger.Info(fmt.Sprintf("chat: malformed body=%x", msg.Body))
		return models.Player{}, "", 0, false
	}
	return player, text, flag, true
}

func readChatLine(body []byte, offset int) (string, byte, bool) {
	text, next, err := ReadProudString(body, offset)
	if err != nil {
		return "", 0, false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", 0, false
	}
	if len(text) > maxChatLength {
		text = text[:maxChatLength]
	}

	flag := byte(0)
	if next < len(body) {
		flag = body[next]
	}
	return text, flag, true
}

func (c *Chat) refuse(session *Session, id uint16) error {
	return session.SendPlain(Message{ID: id, Body: appendUint32(nil, 0)})
}

func (c *Chat) push(steamID string, msg Message) {
	if err := c.sessions.SendPlain(steamID, msg); err != nil {
		c.logger.Error(fmt.Sprintf("chat: push to %s: %v", steamID, err))
	}
}
