package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mipecx/survey-bot-go/internal/service"
)

// makeInlineKeyboard builds an inline keyboard markup from a slice of button labels.
//
// Button behaviour depends on the label:
//   - BtnCommunity and BtnGift render as URL buttons pointing to communityURL.
//   - All other buttons render as callback buttons.
//
// Callback data format:
//   - If stepID is empty (main menu): callback_data = button label text.
//   - If stepID is set (survey step): callback_data = "stepID:index", where index
//     is the zero-based position of the button in the slice. This keeps callback_data
//     within Telegram's 64-byte limit regardless of option text length.
//     The index is resolved back to option text via resolveOption in ProcessCallback.
//
// Each button occupies its own row.
func makeInlineKeyboard(stepID string, buttons []string, communityURL string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for i, btnText := range buttons {
		var button tgbotapi.InlineKeyboardButton
		if btnText == service.BtnCommunity || btnText == service.BtnGift {
			button = tgbotapi.NewInlineKeyboardButtonURL(btnText, communityURL)
		} else {
			callbackData := btnText
			if stepID != "" {
				callbackData = fmt.Sprintf("%s:%d", stepID, i)
			}
			button = tgbotapi.NewInlineKeyboardButtonData(btnText, callbackData)
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// makeReplyKeyboard builds a one-time reply keyboard with a single
// "Share phone number" button, used during the phone collection step.
// OneTimeKeyboard=true hides the keyboard after the user taps it.
func makeReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonContact("Поделиться номером"),
		),
	)
	keyboard.OneTimeKeyboard = true
	keyboard.ResizeKeyboard = true
	return keyboard
}
