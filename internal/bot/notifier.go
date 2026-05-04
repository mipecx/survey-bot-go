package bot

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	admins map[int64]bool
	logger *slog.Logger
}

func NewTelegramNotifier(bot *tgbotapi.BotAPI, admins map[int64]bool, logger *slog.Logger) *TelegramNotifier {
	return &TelegramNotifier{bot: bot, admins: admins, logger: logger}
}

func (n *TelegramNotifier) Notify(text string) error {
	if len(n.admins) == 0 {
		n.logger.Warn("Notifier: admin list is empty, no one to notify")
		return nil
	}

	for adminID := range n.admins {
		msg := tgbotapi.NewMessage(adminID, text)
		msg.ParseMode = "HTML"

		_, err := n.bot.Send(msg)
		if err != nil {
			n.logger.Error("Notifier: failed to send notification",
				"admin_id", adminID,
				"error", err,
			)
		} else {
			n.logger.Info("Notifier: admin notified successfully",
				"admin_id", adminID,
			)
		}
	}
	return nil
}
