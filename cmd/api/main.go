package main

import (
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"github.com/khalid/go-reliable-webhook/internal/platform/config"
	"github.com/khalid/go-reliable-webhook/internal/platform/httpresponse"
	"github.com/khalid/go-reliable-webhook/internal/platform/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.AppEnv)
	app := newApp(log)

	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newApp(log *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "go-reliable-webhook",
		DisableStartupMessage: true,
	})

	app.Use(requestid.New())
	app.Use(logger.Middleware(log))

	api := app.Group("/api/v1")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(httpresponse.OK(fiber.Map{
			"status": "ok",
		}))
	})

	return app
}
