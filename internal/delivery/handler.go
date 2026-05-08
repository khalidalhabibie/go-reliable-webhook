package delivery

import (
	"errors"
	"strconv"

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
	router.Get("/deliveries", h.list)
	router.Get("/deliveries/:id", h.getByID)
}

func (h *Handler) list(c *fiber.Ctx) error {
	page, err := optionalPositiveInt(c.Query("page"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(httpresponse.Error("page must be a positive integer"))
	}
	size, err := optionalPositiveInt(c.Query("size"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(httpresponse.Error("size must be a positive integer"))
	}

	res, err := h.service.List(c.UserContext(), ListFilters{
		Status:       c.Query("status"),
		EventID:      c.Query("event_id"),
		SubscriberID: c.Query("subscriber_id"),
		Page:         page,
		Size:         size,
	})
	if err != nil {
		return writeError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(httpresponse.OK(res))
}

func (h *Handler) getByID(c *fiber.Ctx) error {
	res, err := h.service.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(httpresponse.OK(res))
}

func optionalPositiveInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, ErrInvalidInput
	}
	return parsed, nil
}

func writeError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrInvalidInput):
		return c.Status(fiber.StatusBadRequest).JSON(httpresponse.Error(err.Error()))
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(httpresponse.Error(err.Error()))
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(httpresponse.Error("internal server error"))
	}
}
