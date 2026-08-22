package handlers

import (
	"fmt"
	"log"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/keyboards"
	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
)

// userStates holds per-user conversation state (replaces Python's context.user_data).
var (
	userStates   = map[string]*models.UserState{}
	userStatesMu sync.RWMutex
)

func stateKey(bot *tgbotapi.BotAPI, userID int64) string {
	botID := int64(0)
	if bot != nil {
		botID = bot.Self.ID
	}
	return fmt.Sprintf("%d:%d", botID, userID)
}

// GetState returns the legacy primary-bot state for compatibility.
func GetState(userID int64) *models.UserState { return GetStateForBot(nil, userID) }

// GetStateForBot keeps conversation state isolated by Telegram bot and user.
func GetStateForBot(bot *tgbotapi.BotAPI, userID int64) *models.UserState {
	key := stateKey(bot, userID)
	userStatesMu.Lock()
	defer userStatesMu.Unlock()
	s, ok := userStates[key]
	if !ok {
		s = &models.UserState{}
		userStates[key] = s
	}
	return s
}

// WithState executes fn while holding the legacy primary-bot state lock.
func WithState(userID int64, fn func(*models.UserState)) { WithStateForBot(nil, userID, fn) }

func WithStateForBot(bot *tgbotapi.BotAPI, userID int64, fn func(*models.UserState)) {
	s := GetStateForBot(bot, userID)
	s.Mu.Lock()
	defer s.Mu.Unlock()
	fn(s)
}

// ClearAwaiting clears the legacy primary-bot awaiting state.
func ClearAwaiting(userID int64) { ClearAwaitingForBot(nil, userID) }

func ClearAwaitingForBot(bot *tgbotapi.BotAPI, userID int64) {
	s := GetStateForBot(bot, userID)
	s.Mu.Lock()
	s.Awaiting = nil
	s.Mu.Unlock()
}

// HandleStart handles the /start command.
func HandleStart(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := archiveContext(bot)
	user := message.From
	if user == nil {
		return
	}

	if err := checkNewUserCapacity(ctx, bot, user.ID); err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, quotaMessage(err)))
		return
	}
	_ = services.SaveUser(ctx, user.ID, user.UserName, user.FirstName)

	if services.IsBanned(ctx, user.ID) {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ أنت محظور من استخدام البوت.")
		bot.Send(msg)
		return
	}

	if services.IsMuted(ctx, user.ID) {
		msg := tgbotapi.NewMessage(message.Chat.ID, "🔇 أنت مكتوم من استخدام البوت، يرجى المحاولة لاحقاً.")
		bot.Send(msg)
		return
	}

	// Handle deep links: /start share_XXXX
	args := message.CommandArguments()
	if strings.HasPrefix(args, "share_") {
		shareHash := strings.TrimPrefix(args, "share_")
		telegramFileID, fileType, err := services.GetShareLink(ctx, shareHash)
		if err != nil || telegramFileID == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ رابط المشاركة غير صالح أو انتهت صلاحيته.")
			bot.Send(msg)
			return
		}
		sender := storageBot(bot)
		action := tgbotapi.NewChatAction(message.Chat.ID, "upload_document")
		if fileType == "photo" {
			action = tgbotapi.NewChatAction(message.Chat.ID, "upload_photo")
		}
		sender.Send(action)
		queued, err := SendStorageFile(ctx, bot, message.Chat.ID, telegramFileID, fileType, "📤 الملف المشارك معك")
		if queued {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⏳ تمت جدولة الملف وسيصل تلقائياً عند عودة خدمة التخزين."))
		} else if err != nil {
			log.Printf("shared file send failed: %v", err)
		}

		return
	}

	// Check maintenance mode
	isAdmin, _ := services.IsAdmin(ctx, user.ID)
	if services.IsMaintenanceEnabled(ctx) && !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "🔧 البوت تحت الصيانة حالياً، يرجى المحاولة لاحقاً.")
		bot.Send(msg)
		return
	}

	// Remove any persistent reply keyboard from legacy bot versions
	rmMsg := tgbotapi.NewMessage(message.Chat.ID, "🔄")
	rmMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	sentRm, errRm := bot.Send(rmMsg)
	if errRm == nil {
		del := tgbotapi.NewDeleteMessage(message.Chat.ID, sentRm.MessageID)
		_, _ = bot.Send(del)
	}

	// Send welcome message
	welcome := services.GetWelcomeSettings(ctx)
	kb := MainMenuKeyboardForUser(bot, user.ID, isAdmin)

	if welcome.Photo != "" {
		photo := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FileID(welcome.Photo))
		photo.Caption = welcome.Message
		_, err := bot.Send(photo)
		if err != nil {
			// Fallback to text
			msg := tgbotapi.NewMessage(message.Chat.ID, welcome.Message)
			msg.ReplyMarkup = kb
			bot.Send(msg)
			return
		}
		msg := tgbotapi.NewMessage(message.Chat.ID, "👇 اختر من القائمة:")
		msg.ReplyMarkup = kb
		bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(message.Chat.ID, welcome.Message)
		msg.ReplyMarkup = kb
		bot.Send(msg)
	}
}

func MainMenuKeyboardForUser(bot *tgbotapi.BotAPI, userID int64, isAdmin bool) tgbotapi.InlineKeyboardMarkup {
	showFactory := bot != nil && storageBot(bot) == bot && services.IsOwner(userID)
	return keyboards.MainMenuKeyboardForRole(isAdmin, showFactory)
}

// HandleIDCommand returns the caller's Telegram ID for secure handoff setup.
func HandleIDCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message == nil || message.From == nil || message.Chat == nil {
		return
	}
	_, _ = bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Telegram user ID: %d", message.From.ID)))
}

// HandlePanel handles the /panel command.
func HandlePanel(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := archiveContext(bot)
	userID := message.From.ID

	isAdmin, _ := services.IsAdmin(ctx, userID)
	if !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ هذا الأمر للمشرفين فقط.")
		bot.Send(msg)
		return
	}

	perms, _ := services.GetUserPermissions(ctx, userID)
	kb := keyboards.AdminPanelKeyboard(perms)
	msg := tgbotapi.NewMessage(message.Chat.ID, "⚙️ لوحة التحكم")
	msg.ReplyMarkup = kb
	bot.Send(msg)
}

// HandleBroadcastCommand handles the /broadcast command.
func HandleBroadcastCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := archiveContext(bot)
	userID := message.From.ID

	allowed, _ := services.Can(ctx, userID, "broadcast")
	if !allowed {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية الإذاعة.")
		bot.Send(msg)
		return
	}

	text := message.CommandArguments()
	if text == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "📢 استخدم الأمر: /broadcast <الرسالة>")
		bot.Send(msg)
		return
	}

	statusMsg := tgbotapi.NewMessage(message.Chat.ID, "⏳ جاري إرسال الإذاعة للمستخدمين...")
	sent, _ := bot.Send(statusMsg)

	delay := services.GetBroadcastDelay()
	result := services.SendBroadcast(ctx, bot, text, delay)

	services.LogAdminAction(ctx, userID, "broadcast", map[string]interface{}{
		"text":    truncate(text, 100),
		"success": result.Success,
		"failed":  result.Failed,
	})

	edit := tgbotapi.NewEditMessageText(
		message.Chat.ID,
		sent.MessageID,
		"✅ تم الإرسال بنجاح إلى "+itoa(result.Success)+" مستخدم.\n❌ فشل الإرسال إلى "+itoa(result.Failed)+" مستخدم.",
	)
	bot.Send(edit)
}

// HandleBanCommand handles the /ban command.
func HandleBanCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := archiveContext(bot)
	userID := message.From.ID

	allowed, _ := services.Can(ctx, userID, "manage_users")
	if !allowed {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية إدارة المستخدمين.")
		bot.Send(msg)
		return
	}

	args := message.CommandArguments()
	if args == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "👤 استخدم: /ban <user_id>")
		bot.Send(msg)
		return
	}

	uid := parseInt64(args)
	if uid == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ يجب إرسال ID رقمي.")
		bot.Send(msg)
		return
	}

	canManage, _ := services.CanManage(ctx, userID, uid)
	if !canManage {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية حظر هذا المستخدم!")
		bot.Send(msg)
		return
	}

	if err := services.Ban(ctx, uid); err != nil {
		log.Printf("ban failed: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تنفيذ الحظر."))
		return
	}
	services.LogAdminAction(ctx, userID, "ban", map[string]interface{}{"target_id": uid})
	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ تم حظر المستخدم "+itoa64(uid))
	bot.Send(msg)
}

// HandleUnbanCommand handles the /unban command.
func HandleUnbanCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := archiveContext(bot)
	userID := message.From.ID

	allowed, _ := services.Can(ctx, userID, "manage_users")
	if !allowed {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية إدارة المستخدمين.")
		bot.Send(msg)
		return
	}

	args := message.CommandArguments()
	if args == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "👤 استخدم: /unban <user_id>")
		bot.Send(msg)
		return
	}

	uid := parseInt64(args)
	if uid == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ يجب إرسال ID رقمي.")
		bot.Send(msg)
		return
	}

	canManage, _ := services.CanManage(ctx, userID, uid)
	if !canManage {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⛔ لا تملك صلاحية إدارة هذا المستخدم.")
		bot.Send(msg)
		return
	}
	if err := services.Unban(ctx, uid); err != nil {
		log.Printf("unban failed: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ تعذر تنفيذ إلغاء الحظر."))
		return
	}
	services.LogAdminAction(ctx, userID, "unban", map[string]interface{}{"target_id": uid})
	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ تم إلغاء حظر المستخدم "+itoa64(uid))
	bot.Send(msg)
}

// HandleTextMessage handles text messages based on user's awaiting state.
func HandleTextMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := archiveContext(bot)
	userID := message.From.ID
	state := GetStateForBot(bot, userID)
	state.Mu.Lock()
	awaiting := state.Awaiting
	if awaiting != nil {
		copyAwaiting := *awaiting
		awaiting = &copyAwaiting
	}
	state.Mu.Unlock()

	if awaiting == nil {
		return
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ يرجى إرسال نص صالح.")
		bot.Send(msg)
		return
	}

	switch awaiting.Type {
	case "new_cat":
		catID, err := services.CreateCategory(ctx, text)
		if err != nil {
			log.Printf("Create category error: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ حدث خطأ، يرجى المحاولة مرة أخرى.")
			bot.Send(msg)
		} else {
			services.LogAdminAction(ctx, userID, "create_category", map[string]interface{}{"name": text, "cat_id": catID})
			msg := tgbotapi.NewMessage(message.Chat.ID, "✅ تم إنشاء التصنيف: "+text)
			bot.Send(msg)
		}
		ClearAwaitingForBot(bot, userID)

	case "new_sub":
		catID := awaiting.CatID
		subID, err := services.CreateSubject(ctx, text, catID)
		if err != nil {
			log.Printf("Create subject error: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ حدث خطأ، يرجى المحاولة مرة أخرى.")
			bot.Send(msg)
		} else {
			services.LogAdminAction(ctx, userID, "create_subject", map[string]interface{}{"name": text, "cat_id": catID, "sub_id": subID})
			msg := tgbotapi.NewMessage(message.Chat.ID, "✅ تم إنشاء المادة: "+text)
			bot.Send(msg)
		}
		ClearAwaitingForBot(bot, userID)

	case "broadcast":
		allowed, _ := services.Can(ctx, userID, "broadcast")
		if !allowed {
			ClearAwaitingForBot(bot, userID)
			return
		}
		delay := services.GetBroadcastDelay()
		result := services.SendBroadcast(ctx, bot, text, delay)
		services.LogAdminAction(ctx, userID, "broadcast", map[string]interface{}{
			"text":    truncate(text, 100),
			"success": result.Success,
			"failed":  result.Failed,
		})
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"✅ تم الإرسال بنجاح إلى "+itoa(result.Success)+" مستخدم.\n❌ فشل الإرسال إلى "+itoa(result.Failed)+" مستخدم.",
		)
		bot.Send(msg)
		ClearAwaitingForBot(bot, userID)

	case "add_admin":
		allowed, _ := services.Can(ctx, userID, "manage_admins")
		if !allowed {
			ClearAwaitingForBot(bot, userID)
			return
		}
		uid := parseInt64(text)
		if uid == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ يجب إرسال ID رقمي.")
			bot.Send(msg)
			return
		}
		target, _ := services.GetUser(ctx, uid)
		username, firstName := "", ""
		if target != nil {
			username, firstName = target.Username, target.FirstName
		}
		err := services.AddAdmin(ctx, uid, username, firstName, "admin")
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ حدث خطأ.")
			bot.Send(msg)
		} else {
			services.LogAdminAction(ctx, userID, "add_admin", map[string]interface{}{"target_id": uid})
			msg := tgbotapi.NewMessage(message.Chat.ID, "✅ تم إضافة الأدمن: "+itoa64(uid))
			bot.Send(msg)
		}
		ClearAwaitingForBot(bot, userID)

	case "welcome_text":
		err := services.SetWelcomeMessage(ctx, text)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ حدث خطأ.")
			bot.Send(msg)
		} else {
			services.LogAdminAction(ctx, userID, "set_welcome", map[string]interface{}{"text": truncate(text, 100), "photo": false})
			msg := tgbotapi.NewMessage(message.Chat.ID, "✅ تم تحديث نص رسالة الترحيب.")
			bot.Send(msg)
		}
		ClearAwaitingForBot(bot, userID)

	default:
		// unknown or "upload" — don't clear
		return
	}
}

// HandleFileUpload handles file/media uploads from admins.
func HandleFileUpload(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	ctx := archiveContext(bot)
	userID := message.From.ID
	state := GetStateForBot(bot, userID)

	// Handle welcome photo upload
	state.Mu.Lock()
	awaitingType := ""
	if state.Awaiting != nil {
		awaitingType = state.Awaiting.Type
	}
	state.Mu.Unlock()
	if awaitingType == "welcome_photo" {
		allowed, _ := services.Can(ctx, userID, "manage_settings")
		if !allowed {
			return
		}
		if message.Photo != nil && len(message.Photo) > 0 {
			largest := message.Photo[len(message.Photo)-1]
			_ = services.SetWelcomePhoto(ctx, largest.FileID)
			services.LogAdminAction(ctx, userID, "set_welcome", map[string]interface{}{"text": nil, "photo": true})
			ClearAwaitingForBot(bot, userID)
			msg := tgbotapi.NewMessage(message.Chat.ID, "✅ تم تحديث صورة الترحيب.")
			bot.Send(msg)
		}
		return
	}

	// Only admins can upload files
	if awaitingType != "" && awaitingType != "upload" {
		return
	}
	allowed, _ := services.Can(ctx, userID, "manage_content")
	if !allowed {
		return
	}

	// Initialize upload state if needed
	state.Mu.Lock()
	if state.Awaiting == nil {
		state.Awaiting = &models.AwaitingState{Type: "upload"}
		state.PendingUploads = nil
		state.UploadLoc = nil
	}
	state.Mu.Unlock()

	fileID, fileName, fileType, fileSize := extractFile(message)
	if fileID == "" {
		return
	}

	state.Mu.Lock()
	state.PendingUploads = append(state.PendingUploads, models.PendingUpload{TelegramFileID: fileID, Name: fileName, FileType: fileType, ChatID: message.Chat.ID, MessageID: message.MessageID, FileSize: fileSize})
	startTimer := state.UploadLoc == nil && !state.UploadTimerActive
	if startTimer {
		state.UploadTimerActive = true
	}
	state.Mu.Unlock()
	if startTimer {
		go autoFinishUpload(bot, userID, state)
	}
}

func extractFile(msg *tgbotapi.Message) (string, string, string, int64) {
	if msg.Document != nil {
		name := msg.Document.FileName
		if name == "" {
			name = "ملف"
		}
		return msg.Document.FileID, name, "document", int64(msg.Document.FileSize)
	}
	if msg.Video != nil {
		name := msg.Video.FileName
		if name == "" {
			name = "فيديو"
		}
		return msg.Video.FileID, name, "video", int64(msg.Video.FileSize)
	}
	if msg.Audio != nil {
		name := msg.Audio.FileName
		if name == "" {
			name = "ملف صوتي"
		}
		return msg.Audio.FileID, name, "audio", int64(msg.Audio.FileSize)
	}
	if msg.Photo != nil && len(msg.Photo) > 0 {
		largest := msg.Photo[len(msg.Photo)-1]
		return largest.FileID, "صورة", "photo", int64(largest.FileSize)
	}
	if msg.Voice != nil {
		return msg.Voice.FileID, "رسالة صوتية", "voice", int64(msg.Voice.FileSize)
	}
	if msg.Animation != nil {
		return msg.Animation.FileID, "GIF", "animation", int64(msg.Animation.FileSize)
	}
	return "", "", "", 0
}
