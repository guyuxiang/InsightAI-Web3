package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

const (
	defaultFeedURL      = "https://www.techflowpost.com/rss.aspx"
	defaultPollMinutes  = 15
	defaultBindAddr     = ":8082"
	defaultOpenAIModel  = "gpt-4o"
	defaultOpenAIBase   = "https://aigateway.hrlyit.com/v1"
	defaultMaxItemStore = 50
	defaultDBHost       = "mysql01.dev.lls.com"
	defaultDBPort       = 4120
	defaultDBUser       = "root"
	defaultDBPass       = "123456"
	defaultDBName       = "aiweb3news"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	FeedURL      string
	PollInterval time.Duration
	BindAddr     string
	OpenAIKey    string
	OpenAIModel  string
	OpenAIBase   string
	MaxItems     int
	DBHost       string
	DBPort       int
	DBUser       string
	DBPass       string
	DBName       string
}

// Load reads environment variables, filling in reasonable defaults.
func Load() Config {
	return Config{
		FeedURL:      stringWithDefault("FEED_URL", defaultFeedURL),
		PollInterval: durationFromMinutes("POLL_INTERVAL_MINUTES", defaultPollMinutes),
		BindAddr:     stringWithDefault("BIND_ADDR", defaultBindAddr),
		OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  stringWithDefault("OPENAI_MODEL", defaultOpenAIModel),
		OpenAIBase:   stringWithDefault("OPENAI_BASE_URL", defaultOpenAIBase),
		MaxItems:     intWithDefault("MAX_ITEMS", defaultMaxItemStore),
		DBHost:       stringWithDefault("DB_HOST", defaultDBHost),
		DBPort:       intWithDefault("DB_PORT", defaultDBPort),
		DBUser:       stringWithDefault("DB_USER", defaultDBUser),
		DBPass:       stringWithDefault("DB_PASSWORD", defaultDBPass),
		DBName:       stringWithDefault("DB_NAME", defaultDBName),
	}
}

func stringWithDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationFromMinutes(key string, fallback int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if minutes, err := strconv.Atoi(v); err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute
		}
		log.Printf("invalid %s=%s, using default %d minutes", key, v, fallback)
	}
	return time.Duration(fallback) * time.Minute
}

func intWithDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			return parsed
		}
		log.Printf("invalid %s=%s, using default %d", key, v, fallback)
	}
	return fallback
}
