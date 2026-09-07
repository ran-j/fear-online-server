package middlewares

import (
	"strconv"

	"go-service-template/pkg/logger"

	"github.com/gofiber/fiber/v2"
)

func RequestLogger(logger logger.Interface) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		logger.Info(c.Method() + " " + c.OriginalURL() + " -> " + strconv.Itoa(c.Response().StatusCode()))
		return err
	}
}
