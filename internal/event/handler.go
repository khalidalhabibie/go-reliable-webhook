package event

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/khalid/go-reliable-webhook/internal/platform/httpresponse"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/events", h.create)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var req CreateEventRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(httpresponse.Error("invalid request body"))
	}

	res, err := h.service.Create(c.UserContext(), req, c.Get("Idempotency-Key"))
	if err != nil {
		return writeError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(httpresponse.OK(res))
}

func writeError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidEventPayload):
		return c.Status(fiber.StatusBadRequest).JSON(httpresponse.Error(err.Error()))
	case errors.Is(err, ErrIdempotencyConflict):
		return c.Status(fiber.StatusConflict).JSON(httpresponse.Error(err.Error()))
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(httpresponse.Error("internal server error"))
	}
}
