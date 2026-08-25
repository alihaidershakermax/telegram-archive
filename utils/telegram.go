package utils

import (
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramHTTPTimeout is longer than the 30-second long-poll interval while
// still bounding startup, health, and regular Bot API requests.
const TelegramHTTPTimeout = 45 * time.Second

// NewTelegramBot creates a BotAPI client with a bounded HTTP timeout. The
// upstream library's default http.Client has no timeout, so using it directly
// can hang startup or health checks indefinitely when Telegram is unreachable.
func newTelegramHTTPClient() *http.Client {
	return &http.Client{Timeout: TelegramHTTPTimeout}
}

func NewTelegramBot(token string) (*tgbotapi.BotAPI, error) {
	return tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, newTelegramHTTPClient())
}
