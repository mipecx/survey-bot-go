package bot

import (
	"context"
	"log/slog"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mipecx/survey-bot-go/internal/service"
)

// Hander processes incoming Telegram updates and delegates business logic to Service.
// userLocks ensures that concurrent updates from the same user are processed sequentially.
type Handler struct {
	Bot       *tgbotapi.BotAPI
	Admins    map[int64]bool
	Service   service.UserService
	Logger    *slog.Logger
	userLocks sync.Map
}

// HandleUpdate routes the update to be handled in callback or message handler
func (h *Handler) HandleUpdate(update tgbotapi.Update) {
	ctx := context.Background()
	var tgID int64
	if update.Message != nil {
		tgID = update.Message.From.ID
	}
	if update.CallbackQuery != nil {
		tgID = update.CallbackQuery.From.ID
	}

	lock, _ := h.userLocks.LoadOrStore(tgID, &sync.Mutex{})

	mtx := lock.(*sync.Mutex)
	mtx.Lock()
	defer mtx.Unlock()

	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		data := update.CallbackQuery.Data
		userID := update.CallbackQuery.From.ID

		callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		if _, err := h.Bot.Send(callbackCfg); err != nil {
			h.Logger.Error("failed to answer callback query", "chat_id", chatID, "error", err)
		}

		resp, err := h.Service.ProcessCallback(ctx, userID, update.CallbackQuery.From.UserName, data)
		if err != nil {
			h.Logger.Error("callback error", "err", err)
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
			h.Logger.Error("service error", "err", err)
			return
		}
		h.sendResponse(update.Message.Chat.ID, resp)
	}
}

// sendResponse sends a text message to the given chat, with an inline keyboard if buttons are provided.
func (h *Handler) sendResponse(chatID int64, resp *service.UserResponse) {
	if resp == nil {
		return
	}
	msg := tgbotapi.NewMessage(chatID, resp.Text)
	if len(resp.Buttons) > 0 {
		msg.ReplyMarkup = makeInlineKeyboard(resp.Buttons)
	}
	if _, err := h.Bot.Send(msg); err != nil {
		h.Logger.Error("failed to send message", "chat_id", chatID, "error", err)
	}
}
