package handlers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/mongo"

	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
)

var botRequestLimiter = struct {
	sync.Mutex
	windows map[int64]requestWindow
}{windows: make(map[int64]requestWindow)}

type requestWindow struct {
	started time.Time
	count   int
}

func managedLimits(bot *tgbotapi.BotAPI) (*models.ManagedBot, bool, error) {
	if bot == nil || storageBot(bot) == bot || botFactory == nil || bot.Self.ID <= 0 {
		return nil, false, nil
	}
	row, err := botFactory.GetByTelegramBotID(context.Background(), bot.Self.ID)
	if err != nil {
		return nil, true, err
	}
	return row, true, nil
}

func checkNewUserCapacity(ctx context.Context, bot *tgbotapi.BotAPI, userID int64) error {
	if _, err := services.GetUser(ctx, userID); err == nil {
		return nil
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	limits, managed, err := managedLimits(bot)
	if err != nil || !managed || limits.MaxUsers <= 0 {
		return err
	}
	return services.CheckUserCapacity(ctx, limits.MaxUsers)
}

func checkFileCapacity(ctx context.Context, bot *tgbotapi.BotAPI, fileSize int64) error {
	limits, managed, err := managedLimits(bot)
	if err != nil || !managed {
		return err
	}
	if fileSize < 0 {
		fileSize = 0
	}
	return services.CheckFileCapacity(ctx, limits.MaxFiles, limits.MaxStorageBytes, fileSize)
}

// AllowBotUpdate applies a per-managed-bot minute window. The primary bot
// keeps its existing behavior and is not throttled by factory quotas.
func AllowBotUpdate(bot *tgbotapi.BotAPI) bool {
	limits, managed, err := managedLimits(bot)
	if err != nil || !managed || limits.MaxRequestsPerMinute <= 0 {
		return true
	}
	now := time.Now()
	botRequestLimiter.Lock()
	defer botRequestLimiter.Unlock()
	window := botRequestLimiter.windows[bot.Self.ID]
	if window.started.IsZero() || now.Sub(window.started) >= time.Minute {
		botRequestLimiter.windows[bot.Self.ID] = requestWindow{started: now, count: 1}
		return true
	}
	if window.count >= limits.MaxRequestsPerMinute {
		return false
	}
	window.count++
	botRequestLimiter.windows[bot.Self.ID] = window
	return true
}

func quotaMessage(err error) string {
	if errors.Is(err, services.ErrQuotaExceeded) {
		return "⛔ تم بلوغ حد الاستخدام المسموح لهذا البوت. تواصل مع مالك البوت لزيادة الحد."
	}
	return fmt.Sprintf("❌ تعذر التحقق من حد الاستخدام: %v", err)
}
