package subscriber

import "time"

const (
	StatusActive   = "ACTIVE"
	StatusInactive = "INACTIVE"
)

type Subscriber struct {
	ID        string
	Name      string
	URL       string
	EventType string
	Secret    string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
