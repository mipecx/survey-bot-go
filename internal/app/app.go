package app

import (
	"context"
	"log/slog"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/mipecx/survey-bot-go/internal/bot"
	"github.com/mipecx/survey-bot-go/internal/config"
	"github.com/mipecx/survey-bot-go/internal/repository"
	"github.com/mipecx/survey-bot-go/internal/service"
)

// Run initializes application components, sets up administrative access,
// and starts the main update loop to process incoming messages.
func Run(ctx context.Context, botAPI *tgbotapi.BotAPI, repo repository.UserRepository, logger *slog.Logger, cfg *config.Config) {
	notifier := bot.NewTelegramNotifier(botAPI, cfg.AdminIDs, logger)

	userService := service.NewUserService(repo, logger, notifier, cfg)

	var wg sync.WaitGroup

	h := &bot.Handler{
		Bot:            botAPI,
		Admins:         cfg.AdminIDs,
		Service:        userService,
		Logger:         logger,
		WG:             &wg,
		CommunityURL:   cfg.CommunityURL,
		WelcomeImageID: cfg.WeclomeImageID,
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := botAPI.GetUpdatesChan(u)
	logger.Info("Bot started, waiting for updates...")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping update loop...")
			botAPI.StopReceivingUpdates()
			wg.Wait()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}

			wg.Add(1)
			go func(upd tgbotapi.Update) {
				defer wg.Done()
				h.HandleUpdate(upd)
			}(update)
		}
	}
}
