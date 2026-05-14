package bot

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramNotifier delivers admin notifications via the Telegram Bot API.
// It implements the service.AdminNotifier interface.
// All configured admins receive every notification independently -
// a delivery failure for one admin does not prevent delivery to others.
type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	admins map[int64]bool
	logger *slog.Logger
}

// NewTelegramNotifier creates a TelegramNotifier that broadcasts to all adminIDs.
func NewTelegramNotifier(bot *tgbotapi.BotAPI, admins map[int64]bool, logger *slog.Logger) *TelegramNotifier {
	return &TelegramNotifier{bot: bot, admins: admins, logger: logger}
}

// Notify sends an HTML-formatted message to every configured admin.
// If the admin list is empty the call is a no-op.
// Errors per admin are logged but do not abort delivery to remaining admins.
// Always returns nil - per-admin errors are handled internally.
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

// NotifyUser sends an HTML-formatted message to a single Telegram user.
// Unlike Notify, this targets an arbitrary user ID rather than the admin list.
func (n *TelegramNotifier) NotifyUser(tgID int64, text string, photoFileID string) error {
	if photoFileID != "" {
		photo := tgbotapi.NewPhoto(tgID, tgbotapi.FileID(photoFileID))
		photo.Caption = text
		photo.ParseMode = "HTML"
		_, err := n.bot.Send(photo)
		return err
	}
	msg := tgbotapi.NewMessage(tgID, text)
	msg.ParseMode = "HTML"
	_, err := n.bot.Send(msg)
	return err
}
