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

func isParentBotOwner(bot *tgbotapi.BotAPI, userID int64) bool {
	return bot != nil && storageBot(bot) == bot && services.IsOwner(userID)
}

// SetBotFactory wires the lifecycle manager after the database is ready.
func SetBotFactory(manager *factory.Manager) { botFactory = manager }

// HandleNewBotCommand starts a secure two-step token onboarding flow.
func HandleNewBotCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	allowed := isParentBotOwner(bot, userID)
	if !allowed {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ إنشاء البوتات متاح للمالك أو المشرفين المخولين فقط."))
		return
	}
	if botFactory == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Bot Factory غير مفعّل. أضف FACTORY_ENCRYPTION_KEY ثم أعد التشغيل."))
		return
	}
	WithStateForBot(bot, userID, func(state *models.UserState) {
		state.Awaiting = &models.AwaitingState{Type: "factory_bot_token"}
	})
	msg := tgbotapi.NewMessage(message.Chat.ID,
		"🤖 إنشاء بوت مُدار\n\nأرسل توكن البوت الذي أنشأته من BotFather في رسالة منفصلة.\n🔐 لن يتم تخزين التوكن بصيغته الأصلية.\n\nلإلغاء العملية أرسل /cancel.")
	bot.Send(msg)
}

// HandleMyBotsCommand renders safe metadata for the caller's managed bots.
func HandleMyBotsCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isParentBotOwner(bot, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ إدارة البوتات متاحة لمالك البوت الأب فقط."))
		return
	}
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

// HandleDatabasePanelCommand shows cluster management controls in the parent bot.
func HandleDatabasePanelCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	allowed := isParentBotOwner(bot, message.From.ID)
	if !allowed {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ لوحة قواعد البيانات متاحة للمالك أو المشرفين المخولين فقط."))
		return
	}
	if botFactory == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Bot Factory غير مفعّل حالياً."))
		return
	}
	rows, err := botFactory.ListStorageClusters(context.Background())
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تحميل قواعد البيانات."))
		return
	}
	var b strings.Builder
	b.WriteString("🗄 لوحة قواعد Bot Factory\\n\\n")
	if len(rows) == 0 {
		b.WriteString("لا توجد Clusters مسجلة حالياً.\\n")
	} else {
		for _, row := range rows {
			fmt.Fprintf(&b, "• %s — %s\\n  الحالة: %s\\n", row.Name, row.ID, row.Status)
		}
	}
	b.WriteString("\n/adddb — إضافة MongoDB Cluster\n/dbs — عرض القواعد\n/dbdisable <id> — تعطيل Cluster\n/dbenable <id> — تفعيل Cluster\n/dbremove <id> — إزالة Cluster غير المرتبطة\n/migratebot <bot_id> <cluster_id> — نقل قاعدة بوت\n/migrationstatus [bot_id] — حالة النقل\n/cancel — إلغاء العملية")
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, b.String()))
}

// HandleAddDatabaseCommand starts the private, two-step cluster onboarding flow.
func HandleAddDatabaseCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	allowed := isParentBotOwner(bot, message.From.ID)
	if !allowed {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية إضافة قاعدة."))
		return
	}
	if !message.Chat.IsPrivate() {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "🔐 أرسل أمر /adddb في محادثة خاصة مع البوت الأب لحماية بيانات الاتصال."))
		return
	}
	WithStateForBot(bot, message.From.ID, func(state *models.UserState) {
		state.Awaiting = &models.AwaitingState{Type: "factory_cluster_name"}
	})
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "🗄 إضافة MongoDB Cluster\\n\\nأرسل اسماً مختصراً للقاعدة، مثل: europe-1\\n\\nلن يتم قبول Mongo URI إلا في الخطوة التالية داخل محادثة خاصة."))
}

// HandleDatabaseListCommand lists safe cluster metadata without credentials.
func HandleDatabaseListCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	HandleDatabasePanelCommand(bot, message)
}

func HandleMigrationCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	allowed := isParentBotOwner(bot, message.From.ID)
	if !allowed || botFactory == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية نقل قواعد البوتات."))
		return
	}
	parts := strings.Fields(message.CommandArguments())
	if len(parts) != 2 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ الاستخدام: /migratebot <bot_id> <target_cluster_id>"))
		return
	}
	var botID int64
	if _, err := fmt.Sscan(parts[0], &botID); err != nil || botID <= 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ bot_id غير صالح."))
		return
	}
	target := parts[1]
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⏳ بدأت عملية النقل على دفعات. استخدم /migrationstatus لمتابعة الحالة."))
	go func() {
		err := botFactory.MigrateBotNamespace(context.Background(), botID, target)
		if err != nil {
			log.Printf("namespace migration failed for bot %d: %v", botID, err)
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ فشل النقل؛ بقيت قاعدة البوت على المسار القديم."))
			return
		}
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "✅ اكتمل نقل قاعدة البوت والتحويل إلى Cluster الجديدة."))
	}()
}

func HandleMigrationStatusCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	allowed := isParentBotOwner(bot, message.From.ID)
	if !allowed || botFactory == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية عرض حالة النقل."))
		return
	}
	var botID int64
	if arg := strings.TrimSpace(message.CommandArguments()); arg != "" {
		if _, err := fmt.Sscan(arg, &botID); err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ bot_id غير صالح."))
			return
		}
	}
	rows, err := botFactory.ListNamespaceMigrations(context.Background(), botID)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تحميل حالة النقل."))
		return
	}
	if len(rows) == 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "📭 لا توجد عمليات نقل مسجلة."))
		return
	}
	var b strings.Builder
	b.WriteString("🔄 عمليات نقل قواعد البوتات\\n\\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "• Bot %d: %s\\n  %s → %s | documents: %d\\n", row.BotID, row.Status, row.Source, row.Target, row.Documents)
	}
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, b.String()))
}

func HandleDatabaseClusterAction(bot *tgbotapi.BotAPI, message *tgbotapi.Message, action string) {
	allowed := isParentBotOwner(bot, message.From.ID)
	if !allowed || botFactory == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية إدارة قواعد البيانات."))
		return
	}
	id := strings.TrimSpace(message.CommandArguments())
	if id == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ أرسل مع الأمر معرّف Cluster."))
		return
	}
	var err error
	switch action {
	case "enable":
		err = botFactory.SetStorageClusterStatus(context.Background(), id, models.StorageClusterActive)
	case "disable":
		err = botFactory.SetStorageClusterStatus(context.Background(), id, models.StorageClusterDraining)
	case "remove":
		err = botFactory.DeleteStorageCluster(context.Background(), id)
	}
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تنفيذ العملية: "+err.Error()))
		return
	}
	services.LogAdminAction(context.Background(), message.From.ID, "storage_cluster_"+action, map[string]interface{}{"cluster_id": id})
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "✅ تم تنفيذ العملية على Cluster: "+id))
}

// HandleFactoryText consumes factory token and cluster onboarding steps.
func HandleFactoryText(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	if message == nil || message.From == nil || !isParentBotOwner(bot, message.From.ID) {
		return false
	}
	userID := message.From.ID
	state := GetStateForBot(bot, userID)
	state.Mu.Lock()
	awaiting := state.Awaiting
	if awaiting != nil {
		copyAwaiting := *awaiting
		awaiting = &copyAwaiting
	}
	state.Mu.Unlock()
	if awaiting == nil || (awaiting.Type != "factory_bot_token" && awaiting.Type != "factory_cluster_name" && awaiting.Type != "factory_cluster_uri") {
		return false
	}
	if awaiting.Type == "factory_cluster_name" {
		name := strings.TrimSpace(message.Text)
		if name == "" || len(name) > 64 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ الاسم مطلوب وبحد أقصى 64 حرفاً."))
			return true
		}
		WithStateForBot(bot, userID, func(state *models.UserState) {
			state.Awaiting = &models.AwaitingState{Type: "factory_cluster_uri", Value: name}
		})
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "الآن أرسل Mongo URI في رسالة منفصلة. سيتم حذف الرسالة فوراً وعدم تسجيلها.\\n\\nمثال: mongodb+srv://..."))
		return true
	}
	if awaiting.Type == "factory_cluster_uri" {
		_, _ = bot.Request(tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID))
		if !message.Chat.IsPrivate() {
			ClearAwaitingForBot(bot, userID)
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ يجب إدخال Mongo URI في محادثة خاصة فقط."))
			return true
		}
		if botFactory == nil {
			ClearAwaitingForBot(bot, userID)
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Bot Factory غير مفعّل حالياً."))
			return true
		}
		row, err := botFactory.AddStorageCluster(context.Background(), awaiting.Value, message.Text)
		ClearAwaitingForBot(bot, userID)
		if err != nil {
			log.Printf("storage cluster registration failed for owner %d: %v", userID, err)
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ فشل فحص أو تسجيل Cluster. تأكد من URI والصلاحيات ثم أعد المحاولة."))
			return true
		}
		services.LogAdminAction(context.Background(), userID, "register_storage_cluster", map[string]interface{}{"cluster_id": row.ID, "name": row.Name})
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ تم تسجيل Cluster `%s` وتفعيلها بعد نجاح الفحص.\\nالمعرّف: `%s`", row.Name, row.ID)))
		return true
	}
	if botFactory == nil {
		ClearAwaitingForBot(bot, userID)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Bot Factory غير مفعّل حالياً."))
		return true
	}
	row, err := botFactory.Register(context.Background(), userID, strings.TrimSpace(message.Text))
	// Best-effort deletion keeps the token out of the visible chat history.
	_, _ = bot.Request(tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID))
	if err != nil {
		log.Printf("managed bot registration failed for %d: %v", userID, err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تسجيل التوكن. تأكد أنه صحيح وغير مستخدم مسبقاً ثم أعد المحاولة."))
		return true
	}
	ClearAwaitingForBot(bot, userID)
	services.LogAdminAction(context.Background(), userID, "register_managed_bot", map[string]interface{}{"bot_id": row.ID, "username": row.Username})
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ تم تسجيل @%s وتشغيله بنجاح.\n\nالمعرّف: %s\nالحالة: %s\n\nاستخدم /mybots لعرض الحالة.", row.Username, row.ID, row.Status)))
	return true
}

// HandleCancelCommand clears any pending factory or content workflow.
func HandleCancelCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ClearAwaitingForBot(bot, message.From.ID)
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "✅ تم إلغاء العملية الحالية."))
}

// HandleFactoryInfoCallback shows the factory entry point without exposing secrets.
func HandleFactoryInfoCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	if query == nil || query.Message == nil {
		return
	}
	allowed := isParentBotOwner(bot, query.From.ID)
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
