package handlers

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/ai"
	"telegram-archive-bot/services"
)

var assistantClient *ai.Client

var aiUsage = struct {
	sync.Mutex
	items map[int64][]time.Time
}{items: make(map[int64][]time.Time)}

func allowAIRequest(userID int64) bool {
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	aiUsage.Lock()
	defer aiUsage.Unlock()
	items := aiUsage.items[userID]
	kept := items[:0]
	for _, ts := range items {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= 10 {
		aiUsage.items[userID] = kept
		return false
	}
	aiUsage.items[userID] = append(kept, now)
	return true
}

func SetAIClient(client *ai.Client) { assistantClient = client }

func HandleAICommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, summarize bool) {
	ctx := archiveContext(bot)
	userID := message.From.ID
	if services.IsBanned(ctx, userID) || services.IsMuted(ctx, userID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ لا يمكنك استخدام مساعد الذكاء الاصطناعي حالياً."))
		return
	}
	if !allowAIRequest(userID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⏳ وصلت إلى حد 10 طلبات AI في الدقيقة. حاول لاحقاً."))
		return
	}
	if assistantClient == nil || !assistantClient.Configured() {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ خدمة الذكاء الاصطناعي غير مهيأة حالياً."))
		return
	}
	text := strings.TrimSpace(message.CommandArguments())
	if text == "" {
		usage := "/ai <سؤالك>"
		if summarize {
			usage = "/summarize <النص>"
		}
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "استخدم: "+usage))
		return
	}
	if len(text) > 20000 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ النص طويل جداً. الحد الأقصى 20,000 حرف."))
		return
	}
	prompt := text
	system := "You are a precise educational assistant. Answer in the user's language and do not invent facts."
	if summarize {
		system = "You are an accurate educational summarization assistant. Return a concise Arabic summary with the key facts."
		prompt = "لخّص النص التالي بوضوح مع الحفاظ على النقاط المهمة:\n\n" + text
	}
	result, err := assistantClient.Chat(ctx, ai.ChatRequest{Messages: []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: prompt}}})
	if err != nil {
		log.Printf("telegram AI command failed: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر الحصول على رد من خدمة الذكاء الاصطناعي."))
		return
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ وصلت استجابة فارغة من خدمة الذكاء الاصطناعي."))
		return
	}
	for _, part := range splitTelegramText(result.Choices[0].Message.Content, 3900) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, part))
	}
}

func splitTelegramText(text string, limit int) []string {
	if len([]rune(text)) <= limit {
		return []string{text}
	}
	runes := []rune(text)
	parts := make([]string, 0, (len(runes)/limit)+1)
	for len(runes) > 0 {
		n := limit
		if len(runes) < n {
			n = len(runes)
		}
		parts = append(parts, fmt.Sprint(string(runes[:n])))
		runes = runes[n:]
	}
	return parts
}
