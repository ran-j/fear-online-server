package proudnet

import (
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"
)

type response struct {
	answer    uint16
	body      []byte
	followUps []followUp
}

type Handlers struct {
	players     *services.PlayerService
	clans       *services.ClanService
	logger      logger.Interface
	channels    []Channel
	credentials *Credentials
	sessions    *SessionRegistry
}

func NewLoginHandlers(players *services.PlayerService, clans *services.ClanService, logger logger.Interface, channels []Channel, credentials *Credentials) *Handlers {
	return &Handlers{players: players, clans: clans, logger: logger, channels: channels, credentials: credentials}
}

func (h *Handlers) Register(server *Server) {
	h.sessions = server.Sessions()
	server.Handle(rmiRequestAeriaAuth, h.aeriaAuth)
	server.Handle(rmiRequestAeriaLauncher, h.aeriaLauncher)
	server.Handle(rmiCreateCredential, h.createCredential)
	server.Handle(rmiCheckCredential, h.checkCredential)
	server.Handle(rmiRequestChannelList, h.requestChannelList)
	server.Handle(rmiRequestChannelJoin, h.joinChannel)
	server.Handle(rmiRequestLogin, h.login)
	server.Handle(rmiItemRequestLogin, h.itemLogin)
	server.Handle(rmiMapQuestRequestLogin, h.mapQuestLogin)
}

func (h *Handlers) sendResponse(session *Session, resp response) error {
	if err := session.SendPlain(Message{ID: resp.answer, Body: resp.body}); err != nil {
		return err
	}
	for _, push := range resp.followUps {
		if err := session.SendPlain(Message{ID: push.id, Body: push.body}); err != nil {
			return err
		}
	}
	return nil
}

// sendBoth sends a message encrypted and then plaintext — the auth acks the client accepts in either form.
func sendBoth(session *Session, msg Message) error {
	if err := session.SendEncrypted(msg); err != nil {
		return err
	}
	return session.SendPlain(msg)
}
