package event

import (
	"encoding/json"
	"time"
)

const (
	StatusPending   = "PENDING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"

	DeliveryStatusPending = "PENDING"
)

type Event struct {
	ID                 string
	EventType          string
	Payload            json.RawMessage
	Status             string
	IdempotencyKey     *string
	PayloadFingerprint string
	DeliveryCount      int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
