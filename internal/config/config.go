package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken       string
	DatabaseURL    string
	AdminIDs       map[int64]bool
	CommunityURL   string
	GiftFileID     string
	WeclomeImageID string
	LogLevel       slog.Level
}

func MustLoad() *Config {
	if err := godotenv.Load(); err != nil {
		slog.Warn(".env file not found, using system variables")
	}

	cfg := &Config{
		BotToken:       os.Getenv("BOT_TOKEN"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		CommunityURL:   getEnv("COMMUNITY_URL", "https://t.me/default_group"),
		GiftFileID:     os.Getenv("GIFT_FILE_ID"),
		LogLevel:       parseLogLevel(os.Getenv("LOG_LEVEL")),
		AdminIDs:       make(map[int64]bool),
		WeclomeImageID: os.Getenv("WELCOME_IMAGE_ID"),
	}

	rawAdmins := os.Getenv("ADMIN_IDS")
	for s := range strings.SplitSeq(rawAdmins, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			slog.Error("Failed to parse admin ID", "value", s, "error", err)
			continue
		}
		cfg.AdminIDs[id] = true
	}

	if cfg.BotToken == "" || cfg.DatabaseURL == "" {
		slog.Error("Missing critical config: BOT_TOKEN or DATABASE_URL")
		os.Exit(1)
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
