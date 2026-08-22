package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/factory"
	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
)

var botFactory *factory.Manager

// SetBotFactory wires the lifecycle manager after the database is ready.
func SetBotFactory(manager *factory.Manager) { botFactory = manager }

// HandleNewBotCommand starts a secure two-step token onboarding flow.
func HandleNewBotCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := context.Background()
	userID := message.From.ID
	allowed, _ := services.Can(ctx, userID, "manage_bots")
	if !allowed {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ إنشاء البوتات متاح للمالك أو المشرفين المخولين فقط."))
		return
	}
	if botFactory == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Bot Factory غير مفعّل. أضف FACTORY_ENCRYPTION_KEY ثم أعد التشغيل."))
		return
	}
	WithState(userID, func(state *models.UserState) {
		state.Awaiting = &models.AwaitingState{Type: "factory_bot_token"}
	})
	msg := tgbotapi.NewMessage(message.Chat.ID,
		"🤖 إنشاء بوت مُدار\n\nأرسل توكن البوت الذي أنشأته من BotFather في رسالة منفصلة.\n🔐 لن يتم تخزين التوكن بصيغته الأصلية.\n\nلإلغاء العملية أرسل /cancel.")
	bot.Send(msg)
}

// HandleMyBotsCommand renders safe metadata for the caller's managed bots.
func HandleMyBotsCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if botFactory == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Bot Factory غير مفعّل حالياً."))
		return
	}
	rows, err := botFactory.List(context.Background(), message.From.ID, false)
	if err != nil {
		log.Printf("list managed bots failed: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تحميل قائمة البوتات."))
		return
	}
	if len(rows) == 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "📭 لا توجد بوتات مُدارة. استخدم /newbot لإضافة أول بوت."))
		return
	}
	var b strings.Builder
	b.WriteString("🤖 بوتاتك المُدارة:\n\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "• @%s — %s\n  ID: %s | Updates: %d | Errors: %d\n", row.Username, row.Status, row.ID, row.TotalUpdates, row.TotalErrors)
	}
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, b.String()))
}

// HandleFactoryText consumes only the token step of the factory flow.
func HandleFactoryText(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	userID := message.From.ID
	state := GetState(userID)
	state.Mu.Lock()
	awaiting := state.Awaiting
	if awaiting != nil {
		copyAwaiting := *awaiting
		awaiting = &copyAwaiting
	}
	state.Mu.Unlock()
	if awaiting == nil || awaiting.Type != "factory_bot_token" {
		return false
	}
	if botFactory == nil {
		ClearAwaiting(userID)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Bot Factory غير مفعّل حالياً."))
		return true
	}
	row, err := botFactory.Register(context.Background(), userID, strings.TrimSpace(message.Text))
	if err != nil {
		log.Printf("managed bot registration failed for %d: %v", userID, err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تسجيل التوكن. تأكد أنه صحيح وغير مستخدم مسبقاً ثم أعد المحاولة."))
		return true
	}
	ClearAwaiting(userID)
	services.LogAdminAction(context.Background(), userID, "register_managed_bot", map[string]interface{}{"bot_id": row.ID, "username": row.Username})
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ تم تسجيل @%s وتشغيله بنجاح.\n\nالمعرّف: %s\nالحالة: %s\n\nاستخدم /mybots لعرض الحالة.", row.Username, row.ID, row.Status)))
	return true
}

// HandleCancelCommand clears any pending factory or content workflow.
func HandleCancelCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ClearAwaiting(message.From.ID)
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "✅ تم إلغاء العملية الحالية."))
}

// HandleFactoryInfoCallback shows the factory entry point without exposing secrets.
func HandleFactoryInfoCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	if query == nil || query.Message == nil {
		return
	}
	allowed, _ := services.Can(context.Background(), query.From.ID, "manage_bots")
	if !allowed {
		bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, "⛔ إدارة البوتات متاحة للمالك أو المشرفين المخولين فقط."))
		return
	}
	status := "غير مفعّل"
	if botFactory != nil {
		status = "جاهز"
	}
	text := "🤖 Bot Factory\n\nالحالة: " + status + "\n\n/newbot — إضافة بوت تم إنشاؤه من BotFather\n/mybots — عرض البوتات المُدارة\n/cancel — إلغاء العملية الحالية\n\nيتم تشفير التوكنات ولا تظهر في الردود أو السجلات."
	bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, text))
}
