// Package bot implements the Telegram update handler layer.
// It receives updates from the Telegram Bot API, enriches each request
// with a per-update context (request_id, user_id, logger), and delegates
// business logic to the service layer.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mipecx/survey-bot-go/internal/ctxlog"
	"github.com/mipecx/survey-bot-go/internal/service"
)

// Handler processes incoming Telegram updates and delegates business logic to Service.
// Concurrent updates from the same user are serialised via userLocks to prevent
// race conditions on shared survey state.
// lastBotMsg tracks the most recently sent bot message ID per chat,
// used for in-place message editing.
type Handler struct {
	Bot            *tgbotapi.BotAPI
	Admins         map[int64]bool
	Service        service.UserService
	Logger         *slog.Logger
	userLocks      sync.Map
	WG             *sync.WaitGroup
	lastBotMsg     sync.Map
	CommunityURL   string
	WelcomeImageID string
}

// extractTGID returns the Telegram user ID from any supported update type.
// Returns 0 for unsupported update types.
func (h *Handler) extractTGID(update tgbotapi.Update) int64 {
	if update.Message != nil {
		return update.Message.From.ID
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	return 0
}

// HandleUpdate is the top-level entry point for all incoming Telegram updates.
// It creates a per-request context with a unique request_id and structured logger,
// acquires a per-user mutex to serialise concurrent updates, then routes to
// handleCallback or handleMessage.
func (h *Handler) HandleUpdate(update tgbotapi.Update) {
	tgID := h.extractTGID(update)

	requestID := fmt.Sprintf("%d-%d", tgID, time.Now().UnixNano())
	ctx := context.WithValue(context.Background(), ctxlog.CtxKeyRequestID, requestID)
	ctx = context.WithValue(ctx, ctxlog.CtxKeyUserID, tgID)
	logger := h.Logger.With("request_id", requestID, "user_id", tgID)
	ctx = context.WithValue(ctx, ctxlog.CtxKeyLogger, logger)

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

// handleMessage processes text messages, commands, and contact shares.
// For text input during an active survey step, it deletes the user's message
// when the bot needs to edit its previous message in place (validation errors).
func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	logger := ctxlog.LoggerFromCtx(ctx, h.Logger)
	var resp *service.UserResponse
	var err error

	userID := msg.From.ID
	username := msg.From.UserName
	chatID := msg.Chat.ID

	switch {
	case msg.Contact != nil:
		resp, err = h.Service.ProcessMessage(ctx, userID, username, msg.Contact.PhoneNumber)
		if err != nil {
			logger.Error("Error during phone collection", "error", err)
		}
		if resp != nil && resp.Edit {
			if msgID, ok := h.lastBotMsg.Load(chatID); ok {
				resp.MessageID = msgID.(int)
			}
			h.Bot.Send(tgbotapi.NewDeleteMessage(chatID, msg.MessageID))
		}
	case msg.IsCommand():
		switch msg.Command() {
		case "start":
			resp, err = h.Service.ProcessMessage(ctx, userID, username, "/start")
			if err != nil {
				logger.Error("Error during /start command", "error", err)
			}
		case "broadcast":
			if !h.Admins[userID] {
				return
			}
			text := msg.CommandArguments()
			if text == "" {
				h.Bot.Send(tgbotapi.NewMessage(chatID, "Использование: /broadcast <текст>"))
				return
			}
			sent, failed := h.Service.Broadcast(ctx, text)
			h.Bot.Send(tgbotapi.NewMessage(chatID,
				fmt.Sprintf("✅ Отправлено: %d\n❌ Ошибок: %d", sent, failed)))
			return
		default:
			return
		}
	case msg.Text != "":
		resp, err = h.Service.ProcessMessage(ctx, userID, username, msg.Text)
		if err != nil {
			logger.Error("Error during text processing", "error", err)
		}
		if resp != nil && resp.Edit {
			if msgID, ok := h.lastBotMsg.Load(chatID); ok {
				resp.MessageID = msgID.(int)
			}
			h.Bot.Send(tgbotapi.NewDeleteMessage(chatID, msg.MessageID))
		}
	default:
		return
	}
	h.sendResponse(ctx, chatID, resp)
}

// handleCallback processes inline keyboard button taps.
// It answers the callback query immediately to remove the loading indicator,
// then delegates to the service layer and attempts an in-place edit of the
// originating message.
func (h *Handler) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	logger := ctxlog.LoggerFromCtx(ctx, h.Logger)

	chatID := callback.Message.Chat.ID
	data := callback.Data
	userID := callback.From.ID
	username := callback.From.UserName

	callbackCfg := tgbotapi.NewCallback(callback.ID, "")
	if _, err := h.Bot.Request(callbackCfg); err != nil {
		logger.Error("Failed to answer callback query", "chat_id", chatID, "error", err)
	}

	resp, err := h.Service.ProcessCallback(ctx, userID, username, data)
	if err != nil {
		logger.Error("Callback error", "error", err)
		return
	}

	if resp != nil {
		resp.MessageID = callback.Message.MessageID
		resp.Edit = true
	}

	h.sendResponse(ctx, chatID, resp)
}

// handleCallback processes inline keyboard button taps.
// It answers the callback query immediately to remove the loading indicator,
// then delegates to the service layer and attempts an in-place edit of the
// originating message.
func (h *Handler) sendResponse(ctx context.Context, chatID int64, resp *service.UserResponse) {
	logger := ctxlog.LoggerFromCtx(ctx, h.Logger)

	if resp == nil {
		return
	}

	if resp.Document != "" {
		if resp.MessageID != 0 {
			del := tgbotapi.NewDeleteMessage(chatID, resp.MessageID)
			if _, err := h.Bot.Send(del); err != nil {
				logger.Warn("Failed to delete menu message", "msg_id", resp.MessageID, "error", err)
			}
		}
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FileID(resp.Document))
		if _, err := h.Bot.Send(doc); err != nil {
			logger.Error("Failed to send document", "error", err)
		}

		resp.Edit = false
		resp.MessageID = 0
	}

	if resp.SendWelcomeImage && h.WelcomeImageID != "" {
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(h.WelcomeImageID))
		h.Bot.Send(photo)
	}

	if !h.trySendEdit(ctx, chatID, resp) {
		h.trySendMessage(ctx, chatID, resp)
	}
}

// trySendEdit attempts to edit an existing bot message in place.
// Returns false (and falls back to trySendMessage) when:
//   - resp.Edit is false or MessageID is zero;
//   - the step requires a reply keyboard (InputPhone);
//   - Telegram rejects the edit (e.g. message too old, or not a bot message).
//
// On rejection the original message is deleted so trySendMessage can send a fresh one.
func (h *Handler) trySendEdit(ctx context.Context, chatID int64, resp *service.UserResponse) bool {
	logger := ctxlog.LoggerFromCtx(ctx, h.Logger)

	if !resp.Edit || resp.MessageID == 0 || resp.InputType == service.InputPhone {
		logger.Info("Skip edit", "edit", resp.Edit, "message_id", resp.MessageID, "input_type", resp.InputType)
		return false
	}

	edit := tgbotapi.NewEditMessageText(chatID, resp.MessageID, resp.Text)
	edit.ParseMode = "HTML"
	if len(resp.Buttons) > 0 {
		markup := makeInlineKeyboard(resp.StepID, resp.Buttons, h.CommunityURL)
		edit.ReplyMarkup = &markup
	}

	result, err := h.Bot.Send(edit)
	if err != nil {
		if strings.Contains(err.Error(), "not modified") {
			return true
		}
		logger.Error("Failed to edit message", "error", err, "input_type", resp.InputType)
		if resp.MessageID != 0 {
			h.Bot.Send(tgbotapi.NewDeleteMessage(chatID, resp.MessageID))
			resp.MessageID = 0
		}
		return false
	}
	h.lastBotMsg.Store(chatID, result.MessageID)
	return true
}

// trySendMessage sends a new message to the chat.
// If resp.Edit is true but editing failed, it first deletes the stale bot message.
// Keyboard type is selected based on resp.InputType and resp.Buttons.
// The sent message ID is stored in lastBotMsg for future edit attempts.
func (h *Handler) trySendMessage(ctx context.Context, chatID int64, resp *service.UserResponse) {
	logger := ctxlog.LoggerFromCtx(ctx, h.Logger)

	if resp.Edit && resp.MessageID != 0 {
		h.Bot.Send(tgbotapi.NewDeleteMessage(chatID, resp.MessageID))
	} else if resp.Edit && resp.InputType == service.InputPhone {
		if msgID, ok := h.lastBotMsg.Load(chatID); ok {
			h.Bot.Send(tgbotapi.NewDeleteMessage(chatID, msgID.(int)))
		}
	}

	msg := tgbotapi.NewMessage(chatID, resp.Text)
	msg.ParseMode = "HTML"

	switch {
	case resp.InputType == service.InputPhone:
		msg.ReplyMarkup = makeReplyKeyboard()
	case len(resp.Buttons) > 0:
		msg.ReplyMarkup = makeInlineKeyboard(resp.StepID, resp.Buttons, h.CommunityURL)
	default:
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	}

	sent, err := h.Bot.Send(msg)
	if err != nil {
		logger.Error("Failed to send message", "error", err)
		return
	}
	h.lastBotMsg.Store(chatID, sent.MessageID)
}
