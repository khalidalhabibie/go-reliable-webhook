package main

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
)

func TestHealth(t *testing.T) {
	app := newApp(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)

	req, err := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("X-Request-ID", "test-request-id")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
