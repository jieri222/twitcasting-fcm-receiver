package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"twitcasting-fcm-receiver/internal/fcm"
	"twitcasting-fcm-receiver/internal/log"
	"twitcasting-fcm-receiver/internal/subscriptions"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

func main() {
	verbose := pflag.BoolP("verbose", "v", false, "Verbose output")
	listSubscriptions := pflag.BoolP("list-subs", "l", false, "List subscriptions")
	pflag.Parse()

	log.InitLogger(*verbose)
	godotenv.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *listSubscriptions {
		tc := subscriptions.NewClient()
		if err := tc.ListSubscriptions(ctx); err != nil {
			log.Error("failed to list subscriptions", "error", err)
			os.Exit(1)
		}
		return
	}

	url := os.Getenv("PUSH_URL")
	if url == "" {
		url = "wss://push.services.mozilla.com/"
	}
	s, err := fcm.LoadState()
	if err != nil {
		log.Error("failed to load state", "error", err)
		os.Exit(1)
	}

	ready := make(chan struct{}, 1)
	go func() {
		delay := time.Second
		for ctx.Err() == nil {
			start := time.Now()
			log.Info("connecting to push service")
			err := fcm.RunSession(ctx, s, url, ready)
			log.Info("session ended", "reason", err)
			if time.Since(start) > time.Minute {
				delay = time.Second // 連線穩定過就重置退避
			}
			select {
			case <-ctx.Done():
			case <-time.After(delay):
			}
			if delay *= 2; delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
		}
	}()

	<-ctx.Done()
}
