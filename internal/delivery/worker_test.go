package delivery

import (
	"net/http"
	"testing"
	"time"
)

func TestDecideDeliveryStatusSuccess(t *testing.T) {
	statusCode := http.StatusNoContent

	decision := decideDeliveryStatus(SendResult{
		StatusCode:  &statusCode,
		ShouldRetry: false,
	}, 1, 5, time.Now())

	if decision.Status != DeliveryStatusSuccess {
		t.Fatalf("status = %q, want %q", decision.Status, DeliveryStatusSuccess)
	}
	if decision.NextRetryAt != nil {
		t.Fatal("NextRetryAt is not nil")
	}
}

func TestDecideDeliveryStatusRetrying(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)

	decision := decideDeliveryStatus(SendResult{ShouldRetry: true}, 1, 5, now)

	if decision.Status != DeliveryStatusRetrying {
		t.Fatalf("status = %q, want %q", decision.Status, DeliveryStatusRetrying)
	}
	if decision.NextRetryAt == nil {
		t.Fatal("NextRetryAt is nil")
	}
	want := now.Add(time.Minute)
	if !decision.NextRetryAt.Equal(want) {
		t.Fatalf("NextRetryAt = %s, want %s", decision.NextRetryAt, want)
	}
}

func TestDecideDeliveryStatusRetryingImmediately(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)

	decision := decideDeliveryStatus(SendResult{ShouldRetry: true, RetryImmediately: true}, 1, 5, now)

	if decision.Status != DeliveryStatusRetrying {
		t.Fatalf("status = %q, want %q", decision.Status, DeliveryStatusRetrying)
	}
	if decision.NextRetryAt == nil {
		t.Fatal("NextRetryAt is nil")
	}
	if !decision.NextRetryAt.Equal(now) {
		t.Fatalf("NextRetryAt = %s, want %s", decision.NextRetryAt, now)
	}
}

func TestDecideDeliveryStatusDeadWhenRetriesExhausted(t *testing.T) {
	decision := decideDeliveryStatus(SendResult{ShouldRetry: true}, 5, 5, time.Now())

	if decision.Status != DeliveryStatusDead {
		t.Fatalf("status = %q, want %q", decision.Status, DeliveryStatusDead)
	}
	if decision.NextRetryAt != nil {
		t.Fatal("NextRetryAt is not nil")
	}
}

func TestDecideDeliveryStatusFailedWithoutRetry(t *testing.T) {
	statusCode := http.StatusBadRequest

	decision := decideDeliveryStatus(SendResult{
		StatusCode:  &statusCode,
		ShouldRetry: false,
	}, 1, 5, time.Now())

	if decision.Status != DeliveryStatusFailed {
		t.Fatalf("status = %q, want %q", decision.Status, DeliveryStatusFailed)
	}
	if decision.NextRetryAt != nil {
		t.Fatal("NextRetryAt is not nil")
	}
}

func TestBackoffDuration(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 0},
		{attempt: 2, want: time.Minute},
		{attempt: 3, want: 5 * time.Minute},
		{attempt: 4, want: 15 * time.Minute},
		{attempt: 5, want: 30 * time.Minute},
		{attempt: 6, want: 30 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(time.Duration(tt.attempt).String(), func(t *testing.T) {
			if got := backoffDuration(tt.attempt); got != tt.want {
				t.Fatalf("backoffDuration(%d) = %s, want %s", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestProcessingLockTTL(t *testing.T) {
	if processingLockTTL != 10*time.Minute {
		t.Fatalf("processingLockTTL = %s, want 10m", processingLockTTL)
	}
}
