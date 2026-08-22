package handlers

import (
	"context"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/db"
)

var storageGateway struct {
	sync.RWMutex
	bot *tgbotapi.BotAPI
}

// SetStorageBot configures the only bot allowed to use the shared Telegram
// storage channel. Managed bots may serve updates, but never need channel admin
// access and never use their own file_id to read shared storage.
func SetStorageBot(bot *tgbotapi.BotAPI) {
	storageGateway.Lock()
	storageGateway.bot = bot
	storageGateway.Unlock()
}

func archiveContext(bot *tgbotapi.BotAPI) context.Context {
	ctx := context.Background()
	if bot == nil || storageBot(bot) == bot || bot.Self.ID == 0 {
		return ctx
	}
	return db.WithBotDatabase(ctx, bot.Self.ID)
}

func storageBot(fallback *tgbotapi.BotAPI) *tgbotapi.BotAPI {
	storageGateway.RLock()
	defer storageGateway.RUnlock()
	if storageGateway.bot != nil {
		return storageGateway.bot
	}
	return fallback
}
