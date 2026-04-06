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
	ctx := context.Background()

	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		data := update.CallbackQuery.Data
		userID := update.CallbackQuery.From.ID

		callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		h.Bot.Send(callbackCfg)

		resp, err := h.Service.ProcessCallback(ctx, userID, data)
		if err != nil {
			logger.Error("callback error", "err", err)
			return
		}
		h.sendResponse(chatID, resp)
		return
	}
	if update.Message != nil {
		if update.Message.IsCommand() && update.Message.Text != "/start" {
			return
		}

		resp, err := h.Service.ProcessMessage(ctx, update.Message.From.ID, update.Message.From.UserName, update.Message.Text)
		if err != nil {
			logger.Error("service error", "err", err)
			return
		}
		h.sendResponse(update.Message.Chat.ID, resp)
	}

}

func (h *Handler) sendResponse(chatID int64, resp *service.UserResponse) {
	msg := tgbotapi.NewMessage(chatID, resp.Text)
	if len(resp.Buttons) > 0 {
		msg.ReplyMarkup = makeInlineKeyboard(resp.Buttons)
	}
	h.Bot.Send(msg)
}
