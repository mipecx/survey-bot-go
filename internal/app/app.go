package app

import (
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
func Run(botAPI *tgbotapi.BotAPI, repo repository.UserRepository, logger *slog.Logger) {
	adminMap := make(map[int64]bool)
	rawAdmins := os.Getenv("ADMIN_IDS")
	for _, s := range strings.Split(rawAdmins, ",") {
		id, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		adminMap[id] = true
	}
	if len(adminMap) == 0 {
		logger.Warn("No admin IDs provided in ADMIN_IDS environment variable")
	}

	userService := service.NewUserService(repo, logger)

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

	for update := range updates {
		go h.HandleUpdate(update, logger)
		userID := int64(0)
		if update.Message != nil {
			userID = update.Message.From.ID
		} else if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
		}

		logger.Info("Update received", slog.Int64("user_id", userID))
	}
}
