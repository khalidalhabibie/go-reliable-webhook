package delivery

import (
	"encoding/json"
	"time"
)

type Delivery struct {
	ID            string
	EventID       string
	SubscriberID  string
	Status        string
	AttemptCount  int
	MaxAttempt    int
	NextRetryAt   *time.Time
	LastAttemptAt *time.Time
	ReplayCount   int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Attempts      []Attempt
}

type Attempt struct {
	ID                 string
	DeliveryID         string
	AttemptNumber      int
	RequestURL         string
	RequestHeaders     json.RawMessage
	RequestBody        json.RawMessage
	ResponseStatusCode *int
	ResponseBody       *string
	ErrorMessage       *string
	DurationMS         *int
	CreatedAt          time.Time
}
