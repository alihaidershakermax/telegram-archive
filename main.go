package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/ai"
	"telegram-archive-bot/api"
	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/factory"
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
	handlers.SetStorageBot(bot)
	queueCtx, queueCancel := context.WithCancel(context.Background())
	handlers.StartStorageQueue(queueCtx, config.Cfg)
	defer queueCancel()

	// Start the optional Bot Factory.
	// It stays disabled unless a strong encryption key is configured, so
	// plaintext child-bot tokens are never persisted.
	var factoryManager *factory.Manager
	var expansionController *factory.AutoExpansionController
	if config.Cfg.FactoryEncryptionKey != "" {
		factoryManager, err = factory.NewManager(config.Cfg, handleUpdate)
		if err != nil {
			log.Printf("Bot Factory disabled: %v", err)
		} else {
			handlers.SetBotFactory(factoryManager)
			if err := factoryManager.LoadAndStart(context.Background()); err != nil {
				log.Printf("Bot Factory restore warning: %v", err)
			}
			defer factoryManager.Close()
			expansionController = factory.StartAutoExpansion(context.Background(), config.Cfg)
			defer expansionController.Stop()

		}
	}

	// Set bot commands menu in Telegram
	botCmds := []tgbotapi.BotCommand{
		{Command: "start", Description: "🚀 القائمة الرئيسية / بدء البوت"},
		{Command: "panel", Description: "⚙️ لوحة التحكم (للمشرفين)"},
		{Command: "broadcast", Description: "📢 إرسال إذاعة (للمشرفين)"},
		{Command: "ban", Description: "⛔ حظر مستخدم (للمشرفين)"},
		{Command: "unban", Description: "✅ إلغاء حظر (للمشرفين)"},
		{Command: "ai", Description: "🤖 اسأل المساعد الذكي"},
		{Command: "summarize", Description: "📝 تلخيص نص"},
		{Command: "newbot", Description: "🤖 إضافة بوت مُدار"},
		{Command: "mybots", Description: "📊 بوتاتي المُدارة"},
		{Command: "handoff", Description: "🤝 تسليم إدارة بوت لشخص"},

		{Command: "dbpanel", Description: "🗄 لوحة قواعد البيانات"},
		{Command: "adddb", Description: "➕ إضافة MongoDB Cluster"},
		{Command: "dbs", Description: "📋 عرض MongoDB Clusters"},
		{Command: "dbdisable", Description: "⏸ تعطيل Cluster"},
		{Command: "dbenable", Description: "▶️ تفعيل Cluster"},
		{Command: "dbremove", Description: "🗑 إزالة Cluster غير مرتبطة"},
		{Command: "migratebot", Description: "🔄 نقل قاعدة بوت"},
		{Command: "migrationstatus", Description: "📈 حالة نقل القواعد"},
		{Command: "group", Description: "⚙️ إعدادات المجموعة"},
		{Command: "subscribe", Description: "🔔 الاشتراك في مادة"},
		{Command: "unsubscribe", Description: "🔕 إلغاء اشتراك مادة"},
		{Command: "subscriptions", Description: "📚 اشتراكاتي"},
		{Command: "vault", Description: "🔐 Personal Vault"},
		{Command: "vaultadd", Description: "➕ حفظ ملف في Vault"},

		{Command: "cancel", Description: "❌ إلغاء العملية"},
	}
	if _, errCmd := bot.Request(tgbotapi.NewSetMyCommands(botCmds...)); errCmd != nil {
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
	apiServer := api.NewServer(config.Cfg, bot)
	apiServer.SetFactory(factoryManager)
	apiServer.SetExpansion(expansionController)
	httpServer := &http.Server{Addr: ":" + port, Handler: apiServer.Handler()}
	go func() {
		log.Printf("API and health server listening on port %s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server stopped: %v", err)
		}
	}()

	log.Println("Bot is running...")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		log.Println("Shutdown signal received; stopping Telegram polling and managed workers...")
		bot.StopReceivingUpdates()
		queueCancel()
		if expansionController != nil {
			expansionController.Stop()
		}
		if factoryManager != nil {
			factoryManager.Close()
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown warning: %v", err)
		}
	}()

	for update := range updates {
		go handleUpdate(bot, update)
	}
}

func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if !handlers.AllowBotUpdate(bot) {
		if update.Message != nil {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⏳ تم الوصول إلى حد الطلبات لهذا البوت، يرجى المحاولة بعد قليل."))
		}
		return
	}
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
		case "newbot":
			handlers.HandleNewBotCommand(bot, msg)
		case "mybots":
			handlers.HandleMyBotsCommand(bot, msg)
		case "handoff":
			handlers.HandleHandoffCommand(bot, msg)

		case "dbpanel":
			handlers.HandleDatabasePanelCommand(bot, msg)
		case "adddb":
			handlers.HandleAddDatabaseCommand(bot, msg)
		case "dbs":
			handlers.HandleDatabaseListCommand(bot, msg)
		case "dbdisable":
			handlers.HandleDatabaseClusterAction(bot, msg, "disable")
		case "dbenable":
			handlers.HandleDatabaseClusterAction(bot, msg, "enable")
		case "dbremove":
			handlers.HandleDatabaseClusterAction(bot, msg, "remove")
		case "migratebot":
			handlers.HandleMigrationCommand(bot, msg)
		case "migrationstatus":
			handlers.HandleMigrationStatusCommand(bot, msg)
		case "group":
			handlers.HandleGroupCommand(bot, msg)
		case "subscribe":
			handlers.HandleSubscribeCommand(bot, msg)
		case "unsubscribe":
			handlers.HandleUnsubscribeCommand(bot, msg)
		case "subscriptions":
			handlers.HandleSubscriptionsCommand(bot, msg)
		case "vault":
			handlers.HandleVaultCommand(bot, msg)
		case "vaultadd":
			handlers.HandleVaultAddCommand(bot, msg)

		case "cancel":
			handlers.HandleCancelCommand(bot, msg)
		}
		return
	}

	// Handle text messages (for awaiting state)
	if msg.Text != "" {
		if handlers.HandleFactoryText(bot, msg) {
			return
		}
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
