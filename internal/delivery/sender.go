package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const storedResponseBodyLimit = 4 * 1024

type SendRequest struct {
	DeliveryID       string
	EventID          string
	SubscriberURL    string
	SubscriberSecret string
	Payload          []byte
}

type SendResult struct {
	StatusCode   *int
	ResponseBody *string
	DurationMS   int
	ErrorMessage *string
	ShouldRetry  bool
}

type Sender struct {
	client *http.Client
	now    func() time.Time
}

func NewSender(timeout time.Duration) *Sender {
	return &Sender{
		client: &http.Client{Timeout: timeout},
		now:    time.Now,
	}
}

func (s *Sender) Send(ctx context.Context, req SendRequest) SendResult {
	start := time.Now()
	timestamp := fmt.Sprintf("%d", s.now().UTC().Unix())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.SubscriberURL, bytes.NewReader(req.Payload))
	if err != nil {
		return errorResult(start, err, false)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Webhook-Event-Id", req.EventID)
	httpReq.Header.Set("X-Webhook-Delivery-Id", req.DeliveryID)
	httpReq.Header.Set("X-Webhook-Timestamp", timestamp)
	httpReq.Header.Set("X-Webhook-Signature", signPayload(timestamp, req.Payload, req.SubscriberSecret))

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return errorResult(start, err, shouldRetryError(err))
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, storedResponseBodyLimit+1))
	if readErr != nil {
		return errorResult(start, readErr, true)
	}

	statusCode := resp.StatusCode
	responseBody := truncateBytesToString(body, storedResponseBodyLimit)

	return SendResult{
		StatusCode:   &statusCode,
		ResponseBody: &responseBody,
		DurationMS:   durationMS(start),
		ShouldRetry:  shouldRetryStatus(resp.StatusCode),
	}
}

func signPayload(timestamp string, rawBody []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func shouldRetryStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func shouldRetryError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return true
}

func errorResult(start time.Time, err error, shouldRetry bool) SendResult {
	message := err.Error()
	return SendResult{
		DurationMS:   durationMS(start),
		ErrorMessage: &message,
		ShouldRetry:  shouldRetry,
	}
}

func durationMS(start time.Time) int {
	return int(time.Since(start).Milliseconds())
}

func truncateBytesToString(value []byte, limit int) string {
	if len(value) <= limit {
		return string(value)
	}
	return string(value[:limit])
}
