package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mipecx/survey-bot-go/internal/service"
)

func makeInlineKeyboard(stepID string, buttons []string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, btnText := range buttons {
		var button tgbotapi.InlineKeyboardButton

		if btnText == service.BtnCommunity {
			// Создаем кнопку-ссылку, она не шлет callback, а просто открывает браузер/TG
			button = tgbotapi.NewInlineKeyboardButtonURL(btnText, service.GetCommunityURL())
		} else {
			// Обычная логика с привязкой ID вопроса для защиты от спама
			callbackData := btnText
			if stepID != "" {
				callbackData = fmt.Sprintf("%s:%s", stepID, btnText)
			}
			button = tgbotapi.NewInlineKeyboardButtonData(btnText, callbackData)
		}

		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

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
