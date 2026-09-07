package auth

import "go-service-template/pkg/logger"

type Result struct {
	OK      bool
	SteamID string
}

type Authenticator interface {
	Authenticate(steamID, sessionTicket string) (Result, error)
}

type Mock struct {
	logger logger.Interface
	url    string
}

func NewMock(logger logger.Interface) *Mock {
	return &Mock{logger: logger, url: "http://mock-auth.local/steam/authenticate"}
}

func (m *Mock) Authenticate(steamID, sessionTicket string) (Result, error) {
	m.logger.Info("mock steam auth POST " + m.url + " steam_id=" + steamID)
	if steamID == "" {
		return Result{OK: false}, nil
	}
	return Result{OK: true, SteamID: steamID}, nil
}
