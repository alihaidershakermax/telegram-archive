package main

import (
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/ai"
	"telegram-archive-bot/api"
	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/handlers"
)

func main() {
	log.Println("Starting Telegram Archive Bot (Go)...")

	// Load configuration
	config.Load()

	// Initialize MongoDB
	db.Init()
	defer db.Close()
	db.InitCounters()
	db.EnsureIndexes()

	// Create AI client and bot
	handlers.SetAIClient(ai.NewClient(config.Cfg.AIBaseURL, config.Cfg.AIAPIKey, config.Cfg.AIModel, time.Duration(config.Cfg.AIRequestTimeoutSeconds)*time.Second))
	bot, err := tgbotapi.NewBotAPI(config.Cfg.BotToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Set bot commands menu in Telegram
	botCmds := []tgbotapi.BotCommand{
		{Command: "start", Description: "🚀 القائمة الرئيسية / بدء البوت"},
		{Command: "panel", Description: "⚙️ لوحة التحكم (للمشرفين)"},
		{Command: "broadcast", Description: "📢 إرسال إذاعة (للمشرفين)"},
		{Command: "ban", Description: "⛔ حظر مستخدم (للمشرفين)"},
		{Command: "unban", Description: "✅ إلغاء حظر (للمشرفين)"},
		{Command: "ai", Description: "🤖 اسأل المساعد الذكي"},
		{Command: "summarize", Description: "📝 تلخيص نص"},
	}
	_, errCmd := bot.Request(tgbotapi.NewSetMyCommands(botCmds...))
	if errCmd != nil {
		log.Printf("Warning: failed to set bot commands: %v", errCmd)
	}

	// Start polling
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := bot.GetUpdatesChan(u)

	// Start HTTP health check server for cloud platforms (Render, Koyeb, Railway, etc.)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	go func() {
		apiServer := api.NewServer(config.Cfg, bot)
		log.Printf("API and health server listening on port %s", port)
		if err := http.ListenAndServe(":"+port, apiServer.Handler()); err != nil {
			log.Printf("API server stopped: %v", err)
		}
	}()

	log.Println("Bot is running...")

	for update := range updates {
		go handleUpdate(bot, update)
	}
}

func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic recovered in handleUpdate: %v", r)
		}
	}()

	// Handle callback queries
	if update.CallbackQuery != nil {
		handlers.HandleCallback(bot, update.CallbackQuery)
		return
	}

	// Handle messages
	if update.Message == nil {
		return
	}

	msg := update.Message

	// Handle commands
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			handlers.HandleStart(bot, msg)
		case "panel":
			handlers.HandlePanel(bot, msg)
		case "broadcast":
			handlers.HandleBroadcastCommand(bot, msg)
		case "ban":
			handlers.HandleBanCommand(bot, msg)
		case "unban":
			handlers.HandleUnbanCommand(bot, msg)
		case "ai":
			handlers.HandleAICommand(bot, msg, false)
		case "summarize":
			handlers.HandleAICommand(bot, msg, true)

		}
		return
	}

	// Handle text messages (for awaiting state)
	if msg.Text != "" {
		handlers.HandleTextMessage(bot, msg)
		return
	}

	// Handle file uploads
	if hasFile(msg) {
		handlers.HandleFileUpload(bot, msg)
		return
	}
}

func hasFile(msg *tgbotapi.Message) bool {
	return msg.Document != nil || msg.Video != nil || msg.Audio != nil ||
		(msg.Photo != nil && len(msg.Photo) > 0) || msg.Voice != nil ||
		msg.Animation != nil
}
