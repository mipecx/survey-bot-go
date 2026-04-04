package bot

import (
	"context"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mipecx/survey-bot-go/internal/service"
)

type Handler struct {
	Bot     *tgbotapi.BotAPI
	Admins  map[int64]bool
	Service service.UserService
	Logger  *slog.Logger
}

func (h *Handler) HandleUpdate(update tgbotapi.Update, logger *slog.Logger) {
	var chatID int64
	var text string
	var username string

	if update.Message != nil {
		if update.Message.IsCommand() && update.Message.Text != "/start" {
			return
		}
		chatID = update.Message.Chat.ID
		text = update.Message.Text
		username = update.Message.From.UserName

	} else if update.CallbackQuery != nil {
		chatID = update.CallbackQuery.Message.Chat.ID
		text = update.CallbackQuery.Data
		username = update.CallbackQuery.From.UserName

		callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		h.Bot.Send(callbackCfg)
	} else {
		return
	}

	h.reply(chatID, username, text, logger)
}

func (h *Handler) reply(chatID int64, username, text string, logger *slog.Logger) {
	resp, err := h.Service.ProcessMessage(context.Background(), chatID, username, text)
	if err != nil {
		logger.Error("service error", "err", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, resp.Text)

	if len(resp.Buttons) > 0 {
		msg.ReplyMarkup = makeInlineKeyboard(resp.Buttons)
	}

	if _, err := h.Bot.Send(msg); err != nil {
		logger.Error("failed to send message", "err", err)
	}
}
