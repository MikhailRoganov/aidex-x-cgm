package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/brkss/aidex-x-cgm/config"
	"github.com/brkss/aidex-x-cgm/internal/adapter/aidex"
	"github.com/brkss/aidex-x-cgm/internal/adapter/nightscout"
	"github.com/brkss/aidex-x-cgm/internal/adapter/scheduler"
	"github.com/brkss/aidex-x-cgm/internal/application"
)

func main() {
	cfg := config.Load()

	if cfg.AiDEX.UserID == "" || cfg.AiDEX.Token == "" {
		log.Fatal("AIDEX_USER_ID and AIDEX_TOKEN must be set")
	}
	if cfg.Nightscout.APISecret == "" {
		log.Fatal("NIGHTSCOUT_API_SECRET must be set")
	}

	log.Printf("CGM Scheduler starting")
	log.Printf("  AiDEX user:     %s", cfg.AiDEX.UserID)
	log.Printf("  Nightscout URL: %s", cfg.Nightscout.URL)
	log.Printf("  Poll interval:  %v", cfg.PollInterval)

	cgmClient := aidex.NewClient(cfg.AiDEX)

	nsClient := nightscout.NewClient(cfg.Nightscout)
	syncSvc := application.NewSyncService(cgmClient, nsClient)


	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	scheduler.Start(ctx, cfg.PollInterval, syncSvc.Sync)
}
