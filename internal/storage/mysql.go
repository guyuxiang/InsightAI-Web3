package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"aiweb3news/internal/analysis"
	"aiweb3news/internal/config"
	"aiweb3news/internal/rss"

	_ "github.com/go-sql-driver/mysql"
)

// Store persists analysis results to MySQL.
type Store struct {
	db     *sql.DB
	logger *log.Logger
}

// StoredItem represents a row from the database.
type StoredItem struct {
	GUID        string
	Title       string
	Link        string
	PublishedAt time.Time
	Category    string
	Reason      string
	Tags        []string
	Relevant    bool
}

// NewMySQLStore creates the database (if needed), ensures schema, and returns a ready store.
func NewMySQLStore(ctx context.Context, cfg config.Config, logger *log.Logger) (*Store, error) {
	rootDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=true&loc=Local", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort)
	rootDB, err := sql.Open("mysql", rootDSN)
	if err != nil {
		return nil, fmt.Errorf("open root mysql connection: %w", err)
	}
	if err := rootDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping root mysql: %w", err)
	}
	createDB := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.DBName)
	if _, err := rootDB.ExecContext(ctx, createDB); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}
	_ = rootDB.Close()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql with db: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql with db: %w", err)
	}

	store := &Store{db: db, logger: logger}
	if err := store.ensureSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ensureSchema(ctx context.Context) error {
	const createTable = `
CREATE TABLE IF NOT EXISTS news_analysis (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	guid VARCHAR(255) NOT NULL UNIQUE,
	title TEXT NOT NULL,
	link TEXT,
	published_at DATETIME NULL,
	summary TEXT,
	relevant TINYINT(1) NOT NULL,
	category VARCHAR(255),
	reason TEXT,
	tags TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`
	_, err := s.db.ExecContext(ctx, createTable)
	if err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}

// Exists reports whether a guid has already been processed.
func (s *Store) Exists(ctx context.Context, guid string) (bool, error) {
	row := s.db.QueryRowContext(ctx, "SELECT 1 FROM news_analysis WHERE guid = ? LIMIT 1", guid)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// SaveAnalysis stores or updates an analyzed item.
func (s *Store) SaveAnalysis(ctx context.Context, item rss.Item, result analysis.Result) error {
	tagsJSON, _ := json.Marshal(result.Tags)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO news_analysis (guid, title, link, published_at, summary, relevant, category, reason, tags)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	title=VALUES(title),
	link=VALUES(link),
	published_at=VALUES(published_at),
	summary=VALUES(summary),
	relevant=VALUES(relevant),
	category=VALUES(category),
	reason=VALUES(reason),
	tags=VALUES(tags),
	updated_at=CURRENT_TIMESTAMP
`, item.GUID, item.Title, item.Link, item.PublishedAt, item.Description, result.Relevant, result.Category, result.Reason, string(tagsJSON))
	if err != nil {
		return fmt.Errorf("save analysis: %w", err)
	}
	return nil
}

// ListRelevant returns the most recent relevant items.
func (s *Store) ListRelevant(ctx context.Context, limit int) ([]StoredItem, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT guid, title, link, published_at, category, reason, tags, relevant
FROM news_analysis
WHERE relevant = 1
ORDER BY published_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list relevant: %w", err)
	}
	defer rows.Close()

	var items []StoredItem
	for rows.Next() {
		var (
			item   StoredItem
			tags   sql.NullString
			pub    sql.NullTime
			relInt int
		)
		if err := rows.Scan(&item.GUID, &item.Title, &item.Link, &pub, &item.Category, &item.Reason, &tags, &relInt); err != nil {
			return nil, err
		}
		if pub.Valid {
			item.PublishedAt = pub.Time
		}
		item.Relevant = relInt == 1
		if tags.Valid && tags.String != "" {
			var parsed []string
			if err := json.Unmarshal([]byte(tags.String), &parsed); err == nil {
				item.Tags = parsed
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
