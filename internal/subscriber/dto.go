package subscriber

import "time"

type CreateSubscriberRequest struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	EventType string `json:"event_type"`
}

type SubscriberResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	EventType     string    `json:"event_type"`
	SecretPreview string    `json:"secret_preview"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toResponse(sub Subscriber) SubscriberResponse {
	return SubscriberResponse{
		ID:            sub.ID,
		Name:          sub.Name,
		URL:           sub.URL,
		EventType:     sub.EventType,
		SecretPreview: previewSecret(sub.Secret),
		Status:        sub.Status,
		CreatedAt:     sub.CreatedAt,
		UpdatedAt:     sub.UpdatedAt,
	}
}

func toResponses(subs []Subscriber) []SubscriberResponse {
	responses := make([]SubscriberResponse, 0, len(subs))
	for _, sub := range subs {
		responses = append(responses, toResponse(sub))
	}
	return responses
}

func previewSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 10 {
		return secret[:min(len(secret), 6)] + "..."
	}

	return secret[:10] + "..." + secret[len(secret)-4:]
}
