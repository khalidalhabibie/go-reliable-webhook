package event

import (
	"encoding/json"
	"time"
)

type CreateEventRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

type EventResponse struct {
	ID                 string          `json:"id"`
	EventType          string          `json:"event_type"`
	Payload            json.RawMessage `json:"payload"`
	Status             string          `json:"status"`
	IdempotencyKey     *string         `json:"idempotency_key,omitempty"`
	PayloadFingerprint string          `json:"payload_fingerprint"`
	DeliveryCount      int             `json:"delivery_count"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func toResponse(evt Event) EventResponse {
	return EventResponse(evt)
}
