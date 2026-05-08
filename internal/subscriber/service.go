package subscriber

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid subscriber input")
	ErrInvalidURL   = errors.New("invalid subscriber url")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req CreateSubscriberRequest) (SubscriberResponse, error) {
	name := strings.TrimSpace(req.Name)
	rawURL := strings.TrimSpace(req.URL)
	eventType := strings.TrimSpace(req.EventType)

	if name == "" || rawURL == "" || eventType == "" {
		return SubscriberResponse{}, ErrInvalidInput
	}
	if !validSubscriberURL(rawURL) {
		return SubscriberResponse{}, ErrInvalidURL
	}

	secret, err := generateSecret()
	if err != nil {
		return SubscriberResponse{}, fmt.Errorf("generate subscriber secret: %w", err)
	}

	now := time.Now().UTC()
	sub := Subscriber{
		ID:        uuid.NewString(),
		Name:      name,
		URL:       rawURL,
		EventType: eventType,
		Secret:    secret,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	created, err := s.repo.Create(ctx, sub)
	if err != nil {
		return SubscriberResponse{}, err
	}

	return toResponse(created), nil
}

func (s *Service) List(ctx context.Context) ([]SubscriberResponse, error) {
	subs, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return toResponses(subs), nil
}

func (s *Service) GetByID(ctx context.Context, id string) (SubscriberResponse, error) {
	if strings.TrimSpace(id) == "" {
		return SubscriberResponse{}, ErrInvalidInput
	}

	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return SubscriberResponse{}, err
	}
	return toResponse(sub), nil
}

func (s *Service) Deactivate(ctx context.Context, id string) (SubscriberResponse, error) {
	if strings.TrimSpace(id) == "" {
		return SubscriberResponse{}, ErrInvalidInput
	}

	sub, err := s.repo.Deactivate(ctx, id)
	if err != nil {
		return SubscriberResponse{}, err
	}
	return toResponse(sub), nil
}

func validSubscriberURL(rawURL string) bool {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}
	return parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func generateSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "whsec_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}
