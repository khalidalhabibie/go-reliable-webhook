package event

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	stored       Event
	existing     bool
	createdEvent Event
	err          error
}

func (r *fakeRepository) CreateWithDeliveries(_ context.Context, evt Event) (Event, bool, error) {
	r.createdEvent = evt
	if r.err != nil {
		return Event{}, false, r.err
	}
	if r.existing {
		return r.stored, true, nil
	}
	evt.Status = StatusPending
	evt.DeliveryCount = 2
	return evt, false, nil
}

func TestServiceCreateNewEventWithIdempotencyKey(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo)

	res, err := service.Create(context.Background(), CreateEventRequest{
		EventType: "payment.succeeded",
		Payload:   []byte(`{"currency":"IDR","amount":150000,"payment_id":"pay_123"}`),
	}, "retry-key-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if res.Status != StatusPending {
		t.Fatalf("status = %q, want %q", res.Status, StatusPending)
	}
	if res.DeliveryCount != 2 {
		t.Fatalf("delivery_count = %d, want 2", res.DeliveryCount)
	}
	if repo.createdEvent.IdempotencyKey == nil || *repo.createdEvent.IdempotencyKey != "retry-key-1" {
		t.Fatalf("idempotency key was not passed to repository")
	}
	if repo.createdEvent.PayloadFingerprint == "" {
		t.Fatal("payload fingerprint is empty")
	}
}

func TestServiceCreateReturnsExistingForSameIdempotencyKeyAndPayload(t *testing.T) {
	_, fingerprint, err := normalizeAndFingerprint([]byte(`{"amount":150000,"currency":"IDR","payment_id":"pay_123"}`))
	if err != nil {
		t.Fatalf("normalizeAndFingerprint() error = %v", err)
	}

	repo := &fakeRepository{
		existing: true,
		stored: Event{
			ID:                 "evt_existing",
			EventType:          "payment.succeeded",
			Payload:            []byte(`{"amount":150000,"currency":"IDR","payment_id":"pay_123"}`),
			Status:             StatusPending,
			PayloadFingerprint: fingerprint,
			DeliveryCount:      1,
		},
	}
	service := NewService(repo)

	res, err := service.Create(context.Background(), CreateEventRequest{
		EventType: "payment.succeeded",
		Payload:   []byte(`{"payment_id":"pay_123","amount":150000,"currency":"IDR"}`),
	}, "retry-key-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if res.ID != "evt_existing" {
		t.Fatalf("ID = %q, want evt_existing", res.ID)
	}
}

func TestServiceCreateRejectsSameIdempotencyKeyWithDifferentPayload(t *testing.T) {
	_, fingerprint, err := normalizeAndFingerprint([]byte(`{"payment_id":"pay_123","amount":150000,"currency":"IDR"}`))
	if err != nil {
		t.Fatalf("normalizeAndFingerprint() error = %v", err)
	}

	repo := &fakeRepository{
		existing: true,
		stored: Event{
			ID:                 "evt_existing",
			EventType:          "payment.succeeded",
			PayloadFingerprint: fingerprint,
		},
	}
	service := NewService(repo)

	_, err = service.Create(context.Background(), CreateEventRequest{
		EventType: "payment.succeeded",
		Payload:   []byte(`{"payment_id":"pay_123","amount":200000,"currency":"IDR"}`),
	}, "retry-key-1")
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("Create() error = %v, want %v", err, ErrIdempotencyConflict)
	}
}

func TestServiceCreateRejectsInvalidPayload(t *testing.T) {
	service := NewService(&fakeRepository{})

	_, err := service.Create(context.Background(), CreateEventRequest{
		EventType: "payment.succeeded",
		Payload:   []byte(`{"payment_id":`),
	}, "")
	if !errors.Is(err, ErrInvalidEventPayload) {
		t.Fatalf("Create() error = %v, want %v", err, ErrInvalidEventPayload)
	}
}
