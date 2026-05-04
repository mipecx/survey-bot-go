package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mipecx/survey-bot-go/internal/app"
	"github.com/mipecx/survey-bot-go/internal/config"
	"github.com/mipecx/survey-bot-go/internal/repository/postgres"
)

// main initializes the environment, establishes a database connection,
// and starts the application bot logic.
func main() {
	cfg := config.MustLoad()

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     cfg.LogLevel,
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		logger.Error("Failed to create Bot API", "error", err)
		os.Exit(1)
	}
	logger.Info("Authorized", "username", bot.Self.UserName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := postgres.New(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("Failed to connect to DB", "error", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("Closing database connection...")
		if err := repo.Close(); err != nil {
			logger.Error("Error closing database", "error", err)
		}
	}()

	app.Run(ctx, bot, repo, logger, cfg)

	logger.Info("Bot gracefully stopped.")
}
