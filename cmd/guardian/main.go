package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Scavenger503/Guardian/internal/config"
	"github.com/Scavenger503/Guardian/internal/notifier"
	"github.com/Scavenger503/Guardian/internal/watcher"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Guardian 🛡️")

	cfg := config.Load()

	if cfg.TelegramToken == "" || cfg.TelegramChatID == "" {
		log.Println("Warning: Telegram credentials not set — notifications disabled")
	}

	n := notifier.New(cfg.TelegramToken, cfg.TelegramChatID)

	w, err := watcher.New(cfg, n)
	if err != nil {
		log.Fatalf("Failed to initialize watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Shutdown signal received")
		cancel()
	}()

	w.Run(ctx)
}
