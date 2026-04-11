package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/mipecx/survey-bot-go/internal/app"
	"github.com/mipecx/survey-bot-go/internal/repository/postgres"
)

// main initializes the environment, establishes a database connection,
// and starts the application bot logic.
func main() {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

	logger := slog.New(handler)
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Warn("Warning: .env file not found, using system variables", "error", err)
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		slog.Error("BOT_TOKEN not set in .env")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL not set in .env")
		os.Exit(1)
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		slog.Error("Error creating Bot API", "error", err)
		os.Exit(1)
	}
	slog.Info("Authorized under account", "username", bot.Self.UserName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := postgres.New(ctx, dsn)
	if err != nil {
		slog.Error("Error connecting to the database", "error", err)
		os.Exit(1)
	}
	go app.Run(ctx, bot, repo, logger)
	slog.Info("Bot is running, Press Ctrl+C to stop.")
	<-ctx.Done()
	slog.Info("Shutting down gracefully...")
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := repo.Close(); err != nil {
		slog.Error("Error closing database", "error", err)
	}
	slog.Info("Bot gracefully stopped.")
}
