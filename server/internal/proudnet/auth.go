package proudnet

import (
	"context"
	"fmt"
	"time"
)

const (
	rmiRequestAeriaAuth     uint16 = 0x7534
	rmiAnswerAeriaAuth      uint16 = 0x756A
	rmiRequestAeriaLauncher uint16 = 0x7535
	rmiAnswerAeriaLauncher  uint16 = 0x756C

	rmiCreateCredential       uint16 = 0x7532
	rmiAnswerCreateCredential uint16 = 0x7565
	rmiCheckCredential        uint16 = 0x7533
	rmiAnswerCheckCredential  uint16 = 0x7567
)

// aeriaAuth reads the auth code + SteamID, loads (or creates) the account, keeps
// it on the session, and acks. The acks go both encrypted and plaintext.
func (h *Handlers) aeriaAuth(session *Session, msg Message) error {
	_, next, err := ReadProudString(msg.Body, 0) // auth code
	if err != nil {
		return fmt.Errorf("read auth code: %w", err)
	}
	steamID, _, err := ReadProudString(msg.Body, next)
	if err != nil {
		return fmt.Errorf("read steam id: %w", err)
	}
	h.logger.Info("login: RequestAeriaAutentication steam_id=" + steamID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	player, err := h.players.LoginBySteam(ctx, steamID)
	if err != nil {
		return fmt.Errorf("load player %s: %w", steamID, err)
	}
	bindPlayer(session, player)
	h.logger.Info("login: account loaded name=" + player.Name)

	if err := sendBoth(session, Message{ID: rmiAnswerAeriaAuth}); err != nil {
		return err
	}
	if err := sendBoth(session, Message{ID: rmiAnswerAeriaLauncher}); err != nil {
		return err
	}
	return h.sendChannelList(session)
}

func (h *Handlers) aeriaLauncher(session *Session, _ Message) error {
	return sendBoth(session, Message{ID: rmiAnswerAeriaLauncher})
}

// createCredential issues a 16-byte connect guid token bound to this connection's account.
func (h *Handlers) createCredential(session *Session, _ Message) error {
	player, ok := resolvePlayer(session)
	if !ok {
		return fmt.Errorf("create credential before authentication")
	}
	if len(session.connectGUID) != 16 {
		return fmt.Errorf("create credential before client connect")
	}
	h.credentials.Issue(session.connectGUID, player)
	h.logger.Info(fmt.Sprintf("login: issued credential %x for %s", session.connectGUID, player.Name))
	return session.SendPlain(Message{ID: rmiAnswerCreateCredential, Body: session.connectGUID})
}

func (h *Handlers) checkCredential(session *Session, msg Message) error {
	if len(msg.Body) < 16 {
		return fmt.Errorf("check credential without a token")
	}
	token := msg.Body[:16]
	player, ok := h.credentials.Redeem(token)
	if !ok {
		return fmt.Errorf("check credential: unknown token %x", token)
	}
	bindPlayer(session, player)
	h.logger.Info(fmt.Sprintf("login: credential %x redeemed for %s", token, player.Name))

	if err := session.SendPlain(Message{ID: rmiAnswerCheckCredential, Body: int32LE(0)}); err != nil {
		return err
	}
	return h.sendChannelList(session)
}
