package handlers

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/services"
)

func HandleSubscribeCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	handleSubjectSubscription(bot, message, true)
}

func HandleUnsubscribeCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	handleSubjectSubscription(bot, message, false)
}

func handleSubjectSubscription(bot *tgbotapi.BotAPI, message *tgbotapi.Message, subscribe bool) {
	if message == nil || message.From == nil || message.Chat == nil {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(message.CommandArguments()))
	if err != nil || id <= 0 {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "الاستخدام: /subscribe <subject_id>"))
		return
	}
	ctx := archiveContext(bot)
	if subscribe {
		err = services.SubscribeToSubject(ctx, bot.Self.ID, message.From.ID, id)
	} else {
		err = services.UnsubscribeFromSubject(ctx, bot.Self.ID, message.From.ID, id)
	}
	if err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تحديث الاشتراك."))
		return
	}
	text := "✅ تم الاشتراك بالمادة. سأرسل لك تنبيهاً عند إضافة ملف جديد."
	if !subscribe {
		text = "✅ تم إلغاء الاشتراك بالمادة."
	}
	_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, text))
}

func HandleSubscriptionsCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message == nil || message.From == nil || message.Chat == nil {
		return
	}
	rows, err := services.ListSubjectSubscriptions(archiveContext(bot), bot.Self.ID, message.From.ID)
	if err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تحميل الاشتراكات."))
		return
	}
	if len(rows) == 0 {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "لا توجد اشتراكات حالياً. استخدم /subscribe <subject_id>"))
		return
	}
	var b strings.Builder
	b.WriteString("📚 اشتراكاتك:\n\n")
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("• المادة رقم %d\n", row.SubjectID))
	}
	_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, b.String()))
}

func HandleVaultAddCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message == nil || message.From == nil || message.Chat == nil {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(message.CommandArguments()))
	if err != nil || id <= 0 {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "الاستخدام: /vaultadd <file_id>"))
		return
	}
	if err := services.AddToVault(archiveContext(bot), bot.Self.ID, message.From.ID, id); err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر حفظ الملف في خزنتك."))
		return
	}
	_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "✅ تمت إضافة الملف إلى Personal Vault."))
}

func HandleVaultCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message == nil || message.From == nil || message.Chat == nil {
		return
	}
	rows, err := services.ListVault(archiveContext(bot), bot.Self.ID, message.From.ID)
	if err != nil {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تحميل خزنتك."))
		return
	}
	if len(rows) == 0 {
		_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, "خزنتك فارغة حالياً."))
		return
	}
	var b strings.Builder
	b.WriteString("🔐 Personal Vault\n\n")
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("• الملف رقم %d\n", row.FileID))
	}
	_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, b.String()))
}
