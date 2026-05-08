package delivery

import (
	"encoding/json"
	"time"
)

const responseBodyPreviewLimit = 2000

type ListFilters struct {
	Status       string
	EventID      string
	SubscriberID string
	Page         int
	Size         int
}

type ListResponse struct {
	Items []DeliveryResponse `json:"items"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}

type DeliveryResponse struct {
	ID            string     `json:"id"`
	EventID       string     `json:"event_id"`
	SubscriberID  string     `json:"subscriber_id"`
	Status        string     `json:"status"`
	AttemptCount  int        `json:"attempt_count"`
	MaxAttempt    int        `json:"max_attempt"`
	NextRetryAt   *time.Time `json:"next_retry_at"`
	LastAttemptAt *time.Time `json:"last_attempt_at"`
	ReplayCount   int        `json:"replay_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type DeliveryDetailResponse struct {
	DeliveryResponse
	Attempts []AttemptResponse `json:"attempt_logs"`
}

type AttemptResponse struct {
	ID                 string          `json:"id"`
	DeliveryID         string          `json:"delivery_id"`
	AttemptNumber      int             `json:"attempt_number"`
	RequestURL         string          `json:"request_url"`
	RequestHeaders     json.RawMessage `json:"request_headers,omitempty"`
	RequestBody        json.RawMessage `json:"request_body"`
	ResponseStatusCode *int            `json:"response_status_code"`
	ResponseBody       *string         `json:"response_body"`
	ErrorMessage       *string         `json:"error_message"`
	DurationMS         *int            `json:"duration_ms"`
	CreatedAt          time.Time       `json:"created_at"`
}

func toListResponse(deliveries []Delivery, filters ListFilters) ListResponse {
	items := make([]DeliveryResponse, 0, len(deliveries))
	for _, item := range deliveries {
		items = append(items, toDeliveryResponse(item))
	}
	return ListResponse{
		Items: items,
		Page:  filters.Page,
		Size:  filters.Size,
	}
}

func toDetailResponse(delivery Delivery) DeliveryDetailResponse {
	attempts := make([]AttemptResponse, 0, len(delivery.Attempts))
	for _, attempt := range delivery.Attempts {
		attempts = append(attempts, toAttemptResponse(attempt))
	}
	return DeliveryDetailResponse{
		DeliveryResponse: toDeliveryResponse(delivery),
		Attempts:         attempts,
	}
}

func toDeliveryResponse(delivery Delivery) DeliveryResponse {
	return DeliveryResponse{
		ID:            delivery.ID,
		EventID:       delivery.EventID,
		SubscriberID:  delivery.SubscriberID,
		Status:        delivery.Status,
		AttemptCount:  delivery.AttemptCount,
		MaxAttempt:    delivery.MaxAttempt,
		NextRetryAt:   delivery.NextRetryAt,
		LastAttemptAt: delivery.LastAttemptAt,
		ReplayCount:   delivery.ReplayCount,
		CreatedAt:     delivery.CreatedAt,
		UpdatedAt:     delivery.UpdatedAt,
	}
}

func toAttemptResponse(attempt Attempt) AttemptResponse {
	responseBody := truncateStringPointer(attempt.ResponseBody, responseBodyPreviewLimit)
	return AttemptResponse{
		ID:                 attempt.ID,
		DeliveryID:         attempt.DeliveryID,
		AttemptNumber:      attempt.AttemptNumber,
		RequestURL:         attempt.RequestURL,
		RequestHeaders:     attempt.RequestHeaders,
		RequestBody:        attempt.RequestBody,
		ResponseStatusCode: attempt.ResponseStatusCode,
		ResponseBody:       responseBody,
		ErrorMessage:       attempt.ErrorMessage,
		DurationMS:         attempt.DurationMS,
		CreatedAt:          attempt.CreatedAt,
	}
}

func truncateStringPointer(value *string, limit int) *string {
	if value == nil || len(*value) <= limit {
		return value
	}
	truncated := (*value)[:limit]
	return &truncated
}
