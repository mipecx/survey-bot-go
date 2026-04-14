package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	admins map[int64]bool
}

func NewTelegramNotifier(bot *tgbotapi.BotAPI, admins map[int64]bool) *TelegramNotifier {
	return &TelegramNotifier{bot: bot, admins: admins}
}

func (n *TelegramNotifier) Notify(text string) error {
	if len(n.admins) == 0 {
		fmt.Println("Нотификатор: список админов пуст!")
		return nil
	}

	for adminID := range n.admins {
		msg := tgbotapi.NewMessage(adminID, text)
		msg.ParseMode = "HTML"

		_, err := n.bot.Send(msg)
		if err != nil {
			fmt.Printf("Ошибка уведомления админа %d: %v\n", adminID, err)
		} else {
			fmt.Printf("Уведомление отправлено админу %d\n", adminID)
		}
	}
	return nil
}
