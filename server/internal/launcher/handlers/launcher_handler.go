package launcher

import (
	"context"
	"html/template"
	"net/url"
	"strings"
	"time"

	"go-service-template/internal/assets"
	"go-service-template/internal/auth"
	"go-service-template/internal/configuration"
	"go-service-template/internal/services"
	"go-service-template/pkg/logger"

	"github.com/gofiber/fiber/v2"
)

const launcherAuthCode = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var authCompleteTemplate = template.Must(template.ParseFS(assets.FS, "public/authcomplete.html"))

type Handler struct {
	logger        logger.Interface
	authenticator auth.Authenticator
	config        *configuration.AppConfig
	players       *services.PlayerService
}

func New(logger logger.Interface, authenticator auth.Authenticator, config *configuration.AppConfig, players *services.PlayerService) *Handler {
	return &Handler{logger: logger, authenticator: authenticator, config: config, players: players}
}

func (h *Handler) OAuth(c *fiber.Ctx) error {
	request := oauthLoginRequest{
		SteamID:       c.Query("steam_id"),
		SessionTicket: c.Query("steam_session_ticket"),
	}
	if err := request.Validate(); err != nil {
		h.logger.Warn("invalid steam login: " + err.Error())
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	result, err := h.authenticator.Authenticate(request.SteamID, request.SessionTicket)
	if err != nil {
		h.logger.Error("steam auth error: " + err.Error())
		return c.SendStatus(fiber.StatusBadGateway)
	}
	if !result.OK {
		h.logger.Warn("steam auth rejected for steam_id=" + request.SteamID)
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	account, err := h.players.LoginBySteam(ctx, result.SteamID)
	if err != nil {
		h.logger.Error("failed to persist player " + result.SteamID + ": " + err.Error())
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	h.logger.Info("steam login ok steam_id=" + account.SteamID + " name=" + account.Name)

	state := c.Query("state")
	target := c.Query("redirect_uri")
	if target == "" {
		target = c.Query("redirect_url")
	}
	if target != "" {
		return c.Redirect(buildRedirectURL(target, state, launcherAuthCode), fiber.StatusFound)
	}
	return h.authComplete(c, state, launcherAuthCode)
}

func (h *Handler) Callback(c *fiber.Ctx) error {
	code := c.Query("code")
	if code == "" {
		code = launcherAuthCode
	}
	state := c.Query("state")
	if state == "" {
		state = "xyz"
	}
	if code != launcherAuthCode || !strings.HasSuffix(c.OriginalURL(), "code="+url.QueryEscape(code)) {
		return c.Redirect(codeRedirectURL(state, launcherAuthCode), fiber.StatusFound)
	}
	return h.authComplete(c, state, launcherAuthCode)
}

func (h *Handler) Code2Token(c *fiber.Ctx) error {
	state := c.Query("state", "xyz")
	code := c.Query("code", launcherAuthCode)
	return c.Redirect(codeRedirectURL(state, code), fiber.StatusFound)
}

func (h *Handler) Notice(c *fiber.Ctx) error {
	return c.Type("html").SendString("<html><body><h1>FEAR Online</h1></body></html>")
}

func (h *Handler) SocialSuccess(c *fiber.Ctx) error {
	return c.Type("html").SendString(`<!doctype html><html><head><meta charset="utf-8"><title>success</title></head><body><script>window.name="success";document.title="success";setTimeout(function(){try{window.close();}catch(e){}},500);</script>success</body></html>`)
}

func (h *Handler) SocialRedirect(c *fiber.Ctx) error {
	return c.Redirect("/social_success?success=1", fiber.StatusFound)
}

func (h *Handler) OK(c *fiber.Ctx) error {
	return c.SendString("OK")
}

func (h *Handler) authComplete(c *fiber.Ctx, state, code string) error {
	data := map[string]string{
		"CallbackQuery": "state=" + url.QueryEscape(state) + "&code=" + url.QueryEscape(code),
		"State":         state,
		"Code":          code,
	}
	var page strings.Builder
	if err := authCompleteTemplate.Execute(&page, data); err != nil {
		h.logger.Error("failed to render auth complete page: " + err.Error())
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.SendString(page.String())
}

func buildRedirectURL(rawTarget, state, code string) string {
	target, err := url.Parse(rawTarget)
	if err != nil {
		return rawTarget
	}
	query := target.Query()
	query.Del("code")
	query.Del("state")
	target.RawQuery = appendStateAndCode(query.Encode(), state, code)
	return target.String()
}

func codeRedirectURL(state, code string) string {
	return "/content_only_launcher?" + appendStateAndCode("", state, code)
}

func appendStateAndCode(existing, state, code string) string {
	parts := make([]string, 0, 3)
	if existing != "" {
		parts = append(parts, existing)
	}
	if state != "" {
		parts = append(parts, "state="+url.QueryEscape(state))
	}
	parts = append(parts, "code="+url.QueryEscape(code))
	return strings.Join(parts, "&")
}
