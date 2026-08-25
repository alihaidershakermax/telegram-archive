package handlers

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/services"
)

func HandleGroupCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message == nil || message.Chat == nil {
		return
	}
	if !message.Chat.IsGroup() && !message.Chat.IsSuperGroup() {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "ℹ️ هذا الأمر يعمل داخل المجموعات فقط."))
		return
	}
	ctx := archiveContext(bot)
	group, err := services.GetOrCreateGroup(ctx, bot.Self.ID, message.Chat.ID, message.Chat.Title)
	if err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تحميل إعدادات المجموعة."))
		return
	}
	state := "مفعّلة"
	if !group.Enabled {
		state = "متوقفة"
	}
	_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("⚙️ إعدادات المجموعة\\n\\nالاسم: %s\\nالمعرّف: %d\\nالحالة: %s\\nصلاحيات الأدمن: %t\\n\\nكل إعدادات هذه المجموعة محفوظة داخل namespace البوت الحالي فقط.", group.Title, group.ChatID, state, group.AdminsOnly)))
}
