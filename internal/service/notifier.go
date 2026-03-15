package service

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type TelegramNotifier struct {
	token  string
	chatID string
	client *http.Client
}

func NewTelegramNotifier(token, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		token:  token,
		chatID: chatID,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (n *TelegramNotifier) SendAlert(siteURL string, isUp bool, responseMs int) {
	log.Println("SendAlert called for", siteURL)

	var text string
	if isUp {
		text = fmt.Sprintf("🟢 *%s* is back UP (%d ms)", siteURL, responseMs)
	} else {
		text = fmt.Sprintf("🔴 *%s* is DOWN", siteURL)
	}

	n.send(text)
}

func (n *TelegramNotifier) send(text string) {
	tokenPreview := n.token
	if len(tokenPreview) > 10 {
		tokenPreview = tokenPreview[:10]
	}

	log.Printf("Sending to chatID: %s, token starts with: %s\n", n.chatID, tokenPreview)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.token)

	resp, err := n.client.PostForm(apiURL, url.Values{
		"chat_id":    {n.chatID},
		"text":       {text},
		"parse_mode": {"Markdown"},
	})

	if err != nil {
		log.Println("Failed to send Telegram notification:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read Telegram response:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Telegram API error: %d %s\n", resp.StatusCode, string(body))
		return
	}

	log.Printf("Telegram response: %d %s\n", resp.StatusCode, string(body))
}
