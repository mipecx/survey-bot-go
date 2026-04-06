package main

import (
	"context"
	"log/slog"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/mipecx/survey-bot-go/internal/app"
	"github.com/mipecx/survey-bot-go/internal/repository/postgres"
)

// main initializes the environment, establishes a database connection,
// and starts the application bot logic.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
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

	ctx := context.Background()
	repo, err := postgres.New(ctx, dsn)
	if err != nil {
		slog.Error("Error connecting to database", "error", err)
		os.Exit(1)
	}

	app.Run(bot, repo, logger)
}
