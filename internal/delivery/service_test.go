package delivery

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	filters  ListFilters
	items    []Delivery
	delivery Delivery
	replayed bool
	err      error
}

func (r *fakeRepository) List(_ context.Context, filters ListFilters) ([]Delivery, error) {
	r.filters = filters
	if r.err != nil {
		return nil, r.err
	}
	return r.items, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id string) (Delivery, error) {
	if r.err != nil {
		return Delivery{}, r.err
	}
	if r.delivery.ID == id {
		return r.delivery, nil
	}
	return Delivery{}, ErrNotFound
}

func (r *fakeRepository) Replay(_ context.Context, id string) (Delivery, error) {
	if r.err != nil {
		return Delivery{}, r.err
	}
	if r.delivery.ID != id {
		return Delivery{}, ErrNotReplayable
	}

	r.replayed = true
	item := r.delivery
	item.Status = DeliveryStatusPending
	item.ReplayCount++
	now := time.Now().UTC()
	item.NextRetryAt = &now
	return item, nil
}

func TestServiceListAppliesPaginationDefaults(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)

	res, err := service.List(context.Background(), ListFilters{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if repo.filters.Page != defaultPage {
		t.Fatalf("page = %d, want %d", repo.filters.Page, defaultPage)
	}
	if repo.filters.Size != defaultSize {
		t.Fatalf("size = %d, want %d", repo.filters.Size, defaultSize)
	}
	if res.Page != defaultPage || res.Size != defaultSize {
		t.Fatalf("response pagination = page %d size %d, want page %d size %d", res.Page, res.Size, defaultPage, defaultSize)
	}
}

func TestServiceListCapsPageSize(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)

	_, err := service.List(context.Background(), ListFilters{Page: 1, Size: 500})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if repo.filters.Size != maxPageSize {
		t.Fatalf("size = %d, want %d", repo.filters.Size, maxPageSize)
	}
}

func TestServiceGetByIDIncludesTruncatedAttemptResponseBody(t *testing.T) {
	longBody := strings.Repeat("a", responseBodyPreviewLimit+50)
	now := time.Now().UTC()
	repo := &fakeRepository{
		delivery: Delivery{
			ID:           "delivery_1",
			EventID:      "event_1",
			SubscriberID: "subscriber_1",
			Status:       "FAILED",
			AttemptCount: 1,
			MaxAttempt:   5,
			CreatedAt:    now,
			UpdatedAt:    now,
			Attempts: []Attempt{
				{
					ID:            "attempt_1",
					DeliveryID:    "delivery_1",
					AttemptNumber: 1,
					RequestURL:    "https://example.com/webhooks",
					RequestBody:   []byte(`{"ok":true}`),
					ResponseBody:  &longBody,
					CreatedAt:     now,
				},
			},
		},
	}
	service := NewService(repo)

	res, err := service.GetByID(context.Background(), "delivery_1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if len(res.Attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(res.Attempts))
	}
	if res.Attempts[0].ResponseBody == nil {
		t.Fatal("response body is nil")
	}
	if len(*res.Attempts[0].ResponseBody) != responseBodyPreviewLimit {
		t.Fatalf("response body length = %d, want %d", len(*res.Attempts[0].ResponseBody), responseBodyPreviewLimit)
	}
}

func TestServiceReplayPreservesAttemptCountAndSchedulesPending(t *testing.T) {
	repo := &fakeRepository{
		delivery: Delivery{
			ID:           "delivery_1",
			EventID:      "event_1",
			SubscriberID: "subscriber_1",
			Status:       DeliveryStatusDead,
			AttemptCount: 5,
			MaxAttempt:   5,
			ReplayCount:  1,
		},
	}
	service := NewService(repo)

	res, err := service.Replay(context.Background(), "delivery_1")
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}

	if !repo.replayed {
		t.Fatal("repository Replay was not called")
	}
	if res.Status != DeliveryStatusPending {
		t.Fatalf("status = %q, want %q", res.Status, DeliveryStatusPending)
	}
	if res.AttemptCount != 5 {
		t.Fatalf("attempt_count = %d, want 5", res.AttemptCount)
	}
	if res.ReplayCount != 2 {
		t.Fatalf("replay_count = %d, want 2", res.ReplayCount)
	}
	if res.NextRetryAt == nil {
		t.Fatal("next_retry_at is nil")
	}
}

func TestServiceReplayRejectsEmptyID(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, err := service.Replay(context.Background(), " ")
	if err != ErrInvalidInput {
		t.Fatalf("Replay() error = %v, want %v", err, ErrInvalidInput)
	}
}
