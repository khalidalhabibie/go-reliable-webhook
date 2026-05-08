package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"github.com/khalid/go-reliable-webhook/internal/delivery"
	"github.com/khalid/go-reliable-webhook/internal/event"
	"github.com/khalid/go-reliable-webhook/internal/platform/config"
	"github.com/khalid/go-reliable-webhook/internal/platform/database"
	"github.com/khalid/go-reliable-webhook/internal/platform/httpresponse"
	"github.com/khalid/go-reliable-webhook/internal/platform/logger"
	"github.com/khalid/go-reliable-webhook/internal/subscriber"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.AppEnv)
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.Ping(ctx, db); err != nil {
		log.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	subscriberRepository := subscriber.NewPostgresRepository(db)
	subscriberService := subscriber.NewService(subscriberRepository)
	subscriberHandler := subscriber.NewHandler(subscriberService)
	eventRepository := event.NewPostgresRepository(db)
	eventService := event.NewService(eventRepository)
	eventHandler := event.NewHandler(eventService)
	deliveryRepository := delivery.NewPostgresRepository(db)
	deliveryService := delivery.NewService(deliveryRepository)
	deliveryHandler := delivery.NewHandler(deliveryService)

	app := newApp(log, subscriberHandler, eventHandler, deliveryHandler)

	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newApp(log *slog.Logger, subscriberHandler *subscriber.Handler, eventHandler *event.Handler, deliveryHandler *delivery.Handler) *fiber.App {
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
	if subscriberHandler != nil {
		subscriberHandler.RegisterRoutes(api)
	}
	if eventHandler != nil {
		eventHandler.RegisterRoutes(api)
	}
	if deliveryHandler != nil {
		deliveryHandler.RegisterRoutes(api)
	}

	return app
}
