package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv            string
	Port              string
	DatabaseURL       string
	HTTPClientTimeout time.Duration
}

func Load() (Config, error) {
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	port := strings.TrimSpace(os.Getenv("PORT"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	timeoutValue := strings.TrimSpace(os.Getenv("HTTP_CLIENT_TIMEOUT_SECONDS"))

	if appEnv == "" {
		return Config{}, errors.New("APP_ENV is required")
	}
	if port == "" {
		return Config{}, errors.New("PORT is required")
	}
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if timeoutValue == "" {
		return Config{}, errors.New("HTTP_CLIENT_TIMEOUT_SECONDS is required")
	}

	timeoutSeconds, err := strconv.Atoi(timeoutValue)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_CLIENT_TIMEOUT_SECONDS must be an integer: %w", err)
	}
	if timeoutSeconds <= 0 {
		return Config{}, errors.New("HTTP_CLIENT_TIMEOUT_SECONDS must be greater than zero")
	}

	return Config{
		AppEnv:            appEnv,
		Port:              port,
		DatabaseURL:       databaseURL,
		HTTPClientTimeout: time.Duration(timeoutSeconds) * time.Second,
	}, nil
}
