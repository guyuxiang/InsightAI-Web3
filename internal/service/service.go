package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"aiweb3news/internal/analysis"
	"aiweb3news/internal/config"
	"aiweb3news/internal/rss"
	"aiweb3news/internal/storage"
)

const wecomWebhookURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=74cb55e7-0430-400a-b3e2-2e8d05d8cb06"

// Service ties together RSS polling and AI analysis.
type Service struct {
	fetcher  *rss.Fetcher
	analyzer analysis.Analyzer
	store    *storage.Store
	logger   *log.Logger
	cfg      config.Config
}

// NewService creates a Service instance.
func NewService(fetcher *rss.Fetcher, analyzer analysis.Analyzer, store *storage.Store, logger *log.Logger, cfg config.Config) *Service {
	return &Service{
		fetcher:  fetcher,
		analyzer: analyzer,
		store:    store,
		logger:   logger,
		cfg:      cfg,
	}
}

// Run starts the HTTP server and the polling loop.
func (s *Service) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthHandler)
	mux.HandleFunc("/items", s.itemsHandler)

	srv := &http.Server{
		Addr:    s.cfg.BindAddr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	go func() {
		s.logger.Printf("HTTP server listening on %s", s.cfg.BindAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("http server error: %v", err)
		}
	}()

	// Kick off an initial fetch.
	s.pollOnce(ctx)

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Println("stopping service, context cancelled")
			return nil
		case <-ticker.C:
			s.pollOnce(ctx)
		}
	}
}

func (s *Service) pollOnce(ctx context.Context) {
	s.logger.Println("polling once")
	items, err := s.fetcher.Fetch(ctx)
	if err != nil {
		s.logger.Printf("failed to fetch feed: %v", err)
		return
	}

	for _, item := range items {
		exists, err := s.store.Exists(ctx, item.GUID)
		if err != nil {
			s.logger.Printf("check exists failed for %s: %v", item.GUID, err)
			continue
		}
		if exists {
			continue
		}

		result, err := s.analyzer.Evaluate(ctx, analysis.ItemContext{
			Title:       item.Title,
			Link:        item.Link,
			PublishedAt: item.PublishedAt,
			Summary:     item.Description,
		})
		if err != nil {
			s.logger.Printf("analysis error for %s: %v", item.Title, err)
			continue
		}

		if err := s.store.SaveAnalysis(ctx, item, result); err != nil {
			s.logger.Printf("store analysis failed for %s: %v", item.Title, err)
			continue
		}

		if result.Relevant {
			s.notifyWebhook(ctx, item, result)
		}
	}
}

func (s *Service) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Service) itemsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRelevant(r.Context(), s.cfg.MaxItems)
	if err != nil {
		s.logger.Printf("list relevant failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		Count int                  `json:"count"`
		Items []storage.StoredItem `json:"items"`
	}{
		Count: len(items),
		Items: items,
	}); err != nil {
		s.logger.Printf("write items response failed: %v", err)
	}
}

func (s *Service) notifyWebhook(ctx context.Context, item rss.Item, result analysis.Result) {
	payload := map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": fmt.Sprintf("%s\n分类: %s\nAI分析: %s\n链接: %s", item.Title, result.Category, result.Reason, item.Link),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Printf("marshal webhook payload failed: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wecomWebhookURL, bytes.NewReader(body))
	if err != nil {
		s.logger.Printf("build webhook request failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Printf("send webhook failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		s.logger.Printf("webhook returned non-2xx status: %s", resp.Status)
	}
}
