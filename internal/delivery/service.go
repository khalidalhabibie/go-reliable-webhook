package delivery

import (
	"context"
	"errors"
	"strings"
)

const (
	defaultPage = 1
	defaultSize = 20
	maxPageSize = 100
)

var ErrInvalidInput = errors.New("invalid delivery input")
var ErrNotReplayable = errors.New("delivery is not replayable")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, filters ListFilters) (ListResponse, error) {
	filters.Status = strings.TrimSpace(filters.Status)
	filters.EventID = strings.TrimSpace(filters.EventID)
	filters.SubscriberID = strings.TrimSpace(filters.SubscriberID)

	if filters.Page <= 0 {
		filters.Page = defaultPage
	}
	if filters.Size <= 0 {
		filters.Size = defaultSize
	}
	if filters.Size > maxPageSize {
		filters.Size = maxPageSize
	}

	deliveries, err := s.repo.List(ctx, filters)
	if err != nil {
		return ListResponse{}, err
	}

	return toListResponse(deliveries, filters), nil
}

func (s *Service) GetByID(ctx context.Context, id string) (DeliveryDetailResponse, error) {
	if strings.TrimSpace(id) == "" {
		return DeliveryDetailResponse{}, ErrInvalidInput
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return DeliveryDetailResponse{}, err
	}

	return toDetailResponse(item), nil
}

func (s *Service) Replay(ctx context.Context, id string) (DeliveryResponse, error) {
	if strings.TrimSpace(id) == "" {
		return DeliveryResponse{}, ErrInvalidInput
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return DeliveryResponse{}, err
	}
	if !isReplayableStatus(existing.Status) {
		return DeliveryResponse{}, ErrNotReplayable
	}

	item, err := s.repo.Replay(ctx, id)
	if err != nil {
		return DeliveryResponse{}, err
	}

	return toDeliveryResponse(item), nil
}

func isReplayableStatus(status string) bool {
	return status == DeliveryStatusFailed || status == DeliveryStatusDead
}
