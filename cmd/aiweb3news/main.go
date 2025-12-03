package main

import (
	"context"
	"log"
	"os"

	"aiweb3news/internal/analysis"
	"aiweb3news/internal/config"
	"aiweb3news/internal/rss"
	"aiweb3news/internal/service"
	"aiweb3news/internal/storage"
)

func main() {
	cfg := config.Load()
	logger := log.New(os.Stdout, "[aiweb3news] ", log.LstdFlags)
	ctx := context.Background()

	if cfg.OpenAIKey == "" {
		logger.Println("warning: OPENAI_API_KEY is not set, analysis calls will fail")
	}

	analyzer := analysis.NewClient(cfg.OpenAIKey, cfg.OpenAIModel, cfg.OpenAIBase, logger)
	fetcher := rss.NewFetcher(cfg.FeedURL, logger)
	store, err := storage.NewMySQLStore(ctx, cfg, logger)
	if err != nil {
		logger.Fatalf("failed to init mysql store: %v", err)
	}
	defer store.Close()

	svc := service.NewService(fetcher, analyzer, store, logger, cfg)

	if err := svc.Run(ctx); err != nil {
		logger.Fatalf("service stopped with error: %v", err)
	}
}
