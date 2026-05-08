package delivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSenderSignsRequest(t *testing.T) {
	body := []byte(`{"payment_id":"pay_123","amount":150000}`)
	secret := "whsec_test_secret"
	fixedTime := time.Unix(1710000000, 0).UTC()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("X-Webhook-Event-Id"); got != "evt_123" {
			t.Fatalf("event id header = %q, want evt_123", got)
		}
		if got := r.Header.Get("X-Webhook-Delivery-Id"); got != "del_123" {
			t.Fatalf("delivery id header = %q, want del_123", got)
		}
		if got := r.Header.Get("X-Webhook-Timestamp"); got != "1710000000" {
			t.Fatalf("timestamp header = %q, want 1710000000", got)
		}

		expectedSignature := testSignature("1710000000", body, secret)
		if got := r.Header.Get("X-Webhook-Signature"); got != expectedSignature {
			t.Fatalf("signature = %q, want %q", got, expectedSignature)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender := NewSender(2 * time.Second)
	sender.now = func() time.Time { return fixedTime }

	result := sender.Send(context.Background(), SendRequest{
		DeliveryID:       "del_123",
		EventID:          "evt_123",
		SubscriberURL:    server.URL,
		SubscriberSecret: secret,
		Payload:          body,
	})

	if result.ErrorMessage != nil {
		t.Fatalf("ErrorMessage = %q, want nil", *result.ErrorMessage)
	}
	if result.StatusCode == nil || *result.StatusCode != http.StatusNoContent {
		t.Fatalf("StatusCode = %v, want %d", result.StatusCode, http.StatusNoContent)
	}
	if result.ShouldRetry {
		t.Fatal("ShouldRetry = true, want false")
	}
}

func TestSenderRetryDecisionByStatus(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		shouldRetry bool
	}{
		{name: "success", statusCode: http.StatusOK, shouldRetry: false},
		{name: "bad request", statusCode: http.StatusBadRequest, shouldRetry: false},
		{name: "unauthorized", statusCode: http.StatusUnauthorized, shouldRetry: false},
		{name: "forbidden", statusCode: http.StatusForbidden, shouldRetry: false},
		{name: "not found", statusCode: http.StatusNotFound, shouldRetry: false},
		{name: "too many requests", statusCode: http.StatusTooManyRequests, shouldRetry: true},
		{name: "server error", statusCode: http.StatusInternalServerError, shouldRetry: true},
		{name: "bad gateway", statusCode: http.StatusBadGateway, shouldRetry: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryStatus(tt.statusCode); got != tt.shouldRetry {
				t.Fatalf("shouldRetryStatus(%d) = %v, want %v", tt.statusCode, got, tt.shouldRetry)
			}
		})
	}
}

func TestSenderRetriesOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(1 * time.Millisecond)
	result := sender.Send(context.Background(), SendRequest{
		DeliveryID:       "del_123",
		EventID:          "evt_123",
		SubscriberURL:    server.URL,
		SubscriberSecret: "whsec_test_secret",
		Payload:          []byte(`{"ok":true}`),
	})

	if result.ErrorMessage == nil {
		t.Fatal("ErrorMessage = nil, want timeout error")
	}
	if !result.ShouldRetry {
		t.Fatal("ShouldRetry = false, want true")
	}
}

func TestSenderTruncatesResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("a", storedResponseBodyLimit+100)))
	}))
	defer server.Close()

	sender := NewSender(2 * time.Second)
	result := sender.Send(context.Background(), SendRequest{
		DeliveryID:       "del_123",
		EventID:          "evt_123",
		SubscriberURL:    server.URL,
		SubscriberSecret: "whsec_test_secret",
		Payload:          []byte(`{"ok":true}`),
	})

	if result.ResponseBody == nil {
		t.Fatal("ResponseBody = nil, want value")
	}
	if len(*result.ResponseBody) != storedResponseBodyLimit {
		t.Fatalf("response body length = %d, want %d", len(*result.ResponseBody), storedResponseBodyLimit)
	}
	if !result.ShouldRetry {
		t.Fatal("ShouldRetry = false, want true")
	}
}

func testSignature(timestamp string, body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
