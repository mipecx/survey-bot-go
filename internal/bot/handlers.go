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

func (h *Handler) extractTGID(update tgbotapi.Update) int64 {
	if update.Message != nil {
		return update.Message.From.ID
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	return 0
}

// HandleUpdate routes the update to be handled in callback or message handler
func (h *Handler) HandleUpdate(update tgbotapi.Update) {
	ctx := context.Background()
	tgID := h.extractTGID(update)

	lock, _ := h.userLocks.LoadOrStore(tgID, &sync.Mutex{})
	mtx := lock.(*sync.Mutex)
	mtx.Lock()
	defer mtx.Unlock()

	if update.CallbackQuery != nil {
		h.handleCallback(ctx, update.CallbackQuery)
		return
	}

	if update.Message != nil {
		h.handleMessage(ctx, update.Message)
	}
}

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	var resp *service.UserResponse
	var err error

	userID := msg.From.ID
	username := msg.From.UserName
	chatID := msg.Chat.ID

	switch {
	case msg.Contact != nil:
		resp, err = h.Service.ProcessMessage(ctx, userID, username, msg.Contact.PhoneNumber)
		if err != nil {
			h.Logger.Error("error during phone collection", "user_id", userID, "error", err)
		}
	case msg.IsCommand():
		if msg.Command() == "start" {
			resp, err = h.Service.ProcessMessage(ctx, userID, username, "/start")
			if err != nil {
				h.Logger.Error("error during /start command", "user_id", userID, "error", err)
			}
		} else {
			return
		}
	case msg.Text != "":
		resp, err = h.Service.ProcessMessage(ctx, userID, username, msg.Text)
		if err != nil {
			h.Logger.Error("error during text processing", "user_id", userID, "error", err)
		}
	default:
		return
	}
	h.sendResponse(chatID, resp)
}

func (h *Handler) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	data := callback.Data
	userID := callback.From.ID
	username := callback.From.UserName

	callbackCfg := tgbotapi.NewCallback(callback.ID, "")
	if _, err := h.Bot.Request(callbackCfg); err != nil {
		h.Logger.Error("failed to answer callback query", "chat_id", chatID, "error", err)
	}

	resp, err := h.Service.ProcessCallback(ctx, userID, username, data)
	if err != nil {
		h.Logger.Error("callback error", "err", err)
		return
	}

	// ФИКС: Передаем ID сообщения из колбэка в структуру ответа
	if resp != nil {
		resp.MessageID = callback.Message.MessageID
	}

	h.sendResponse(chatID, resp)
}

// sendResponse sends a text message to the given chat, with an inline keyboard if buttons are provided.
func (h *Handler) sendResponse(chatID int64, resp *service.UserResponse) {
	if resp == nil {
		return
	}

	var err error

	// Проверяем: если это Callback (MessageID != 0) и тип ввода НЕ телефон
	// (Потому что Reply Keyboard нельзя прикрепить к EditMessage)
	if resp.MessageID != 0 && resp.InputType != service.InputPhone {
		edit := tgbotapi.NewEditMessageText(chatID, resp.MessageID, resp.Text)
		edit.ParseMode = "HTML"

		if len(resp.Buttons) > 0 {
			markup := makeInlineKeyboard(resp.StepID, resp.Buttons)
			edit.ReplyMarkup = &markup
		}

		_, err = h.Bot.Send(edit)

		// Если вдруг сообщение уже удалено или оно такое же (Telegram выдает ошибку),
		// можно зафолбэчиться на отправку нового, но пока оставим логирование
	} else {
		// Шлем новое сообщение (для команд, обычного текста или сбора телефона)
		msg := tgbotapi.NewMessage(chatID, resp.Text)
		msg.ParseMode = "HTML"

		if resp.InputType == service.InputPhone {
			msg.ReplyMarkup = makeReplyKeyboard()
		} else if len(resp.Buttons) > 0 {
			msg.ReplyMarkup = makeInlineKeyboard(resp.StepID, resp.Buttons)
		} else {
			msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		}

		_, err = h.Bot.Send(msg)
	}

	if err != nil {
		h.Logger.Error("failed to process message response", "chat_id", chatID, "error", err)
	}

	// Отправка подарка (PDF) всегда идет отдельным сообщением после текста
	if resp.Document != "" {
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FileID(resp.Document))
		if _, err := h.Bot.Send(doc); err != nil {
			h.Logger.Error("failed to send gift document", "chat_id", chatID, "error", err)
		}
	}
}
