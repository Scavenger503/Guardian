package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	TelegramToken  string
	TelegramChatID string
	PollInterval   time.Duration
	Label          string
}

func Load() *Config {
	interval := 300
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			interval = i
		}
	}

	return &Config{
		TelegramToken:  os.Getenv("TELEGRAM_TOKEN"),
		TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
		PollInterval:   time.Duration(interval) * time.Second,
		Label:          "guardian.enable",
	}
}
