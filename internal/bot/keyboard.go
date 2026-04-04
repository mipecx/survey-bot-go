package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func makeInlineKeyboard(buttons []string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, btnText := range buttons {
		button := tgbotapi.NewInlineKeyboardButtonData(btnText, btnText)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}
