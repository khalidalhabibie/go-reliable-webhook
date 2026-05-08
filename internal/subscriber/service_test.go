package subscriber

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	created     Subscriber
	subscribers []Subscriber
	err         error
}

func (r *fakeRepository) Create(_ context.Context, sub Subscriber) (Subscriber, error) {
	r.created = sub
	if r.err != nil {
		return Subscriber{}, r.err
	}
	return sub, nil
}

func (r *fakeRepository) List(_ context.Context) ([]Subscriber, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.subscribers, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id string) (Subscriber, error) {
	if r.err != nil {
		return Subscriber{}, r.err
	}
	for _, sub := range r.subscribers {
		if sub.ID == id {
			return sub, nil
		}
	}
	return Subscriber{}, ErrNotFound
}

func (r *fakeRepository) Deactivate(_ context.Context, id string) (Subscriber, error) {
	if r.err != nil {
		return Subscriber{}, r.err
	}
	for _, sub := range r.subscribers {
		if sub.ID == id {
			sub.Status = StatusInactive
			return sub, nil
		}
	}
	return Subscriber{}, ErrNotFound
}

func TestServiceCreateGeneratesSecretAndHidesFullSecret(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)

	res, err := service.Create(context.Background(), CreateSubscriberRequest{
		Name:      "Billing",
		URL:       "https://example.com/webhooks",
		EventType: "invoice.created",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if repo.created.Status != StatusActive {
		t.Fatalf("status = %q, want %q", repo.created.Status, StatusActive)
	}
	if !strings.HasPrefix(repo.created.Secret, "whsec_") {
		t.Fatalf("secret = %q, want whsec_ prefix", repo.created.Secret)
	}
	if res.SecretPreview == "" {
		t.Fatal("SecretPreview is empty")
	}
	if res.SecretPreview == repo.created.Secret {
		t.Fatal("response exposed full secret")
	}
	if res.Status != StatusActive {
		t.Fatalf("response status = %q, want %q", res.Status, StatusActive)
	}
}

func TestServiceCreateRejectsInvalidURL(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, err := service.Create(context.Background(), CreateSubscriberRequest{
		Name:      "Billing",
		URL:       "not-a-url",
		EventType: "invoice.created",
	})
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("Create() error = %v, want %v", err, ErrInvalidURL)
	}
}

func TestServiceDeactivate(t *testing.T) {
	now := time.Now().UTC()
	service := NewService(&fakeRepository{subscribers: []Subscriber{
		{
			ID:        "sub_1",
			Name:      "Billing",
			URL:       "https://example.com/webhooks",
			EventType: "invoice.created",
			Secret:    "whsec_secretsecretsecret",
			Status:    StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}})

	res, err := service.Deactivate(context.Background(), "sub_1")
	if err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}
	if res.Status != StatusInactive {
		t.Fatalf("status = %q, want %q", res.Status, StatusInactive)
	}
	if res.SecretPreview == "whsec_secretsecretsecret" {
		t.Fatal("response exposed full secret")
	}
}
