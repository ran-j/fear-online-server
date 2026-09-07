package launcher_test

import (
	"fmt"
	"go-service-template/internal/catalog"
	"go-service-template/internal/configuration"
	launcherRouter "go-service-template/internal/launcher/routes"
	"go-service-template/internal/repositories"
	"go-service-template/internal/services"
	"go-service-template/pkg/clock"
	"go-service-template/pkg/logger"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestLauncherRouter(t *testing.T) {
	Logger := logger.New(
		"Foo",
		clock.Mock{},
		true,
	)
	config, _ := configuration.Load()
	gameCatalog, _ := catalog.Load()
	app := fiber.New()
	playerService := services.NewPlayerService(repositories.NewMemoryPlayerRepository(), gameCatalog, 50000, 50000)
	launcherRouter.RegisterRoutes(app, Logger, config, playerService)

	cases := []struct {
		name     string
		target   string
		expected int
	}{
		{"Steam auth issues code", "/dialog/oauth/authorize?steam_id=76561198114269084&steam_session_ticket=abc&state=xyz&redirect_url=http://127.0.0.1:8080/code2token.php", 302},
		{"Missing steam_id fails validation", "/dialog/oauth/authorize?state=xyz", 400},
		{"Non-numeric steam_id fails validation", "/dialog/oauth/authorize?steam_id=abc&state=xyz", 400},
		{"Code2Token redirects", "/code2token.php?state=xyz", 302},
		{"Callback without code redirects", "/content_only_launcher?state=xyz", 302},
		{"Notice is served", "/fogame/notice", 200},
		{"LivePatch config is served", "/LivePatch/Launcher/Config.xml", 200},
		{"Prerequisite catalog is served", "/LivePatch/ClientFear/PrerequisiteList.xml", 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", tc.target, nil))
			assert.Nil(t, err, fmt.Sprintf("Expected no error, but got '%s'", err))
			assert.Equal(t, tc.expected, resp.StatusCode)
		})
	}
}
