package event

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput        = errors.New("invalid event input")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different payload")
	ErrInvalidEventPayload = errors.New("invalid event payload")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req CreateEventRequest, idempotencyKey string) (EventResponse, error) {
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" || len(req.Payload) == 0 {
		return EventResponse{}, ErrInvalidInput
	}

	normalizedPayload, fingerprint, err := normalizeAndFingerprint(req.Payload)
	if err != nil {
		return EventResponse{}, ErrInvalidEventPayload
	}

	var key *string
	trimmedKey := strings.TrimSpace(idempotencyKey)
	if trimmedKey != "" {
		key = &trimmedKey
	}

	now := time.Now().UTC()
	evt := Event{
		ID:                 uuid.NewString(),
		EventType:          eventType,
		Payload:            normalizedPayload,
		IdempotencyKey:     key,
		PayloadFingerprint: fingerprint,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	stored, existing, err := s.repo.CreateWithDeliveries(ctx, evt)
	if err != nil {
		return EventResponse{}, err
	}
	if existing && stored.PayloadFingerprint != fingerprint {
		return EventResponse{}, ErrIdempotencyConflict
	}

	return toResponse(stored), nil
}

func normalizeAndFingerprint(payload json.RawMessage) (json.RawMessage, string, error) {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, "", err
	}

	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}

	sum := sha256.Sum256(normalized)
	return normalized, hex.EncodeToString(sum[:]), nil
}
