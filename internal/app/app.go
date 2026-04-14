package app

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mipecx/survey-bot-go/internal/bot"
	"github.com/mipecx/survey-bot-go/internal/repository"
	"github.com/mipecx/survey-bot-go/internal/service"
)

// Run initializes application components, sets up administrative access,
// and starts the main update loop to process incoming messages.
func Run(ctx context.Context, botAPI *tgbotapi.BotAPI, repo repository.UserRepository, logger *slog.Logger) {
	adminMap := make(map[int64]bool)
	rawAdmins := os.Getenv("ADMIN_IDS")

	for _, s := range strings.Split(rawAdmins, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			logger.Error("Failed to parse Admin_ID", "value", s, "err", err)
			continue
		}
		adminMap[id] = true
	}

	if len(adminMap) == 0 {
		logger.Warn("No admin IDs provided in ADMIN_IDS environment variable")
	}

	notifier := bot.NewTelegramNotifier(botAPI, adminMap)

	userService := service.NewUserService(repo, logger, notifier, adminMap)

	h := &bot.Handler{
		Bot:     botAPI,
		Admins:  adminMap,
		Service: userService,
		Logger:  logger,
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := botAPI.GetUpdatesChan(u)
	logger.Info("Bot started, waiting for updates...")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping update loop...")
			return
		case update, ok := <-updates:
			if !ok {
				logger.Error("Update channel closed unexpectedly")
				return
			}

			var (
				userID   int64
				username string
			)
			if update.Message != nil {
				userID = update.Message.From.ID
				username = update.Message.From.UserName
			} else if update.CallbackQuery != nil {
				userID = update.CallbackQuery.From.ID
				username = update.CallbackQuery.From.UserName
			}

			if userID != 0 {
				logger.Info("Update received",
					slog.Int64("user_id", userID),
					slog.String("username", username))
			}

			go h.HandleUpdate(update)
		}
	}
}
