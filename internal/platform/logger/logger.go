package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

func New(appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if appEnv == "development" || appEnv == "local" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

func Middleware(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
		}

		log.Info("http_request",
			"request_id", requestID(c),
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", duration.Milliseconds(),
		)

		return err
	}
}

func requestID(c *fiber.Ctx) string {
	if value, ok := c.Locals("requestid").(string); ok {
		return value
	}
	return c.GetRespHeader(fiber.HeaderXRequestID)
}
