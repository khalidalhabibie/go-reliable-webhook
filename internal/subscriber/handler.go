package subscriber

import (
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
	router.Post("/subscribers", h.create)
	router.Get("/subscribers", h.list)
	router.Get("/subscribers/:id", h.getByID)
	router.Patch("/subscribers/:id/deactivate", h.deactivate)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var req CreateSubscriberRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(httpresponse.Error("invalid request body"))
	}

	res, err := h.service.Create(c.UserContext(), req)
	if err != nil {
		return writeError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(httpresponse.OK(res))
}

func (h *Handler) list(c *fiber.Ctx) error {
	res, err := h.service.List(c.UserContext())
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

func (h *Handler) deactivate(c *fiber.Ctx) error {
	res, err := h.service.Deactivate(c.UserContext(), c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(httpresponse.OK(res))
}

func writeError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidURL):
		return c.Status(fiber.StatusBadRequest).JSON(httpresponse.Error(err.Error()))
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(httpresponse.Error(err.Error()))
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(httpresponse.Error("internal server error"))
	}
}
