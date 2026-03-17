package bot

import (
	"fmt"
	"log"

	tele "gopkg.in/telebot.v3"
)

type TelegramNotifier struct {
	bot    *tele.Bot
	chatID int64
}

func NewTelegramNotifier(bot *tele.Bot, chatID int64) *TelegramNotifier {
	return &TelegramNotifier{bot: bot, chatID: chatID}
}

func (n *TelegramNotifier) SendAlert(siteURL string, isUp bool, responseMs int) {
	var text string
	if isUp {
		text = fmt.Sprintf("🟢 %s is back UP (%d ms)", siteURL, responseMs)
	} else {
		text = fmt.Sprintf("🔴 %s is DOWN", siteURL)
	}

	recipient := &tele.Chat{ID: n.chatID}
	if _, err := n.bot.Send(recipient, text, SendSettings); err != nil {
		log.Println("Failed to send alert:", err)
	}
}
