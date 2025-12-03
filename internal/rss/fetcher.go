package rss

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// Item represents a normalized RSS entry.
type Item struct {
	GUID        string
	Title       string
	Link        string
	PublishedAt time.Time
	Description string
}

// Fetcher pulls and parses RSS feeds.
type Fetcher struct {
	feedURL string
	parser  *gofeed.Parser
	logger  *log.Logger
}

// NewFetcher creates an RSS fetcher for a single feed.
func NewFetcher(feedURL string, logger *log.Logger) *Fetcher {
	return &Fetcher{
		feedURL: feedURL,
		parser:  gofeed.NewParser(),
		logger:  logger,
	}
}

// Fetch pulls the feed and returns the parsed items.
func (f *Fetcher) Fetch(ctx context.Context) ([]Item, error) {
	feed, err := f.parser.ParseURLWithContext(f.feedURL, ctx)
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(feed.Items))
	for _, entry := range feed.Items {
		pubTime := time.Now()
		if entry.PublishedParsed != nil {
			pubTime = *entry.PublishedParsed
		}
		guid := pickGUID(entry)
		// Only process newsletter items; skip others early.
		if !strings.Contains(guid, "/newsletter/") && !strings.Contains(entry.Link, "/newsletter/") {
			continue
		}
		items = append(items, Item{
			GUID:        guid,
			Title:       entry.Title,
			Link:        entry.Link,
			PublishedAt: pubTime,
			Description: entry.Description,
		})
	}
	return items, nil
}

func pickGUID(entry *gofeed.Item) string {
	if entry.GUID != "" {
		return entry.GUID
	}
	if entry.Link != "" {
		return entry.Link
	}
	return entry.Title
}
