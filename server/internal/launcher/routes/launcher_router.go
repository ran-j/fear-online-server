package launcher

import (
	"go-service-template/internal/auth"
	"go-service-template/internal/configuration"
	launcherHandler "go-service-template/internal/launcher/handlers"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App, logger *logger.Logger, config *configuration.AppConfig, players *services.PlayerService) {
	handler := launcherHandler.New(logger, auth.NewMock(logger), config, players)

	app.Get("/LivePatch/*", handler.PatchFile)

	app.Get("/dialog/oauth", handler.OAuth)
	app.Get("/dialog/oauth/authorize", handler.OAuth)
	app.Get("/content_only_launcher", handler.Callback)
	app.Get("/code2token.php", handler.Code2Token)

	app.Get("/fogame/notice", handler.Notice)
	app.Get("/fogame/steam_notice", handler.Notice)
	app.Get("/news", handler.Notice)
	app.Get("/contact", handler.Notice)

	app.Get("/social_success", handler.SocialSuccess)
	app.Get("/steam_success", handler.SocialSuccess)
	app.All("/social_connect/steam/connect/callback/redirect", handler.SocialRedirect)
	app.All("/redirect", handler.SocialRedirect)

	app.All("/auth", handler.OK)
	app.All("/login", handler.OK)
	app.All("/social", handler.OK)
}
