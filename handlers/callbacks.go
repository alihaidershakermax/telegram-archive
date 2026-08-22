package handlers

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/config"
	"telegram-archive-bot/keyboards"
	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
	"telegram-archive-bot/utils"
)

// HandleCallback routes all callback queries.
func HandleCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	ctx := archiveContext(bot)
	data := query.Data
	userID := query.From.ID
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID
	hasPhoto := query.Message.Photo != nil && len(query.Message.Photo) > 0

	log.Printf("Callback: data=%q user_id=%d", data, userID)

	// Check ban/mute
	if services.IsBanned(ctx, userID) {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ أنت محظور من استخدام البوت.")
		bot.Send(cb)
		return
	}
	if services.IsMuted(ctx, userID) {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "🔇 أنت مكتوم من استخدام البوت.")
		bot.Send(cb)
		return
	}

	isAdmin, _ := services.IsAdmin(ctx, userID)
	if !isAdmin && services.IsMaintenanceEnabled(ctx) {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "🔧 البوت تحت الصيانة حالياً")
		bot.Send(cb)
		return
	}

	// Answer callback
	cb := tgbotapi.NewCallback(query.ID, "")
	bot.Send(cb)

	// Route callbacks
	switch {
	case data == "back_main":
		welcome := services.GetWelcomeSettings(ctx)
		kb := MainMenuKeyboardForUser(bot, userID, isAdmin)
		utils.EditOrSend(bot, query.ID, chatID, msgID, welcome.Message, &kb, hasPhoto)

	case data == "view_archive" || data == "back_cats":
		showCategories(bot, chatID, msgID, hasPhoto)

	case strings.HasPrefix(data, "cat_"):
		catID := parseID(data, "cat_")
		showSubjects(bot, chatID, msgID, catID, hasPhoto)

	case strings.HasPrefix(data, "back_subs_"):
		catID := parseID(data, "back_subs_")
		showSubjects(bot, chatID, msgID, catID, hasPhoto)

	case strings.HasPrefix(data, "sub_"):
		subID := parseID(data, "sub_")
		showFiles(bot, chatID, msgID, subID, hasPhoto)

	case strings.HasPrefix(data, "back_files_"):
		subID := parseID(data, "back_files_")
		showFiles(bot, chatID, msgID, subID, hasPhoto)

	case strings.HasPrefix(data, "file_"):
		fileID := parseID(data, "file_")
		showFileDetails(bot, chatID, msgID, fileID, hasPhoto)

	case strings.HasPrefix(data, "download_"):
		fileID := parseID(data, "download_")
		downloadFileAsDocument(bot, query, fileID)

	case strings.HasPrefix(data, "share_"):
		fileID := parseID(data, "share_")
		shareFile(bot, query, fileID, hasPhoto)

	case data == "about":
		showAbout(bot, query, hasPhoto)

	case data == "factory_info":
		HandleFactoryInfoCallback(bot, query)

	case data == "noop":

		// do nothing

	// ── Admin callbacks ─────────────────────────────────
	case data == "panel" || data == "adm_back":
		if !isAdmin {
			return
		}
		perms, _ := services.GetUserPermissions(ctx, userID)
		kb := keyboards.AdminPanelKeyboard(perms)
		utils.EditOrSend(bot, query.ID, chatID, msgID, "⚙️ لوحة التحكم", &kb, hasPhoto)

	case data == "panel_stats":
		if ok, _ := services.Can(ctx, userID, "view_stats"); !ok {
			return
		}
		showStats(bot, query, hasPhoto)

	case strings.HasPrefix(data, "panel_users"):
		if ok, _ := services.Can(ctx, userID, "manage_users"); !ok {
			return
		}
		showUsers(bot, query, data, hasPhoto)

	case strings.HasPrefix(data, "panel_user_"):
		if ok, _ := services.Can(ctx, userID, "manage_users"); !ok {
			return
		}
		uid := parseID(data, "panel_user_")
		showUserProfile(bot, query, int64(uid), hasPhoto)

	case strings.HasPrefix(data, "panel_ban_"):
		if ok, _ := services.Can(ctx, userID, "manage_users"); !ok {
			return
		}
		uid := parseID(data, "panel_ban_")
		adminBanUser(bot, query, int64(uid), hasPhoto)

	case strings.HasPrefix(data, "panel_unban_"):
		if ok, _ := services.Can(ctx, userID, "manage_users"); !ok {
			return
		}
		uid := parseID(data, "panel_unban_")
		adminUnbanUser(bot, query, int64(uid), hasPhoto)

	case strings.HasPrefix(data, "panel_mute_"):
		if ok, _ := services.Can(ctx, userID, "manage_users"); !ok {
			return
		}
		uid := parseID(data, "panel_mute_")
		adminMuteUser(bot, query, int64(uid), hasPhoto)

	case strings.HasPrefix(data, "panel_unmute_"):
		if ok, _ := services.Can(ctx, userID, "manage_users"); !ok {
			return
		}
		uid := parseID(data, "panel_unmute_")
		adminUnmuteUser(bot, query, int64(uid), hasPhoto)

	case data == "panel_admins":
		if ok, _ := services.Can(ctx, userID, "manage_admins"); !ok {
			return
		}
		showAdmins(bot, query, hasPhoto)

	case data == "panel_add_admin":
		if ok, _ := services.Can(ctx, userID, "manage_admins"); !ok {
			return
		}
		WithStateForBot(bot, userID, func(state *models.UserState) { state.Awaiting = &models.AwaitingState{Type: "add_admin"} })
		utils.EditOrSend(bot, query.ID, chatID, msgID, "👑 أرسل ID المستخدم لإضافته كأدمن (بعد الإضافة سيصبح «🥈 أدمن»).", nil, hasPhoto)

	case strings.HasPrefix(data, "panel_rank_"):
		if ok, _ := services.Can(ctx, userID, "manage_admins"); !ok {
			return
		}
		uid := parseID(data, "panel_rank_")
		showRankPicker(bot, query, int64(uid), hasPhoto)

	case strings.HasPrefix(data, "panel_setrank_"):
		if ok, _ := services.Can(ctx, userID, "manage_admins"); !ok {
			return
		}
		// panel_setrank_UID_RANK
		rest := strings.TrimPrefix(data, "panel_setrank_")
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) == 2 {
			uid := parseInt64(parts[0])
			rank := parts[1]
			if uid != 0 {
				adminSetRank(bot, query, uid, rank, hasPhoto)
			}
		}

	case strings.HasPrefix(data, "panel_rm_admin_"):
		if ok, _ := services.Can(ctx, userID, "manage_admins"); !ok {
			return
		}
		uid := parseID(data, "panel_rm_admin_")
		adminRemoveAdmin(bot, query, int64(uid), hasPhoto)

	case data == "panel_content":
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		showContentCategories(bot, chatID, msgID, hasPhoto)

	case strings.HasPrefix(data, "panel_content_cat_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		catID := parseID(data, "panel_content_cat_")
		showContentSubjects(bot, chatID, msgID, catID, hasPhoto)

	case strings.HasPrefix(data, "panel_content_sub_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		subID := parseID(data, "panel_content_sub_")
		showContentFiles(bot, chatID, msgID, subID, hasPhoto)

	case data == "panel_new_cat":
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		WithStateForBot(bot, userID, func(state *models.UserState) { state.Awaiting = &models.AwaitingState{Type: "new_cat"} })
		utils.EditOrSend(bot, query.ID, chatID, msgID, "📝 أرسل اسم التصنيف الجديد:", nil, hasPhoto)

	case strings.HasPrefix(data, "panel_new_sub_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		catID := parseID(data, "panel_new_sub_")
		WithStateForBot(bot, userID, func(state *models.UserState) { state.Awaiting = &models.AwaitingState{Type: "new_sub", CatID: catID} })
		utils.EditOrSend(bot, query.ID, chatID, msgID, "📝 أرسل اسم المادة الجديدة:", nil, hasPhoto)

	case strings.HasPrefix(data, "panel_del_sub_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		subID := parseID(data, "panel_del_sub_")
		adminDeleteSubject(bot, query, subID, hasPhoto)

	case strings.HasPrefix(data, "panel_del_file_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		fileID := parseID(data, "panel_del_file_")
		adminDeleteFile(bot, query, fileID, hasPhoto)

	case strings.HasPrefix(data, "panel_del_cat_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		catID := parseID(data, "panel_del_cat_")
		adminDeleteCategory(bot, query, catID, hasPhoto)

	case strings.HasPrefix(data, "panel_up_cat_") || strings.HasPrefix(data, "panel_down_cat_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		dir := "up"
		prefix := "panel_up_cat_"
		if strings.HasPrefix(data, "panel_down_cat_") {
			dir = "down"
			prefix = "panel_down_cat_"
		}
		catID := parseID(data, prefix)
		adminMoveCategory(bot, query, catID, dir, hasPhoto)

	case strings.HasPrefix(data, "panel_up_sub_") || strings.HasPrefix(data, "panel_down_sub_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		dir := "up"
		prefix := "panel_up_sub_"
		if strings.HasPrefix(data, "panel_down_sub_") {
			dir = "down"
			prefix = "panel_down_sub_"
		}
		subID := parseID(data, prefix)
		adminMoveSubject(bot, query, subID, dir, hasPhoto)

	case strings.HasPrefix(data, "panel_up_file_") || strings.HasPrefix(data, "panel_down_file_"):
		if ok, _ := services.Can(ctx, userID, "manage_content"); !ok {
			return
		}
		dir := "up"
		prefix := "panel_up_file_"
		if strings.HasPrefix(data, "panel_down_file_") {
			dir = "down"
			prefix = "panel_down_file_"
		}
		fileID := parseID(data, prefix)
		adminMoveFile(bot, query, fileID, dir, hasPhoto)

	case data == "panel_broadcast":
		if ok, _ := services.Can(ctx, userID, "broadcast"); !ok {
			return
		}
		WithStateForBot(bot, userID, func(state *models.UserState) { state.Awaiting = &models.AwaitingState{Type: "broadcast"} })
		utils.EditOrSend(bot, query.ID, chatID, msgID, "📢 أرسل نص الإذاعة الآن:", nil, hasPhoto)

	case data == "panel_maint":
		if ok, _ := services.Can(ctx, userID, "manage_maintenance"); !ok {
			return
		}
		adminToggleMaintenance(bot, query, hasPhoto)

	case data == "panel_logs" || strings.HasPrefix(data, "panel_logs_"):
		if ok, _ := services.Can(ctx, userID, "view_logs"); !ok {
			return
		}
		showLogs(bot, query, data, hasPhoto)

	case data == "panel_welcome":
		if ok, _ := services.Can(ctx, userID, "manage_settings"); !ok {
			return
		}
		showWelcomeSettings(bot, query, hasPhoto)

	case data == "panel_welcome_text":
		if ok, _ := services.Can(ctx, userID, "manage_settings"); !ok {
			return
		}
		WithStateForBot(bot, userID, func(state *models.UserState) { state.Awaiting = &models.AwaitingState{Type: "welcome_text"} })
		utils.EditOrSend(bot, query.ID, chatID, msgID, "📝 أرسل نص رسالة الترحيب الجديد:", nil, hasPhoto)

	case data == "panel_welcome_photo":
		if ok, _ := services.Can(ctx, userID, "manage_settings"); !ok {
			return
		}
		WithStateForBot(bot, userID, func(state *models.UserState) { state.Awaiting = &models.AwaitingState{Type: "welcome_photo"} })
		utils.EditOrSend(bot, query.ID, chatID, msgID, "🖼️ أرسل صورة الترحيب الجديدة الآن:", nil, hasPhoto)

	case data == "panel_welcome_preview":
		if ok, _ := services.Can(ctx, userID, "manage_settings"); !ok {
			return
		}
		showWelcomePreview(bot, query)

	case data == "start_upload":
		canUpload, _ := services.Can(ctx, userID, "manage_content")
		if !canUpload {
			return
		}
		WithStateForBot(bot, userID, func(state *models.UserState) {
			state.Awaiting = &models.AwaitingState{Type: "upload"}
			state.PendingUploads = nil
			state.UploadLoc = nil
		})
		msg := tgbotapi.NewMessage(chatID, "📤 أرسل الملفات الآن (مستند/فيديو/صوت/صورة).\nبعد إرسال الملفات سيظهر اختيار التصنيف تلقائياً.")
		bot.Send(msg)

	case data == "cancel_upload":
		canUpload, _ := services.Can(ctx, userID, "manage_content")
		if !canUpload {
			return
		}
		WithStateForBot(bot, userID, clearUploadState)
		msg := tgbotapi.NewMessage(chatID, "🚫 تم إلغاء الرفع.")
		bot.Send(msg)

	case strings.HasPrefix(data, "uploc_cat_"):
		canUpload, _ := services.Can(ctx, userID, "manage_content")
		if !canUpload {
			return
		}
		catID := parseID(data, "uploc_cat_")
		handleUploadCatSelect(bot, query, catID, hasPhoto)

	case strings.HasPrefix(data, "uploc_sub_"):
		canUpload, _ := services.Can(ctx, userID, "manage_content")
		if !canUpload {
			return
		}
		subID := parseID(data, "uploc_sub_")
		handleUploadSubSelect(bot, query, subID)

	case data == "uploc_back_cats":
		canUpload, _ := services.Can(ctx, userID, "manage_content")
		if !canUpload {
			return
		}
		state := GetStateForBot(bot, userID)
		state.Mu.Lock()
		count := len(state.PendingUploads)
		state.Mu.Unlock()
		if count == 0 {
			return
		}
		cats, _ := services.GetCategories(ctx)
		kb := keyboards.UploadCategoriesKeyboard(cats)
		text := fmt.Sprintf("📂 اختر التصنيف لحفظ (%d) ملف:", count)
		utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
	}
}

// ── User navigation helpers ─────────────────────────────────

func showCategories(bot *tgbotapi.BotAPI, chatID int64, msgID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	cats, _ := services.GetCategories(ctx)
	kb := keyboards.CategoriesKeyboard(cats)
	utils.EditOrSend(bot, "", chatID, msgID, "📂 اختر التصنيف:", &kb, hasPhoto)
}

func showSubjects(bot *tgbotapi.BotAPI, chatID int64, msgID, catID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	subs, _ := services.GetSubjects(ctx, &catID)
	if len(subs) == 0 {
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع للتصنيفات", "back_cats"),
			),
		)
		utils.EditOrSend(bot, "", chatID, msgID, "❌ لا توجد مواد في هذا التصنيف.", &kb, hasPhoto)
		return
	}
	kb := keyboards.SubjectsKeyboard(subs)
	utils.EditOrSend(bot, "", chatID, msgID, "📁 اختر المادة:", &kb, hasPhoto)
}

func showFiles(bot *tgbotapi.BotAPI, chatID int64, msgID, subID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	files, _ := services.GetFiles(ctx, subID)
	sub, _ := services.GetSubjectByID(ctx, subID)
	catID := 0
	if sub != nil {
		catID = sub.CategoryID
	}
	if len(files) == 0 {
		backTarget := "back_cats"
		if catID > 0 {
			backTarget = fmt.Sprintf("back_subs_%d", catID)
		}
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع للمواد", backTarget),
			),
		)
		utils.EditOrSend(bot, "", chatID, msgID, "❌ لا توجد ملفات في هذه المادة.", &kb, hasPhoto)
		return
	}
	kb := keyboards.FilesKeyboard(files, subID, catID)
	utils.EditOrSend(bot, "", chatID, msgID, "📄 اختر الملف:", &kb, hasPhoto)
}

func showFileDetails(bot *tgbotapi.BotAPI, chatID int64, msgID, fileID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	f, err := services.GetFileRow(ctx, fileID)
	if err != nil {
		utils.EditOrSend(bot, "", chatID, msgID, "❌ الملف غير موجود.", nil, hasPhoto)
		return
	}
	backData := "back_cats"
	if f.SubjectID > 0 {
		backData = fmt.Sprintf("back_files_%d", f.SubjectID)
	}
	text := fmt.Sprintf("📄 %s\n━━━━━━━━━━━━━━━\n📂 المادة: %s\n📁 النوع: %s\n⬇️ مرات التحميل: %d\n",
		f.Name, f.SubjectName, f.FileType, f.Downloads)
	kb := keyboards.FileActionsKeyboard(fileID, backData, f.FileType, f.Name)
	utils.EditOrSend(bot, "", chatID, msgID, text, &kb, hasPhoto)
}

func downloadFile(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, fileID int) {
	ctx := archiveContext(bot)
	f, err := services.GetFileRow(ctx, fileID)
	if err != nil {
		return
	}
	services.IncrementDownloads(ctx, fileID)
	sender := storageBot(bot)
	action := tgbotapi.NewChatAction(query.Message.Chat.ID, "upload_document")
	sender.Send(action)
	queued, err := SendStorageFile(ctx, bot, query.Message.Chat.ID, f.TelegramFileID, f.FileType, f.Name)
	if queued {
		bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, "⏳ تعذر الإرسال فوراً، تمت إضافة الملف إلى طابور الإرسال وسيصل تلقائياً عند عودة الخدمة."))
	} else if err != nil {
		// Never retry with the managed bot: Telegram file_id values are bot-scoped.
		log.Printf("storage gateway file send failed for %d: %v", fileID, err)
	}
}

func downloadFileAsDocument(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, fileID int) {
	// Every archive download is delivered as a document, including image files.
	downloadFile(bot, query, fileID)
}

func shareFile(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, fileID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	f, err := services.GetFileRow(ctx, fileID)
	if err != nil {
		utils.EditOrSend(bot, query.ID, chatID, msgID, "❌ الملف غير موجود.", nil, hasPhoto)
		return
	}
	// Keep the deep link on the current bot so its scoped shared_files
	// collection can resolve the hash; only the actual media send uses storageBot.
	botInfo, _ := bot.GetMe()
	shareHash, err := services.CreateShareLink(ctx, f.TelegramFileID, f.FileType, query.From.ID, 7)
	if err != nil {
		return
	}
	link := fmt.Sprintf("https://t.me/%s?start=share_%s", botInfo.UserName, shareHash)
	shareURL := fmt.Sprintf("https://t.me/share/url?url=%s&text=%s", url.QueryEscape(link), url.QueryEscape(f.Name))

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📤 مشاركة الملف", shareURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع", fmt.Sprintf("file_%d", fileID)),
		),
	)
	text := fmt.Sprintf("📤 اضغط على الزر لاختيار محادثة وإرسال رابط الملف:\n\n📄 %s", f.Name)
	utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
}

func showAbout(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, hasPhoto bool) {
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع", "back_main"),
		),
	)

	if config.AboutPhoto != "" {
		// Delete old message and send photo
		del := tgbotapi.NewDeleteMessage(chatID, msgID)
		bot.Send(del)
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(config.AboutPhoto))
		photo.Caption = config.AboutDevText
		photo.ReplyMarkup = kb
		_, err := bot.Send(photo)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, config.AboutDevText)
			msg.ReplyMarkup = kb
			bot.Send(msg)
		}
		return
	}
	utils.EditOrSend(bot, query.ID, chatID, msgID, config.AboutDevText, &kb, hasPhoto)
}

// ── Admin callback helpers ──────────────────────────────────

func showStats(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	users, _ := services.GetAllUsersCount(ctx)
	files, _ := services.GetAllFilesCount(ctx)
	subjects, _ := services.GetAllSubjectsCount(ctx)
	downloads, _ := services.GetTotalDownloads(ctx)
	top, _ := services.GetTopFiles(ctx, 5)

	text := fmt.Sprintf("📊 إحصائيات البوت\n━━━━━━━━━━━━━━━\n👥 المستخدمين: %d\n📂 المواد: %d\n📄 الملفات: %d\n⬇️ إجمالي التحميلات: %d\n",
		users, subjects, files, downloads)

	if len(top) > 0 {
		text += "━━━━━━━━━━━━━━━\n🏆 أكثر الملفات تحميلاً:\n"
		for i, t := range top {
			text += fmt.Sprintf("%d. %s — %d ⬇️\n", i+1, t.Name, t.Downloads)
		}
	}

	perms, _ := services.GetUserPermissions(ctx, query.From.ID)
	kb := keyboards.AdminPanelKeyboard(perms)
	utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
}

func showUsers(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, data string, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	page := 1
	parts := strings.Split(data, "_")
	if len(parts) >= 3 {
		if p, err := strconv.Atoi(parts[2]); err == nil {
			page = p
		}
	}

	perPage := 10
	users, _ := services.GetUsersPage(ctx, page, perPage)
	total, _ := services.GetUsersCount(ctx)
	totalPages := int(total+int64(perPage)-1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	state := GetStateForBot(bot, query.From.ID)
	state.UsersPage = page

	kb := keyboards.AdminUsersKeyboard(users, page, totalPages)
	text := fmt.Sprintf("👥 المستخدمين (صفحة %d/%d)", page, totalPages)
	utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
}

func showUserProfile(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID
	actorID := query.From.ID

	targetRank, _ := services.GetUserRank(ctx, uid)
	actorRank, _ := services.GetUserRank(ctx, actorID)
	actorLevel := services.RankLevelValue(actorRank)
	targetLevel := services.RankLevelValue(targetRank)

	user, _ := services.GetUser(ctx, uid)
	name := "؟"
	banned := false
	muted := false
	username := "-"
	createdAt := "-"
	lastSeen := "-"

	if user != nil {
		if user.FirstName != "" {
			name = user.FirstName
		}
		banned = user.IsBanned
		muted = user.IsMuted
		if user.Username != "" {
			username = "@" + user.Username
		}
		if !user.CreatedAt.IsZero() {
			createdAt = user.CreatedAt.Format("2006-01-02 15:04")
		}
		if !user.LastSeenAt.IsZero() {
			lastSeen = user.LastSeenAt.Format("2006-01-02 15:04")
		}
	}

	rankLine := services.RankLabel(targetRank)
	if targetRank == "" {
		rankLine = "👤 مستخدم عادي"
	}

	bannedStr := "🟢 غير محظور"
	if banned {
		bannedStr = "🔴 محظور"
	}
	mutedStr := "🔊 غير مكتوم"
	if muted {
		mutedStr = "🔇 مكتوم"
	}

	text := fmt.Sprintf("👤 ملف المستخدم\n━━━━━━━━━━━━━━━\n🆔 ID: %d\n📋 الاسم: %s\n🔗 المعرف: %s\n📅 تاريخ الانضمام: %s\n🕑 آخر ظهور: %s\n━━━━━━━━━━━━━━━\n%s\n%s\n%s",
		uid, name, username, createdAt, lastSeen, bannedStr, mutedStr, rankLine)

	state := GetStateForBot(bot, actorID)
	page := state.UsersPage
	if page == 0 {
		page = 1
	}

	canUsers, _ := services.Can(ctx, actorID, "manage_users")
	canAdmins, _ := services.Can(ctx, actorID, "manage_admins")
	kb := keyboards.AdminUserProfileKeyboard(uid, banned, muted, targetRank,
		canUsers && actorLevel > targetLevel,
		canAdmins && actorLevel > targetLevel,
		page)
	utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
}

func adminBanUser(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, hasPhoto bool) {
	ctx := archiveContext(bot)
	actorID := query.From.ID
	canManage, _ := services.CanManage(ctx, actorID, uid)
	if !canManage {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا تملك صلاحية إدارة هذا المستخدم!")
		bot.Send(cb)
		return
	}
	services.Ban(ctx, uid)
	services.LogAdminAction(ctx, actorID, "ban_user", map[string]interface{}{"target_id": uid})
	showUserProfile(bot, query, uid, hasPhoto)
}

func adminUnbanUser(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, hasPhoto bool) {
	ctx := archiveContext(bot)
	actorID := query.From.ID
	canManage, _ := services.CanManage(ctx, actorID, uid)
	if !canManage {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا تملك صلاحية إدارة هذا المستخدم!")
		bot.Send(cb)
		return
	}
	services.Unban(ctx, uid)
	services.LogAdminAction(ctx, actorID, "unban_user", map[string]interface{}{"target_id": uid})
	showUserProfile(bot, query, uid, hasPhoto)
}

func adminMuteUser(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, hasPhoto bool) {
	ctx := archiveContext(bot)
	actorID := query.From.ID
	canManage, _ := services.CanManage(ctx, actorID, uid)
	if !canManage {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا تملك صلاحية إدارة هذا المستخدم!")
		bot.Send(cb)
		return
	}
	services.Mute(ctx, uid)
	services.LogAdminAction(ctx, actorID, "mute_user", map[string]interface{}{"target_id": uid})
	showUserProfile(bot, query, uid, hasPhoto)
}

func adminUnmuteUser(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, hasPhoto bool) {
	ctx := archiveContext(bot)
	actorID := query.From.ID
	canManage, _ := services.CanManage(ctx, actorID, uid)
	if !canManage {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا تملك صلاحية إدارة هذا المستخدم!")
		bot.Send(cb)
		return
	}
	services.Unmute(ctx, uid)
	services.LogAdminAction(ctx, actorID, "unmute_user", map[string]interface{}{"target_id": uid})
	showUserProfile(bot, query, uid, hasPhoto)
}

func showAdmins(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	admins, _ := services.GetAdmins(ctx)
	actorRank, _ := services.GetUserRank(ctx, query.From.ID)
	actorLevel := services.RankLevelValue(actorRank)

	text := "👑 قائمة الأدمنز:\n━━━━━━━━━━━━━━━\n"
	for _, a := range admins {
		name := a.FirstName
		if name == "" {
			name = a.Username
		}
		if name == "" {
			name = fmt.Sprintf("ID:%d", a.ID)
		}
		rank := a.Rank
		if rank == "" {
			rank = "admin"
		}
		text += fmt.Sprintf("\n%s — %s", services.RankLabel(rank), name)
	}

	kb := keyboards.AdminAdminsKeyboard(admins, actorLevel)
	utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
}

func showRankPicker(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	actorRank, _ := services.GetUserRank(ctx, query.From.ID)
	actorLevel := services.RankLevelValue(actorRank)
	targetRank, _ := services.GetUserRank(ctx, uid)
	targetLevel := services.RankLevelValue(targetRank)

	if targetLevel >= actorLevel {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا يمكنك تغيير رتبة هذا المستخدم!")
		bot.Send(cb)
		return
	}

	state := GetStateForBot(bot, query.From.ID)
	page := state.UsersPage
	if page == 0 {
		page = 1
	}

	kb := keyboards.AdminRankPickerKeyboard(uid, actorLevel, page)
	utils.EditOrSend(bot, query.ID, chatID, msgID, "🎖️ اختر الرتبة الجديدة:", &kb, hasPhoto)
}

func adminSetRank(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, rank string, hasPhoto bool) {
	ctx := archiveContext(bot)
	actorID := query.From.ID

	actorRank, _ := services.GetUserRank(ctx, actorID)
	actorLevel := services.RankLevelValue(actorRank)
	targetRank, _ := services.GetUserRank(ctx, uid)
	targetLevel := services.RankLevelValue(targetRank)

	if actorID == uid {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا يمكنك تغيير رتبتك بنفسك!")
		bot.Send(cb)
		return
	}

	if rank == "none" {
		if targetLevel >= actorLevel {
			cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا يمكنك إزالة هذا المستخدم!")
			bot.Send(cb)
			return
		}
		services.SetAdminRank(ctx, uid, "", "", "")
		services.LogAdminAction(ctx, actorID, "set_rank", map[string]interface{}{"target_id": uid, "rank": nil})
	} else {
		rankLevel, ok := services.RankLevels[rank]
		if !ok || rankLevel >= actorLevel {
			cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا تملك صلاحية تعيين هذه الرتبة!")
			bot.Send(cb)
			return
		}
		user, _ := services.GetUser(ctx, uid)
		username := ""
		firstName := ""
		if user != nil {
			username = user.Username
			firstName = user.FirstName
		}
		services.SetAdminRank(ctx, uid, rank, username, firstName)
		services.LogAdminAction(ctx, actorID, "set_rank", map[string]interface{}{"target_id": uid, "rank": rank})
	}

	showUserProfile(bot, query, uid, hasPhoto)
}

func adminRemoveAdmin(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, uid int64, hasPhoto bool) {
	ctx := archiveContext(bot)
	actorID := query.From.ID
	canManage, _ := services.CanManage(ctx, actorID, uid)
	if !canManage {
		cb := tgbotapi.NewCallbackWithAlert(query.ID, "⛔ لا تملك صلاحية إدارة هذا الأدمن!")
		bot.Send(cb)
		return
	}
	services.RemoveAdmin(ctx, uid)
	services.LogAdminAction(ctx, actorID, "remove_admin", map[string]interface{}{"target_id": uid})
	showAdmins(bot, query, hasPhoto)
}

func showContentCategories(bot *tgbotapi.BotAPI, chatID int64, msgID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	cats, _ := services.GetCategories(ctx)
	kb := keyboards.AdminContentKeyboard(cats)
	utils.EditOrSend(bot, "", chatID, msgID, "📁 إدارة المحتوى - اختر تصنيفاً:", &kb, hasPhoto)
}

func showContentSubjects(bot *tgbotapi.BotAPI, chatID int64, msgID, catID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	subs, _ := services.GetSubjects(ctx, &catID)
	kb := keyboards.AdminContentSubjectsKeyboard(subs, catID)
	utils.EditOrSend(bot, "", chatID, msgID, "📁 اختر مادة:", &kb, hasPhoto)
}

func showContentFiles(bot *tgbotapi.BotAPI, chatID int64, msgID, subID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	files, _ := services.GetFiles(ctx, subID)
	sub, _ := services.GetSubjectByID(ctx, subID)
	catID := 0
	if sub != nil {
		catID = sub.CategoryID
	}
	kb := keyboards.AdminContentFilesKeyboard(files, subID, catID)
	utils.EditOrSend(bot, "", chatID, msgID, "📁 الملفات في هذه المادة:", &kb, hasPhoto)
}

func adminDeleteSubject(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, subID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	sub, _ := services.GetSubjectByID(ctx, subID)
	name := fmt.Sprintf("%d", subID)
	if sub != nil {
		name = sub.Name
	}
	services.DeleteSubject(ctx, subID)
	services.LogAdminAction(ctx, query.From.ID, "delete_subject", map[string]interface{}{"sub_id": subID, "name": name})
	showContentCategories(bot, query.Message.Chat.ID, query.Message.MessageID, hasPhoto)
}

func adminDeleteFile(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, fileID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	f, _ := services.GetFileRow(ctx, fileID)
	name := fmt.Sprintf("%d", fileID)
	subjectID := 0
	if f != nil {
		name = f.Name
		subjectID = f.SubjectID
	}
	services.DeleteFile(ctx, fileID)
	services.LogAdminAction(ctx, query.From.ID, "delete_file", map[string]interface{}{"file_id": fileID, "name": name})
	if subjectID > 0 {
		showContentFiles(bot, query.Message.Chat.ID, query.Message.MessageID, subjectID, hasPhoto)
	}
}

func adminDeleteCategory(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, catID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	cat, _ := services.GetCategoryByID(ctx, catID)
	name := fmt.Sprintf("%d", catID)
	if cat != nil {
		name = cat.Name
	}
	services.DeleteCategory(ctx, catID)
	services.LogAdminAction(ctx, query.From.ID, "delete_category", map[string]interface{}{"cat_id": catID, "name": name})
	showContentCategories(bot, query.Message.Chat.ID, query.Message.MessageID, hasPhoto)
}

func adminMoveCategory(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, catID int, direction string, hasPhoto bool) {
	ctx := archiveContext(bot)
	moved, _ := services.MoveCategory(ctx, catID, direction)
	if moved {
		services.LogAdminAction(ctx, query.From.ID, "move_category", map[string]interface{}{"cat_id": catID, "direction": direction})
	}
	showContentCategories(bot, query.Message.Chat.ID, query.Message.MessageID, hasPhoto)
}

func adminMoveSubject(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, subID int, direction string, hasPhoto bool) {
	ctx := archiveContext(bot)
	sub, _ := services.GetSubjectByID(ctx, subID)
	moved, _ := services.MoveSubject(ctx, subID, direction)
	if moved {
		services.LogAdminAction(ctx, query.From.ID, "move_subject", map[string]interface{}{"sub_id": subID, "direction": direction})
	}
	catID := 0
	if sub != nil {
		catID = sub.CategoryID
	}
	if catID > 0 {
		showContentSubjects(bot, query.Message.Chat.ID, query.Message.MessageID, catID, hasPhoto)
	} else {
		showContentCategories(bot, query.Message.Chat.ID, query.Message.MessageID, hasPhoto)
	}
}

func adminMoveFile(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, fileID int, direction string, hasPhoto bool) {
	ctx := archiveContext(bot)
	f, _ := services.GetFileRow(ctx, fileID)
	moved, _ := services.MoveFile(ctx, fileID, direction)
	if moved {
		services.LogAdminAction(ctx, query.From.ID, "move_file", map[string]interface{}{"file_id": fileID, "direction": direction})
	}
	subjectID := 0
	if f != nil {
		subjectID = f.SubjectID
	}
	if subjectID > 0 {
		showContentFiles(bot, query.Message.Chat.ID, query.Message.MessageID, subjectID, hasPhoto)
	} else {
		showContentCategories(bot, query.Message.Chat.ID, query.Message.MessageID, hasPhoto)
	}
}

func adminToggleMaintenance(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	current := services.IsMaintenanceEnabled(ctx)
	newVal := !current
	services.SetMaintenanceMode(ctx, newVal)

	status := "🟢 تم تعطيل الصيانة"
	if newVal {
		status = "🔴 تم تفعيل الصيانة"
	}

	services.LogAdminAction(ctx, query.From.ID, "toggle_maintenance", map[string]interface{}{"enabled": newVal})
	perms, _ := services.GetUserPermissions(ctx, query.From.ID)
	kb := keyboards.AdminPanelKeyboard(perms)
	utils.EditOrSend(bot, query.ID, chatID, msgID, status, &kb, hasPhoto)
}

func showLogs(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, data string, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	page := 1
	parts := strings.Split(data, "_")
	if len(parts) >= 3 {
		if p, err := strconv.Atoi(parts[2]); err == nil {
			page = p
		}
	}

	perPage := 10
	logs, _ := services.GetAdminLogs(ctx, page, perPage)
	total, _ := services.GetAdminLogsCount(ctx)
	totalPages := int(total+int64(perPage)-1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	kb := keyboards.AdminLogsKeyboard(page, totalPages)

	if len(logs) == 0 {
		utils.EditOrSend(bot, query.ID, chatID, msgID, "📋 لا توجد سجلات بعد.", &kb, hasPhoto)
		return
	}

	lines := []string{"📋 سجل النشاط", "━━━━━━━━━━━━━━━"}
	for _, lg := range logs {
		action := services.ActionLabels[lg.Action]
		if action == "" {
			action = lg.Action
		}
		tsStr := "-"
		if !lg.Timestamp.IsZero() {
			tsStr = lg.Timestamp.Format("01-02 15:04")
		}
		extra := ""
		if tid, ok := lg.Details["target_id"]; ok && tid != nil {
			extra = fmt.Sprintf(" (%v)", tid)
		} else if name, ok := lg.Details["name"]; ok && name != nil {
			extra = fmt.Sprintf(" (%v)", name)
		}
		lines = append(lines, fmt.Sprintf("%s | %s%s", tsStr, action, extra))
	}

	utils.EditOrSend(bot, query.ID, chatID, msgID, strings.Join(lines, "\n"), &kb, hasPhoto)
}

func showWelcomeSettings(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	s := services.GetWelcomeSettings(ctx)
	photoStatus := "بدون صورة"
	if s.Photo != "" {
		photoStatus = "🖼️ موجودة"
	}
	msgText := s.Message
	if len([]rune(msgText)) > 200 {
		msgText = string([]rune(msgText)[:200])
	}
	text := fmt.Sprintf("🎨 إعدادات رسالة الترحيب\n━━━━━━━━━━━━━━━\n🖼️ الصورة: %s\n📝 النص:\n%s\n━━━━━━━━━━━━━━━", photoStatus, msgText)

	kb := keyboards.AdminWelcomeKeyboard()
	utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
}

func showWelcomePreview(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID

	s := services.GetWelcomeSettings(ctx)
	if s.Photo != "" {
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(s.Photo))
		photo.Caption = s.Message
		_, err := bot.Send(photo)
		if err == nil {
			return
		}
	}
	msg := tgbotapi.NewMessage(chatID, s.Message)
	bot.Send(msg)
}

func handleUploadCatSelect(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, catID int, hasPhoto bool) {
	ctx := archiveContext(bot)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID
	userID := query.From.ID

	cat, _ := services.GetCategoryByID(ctx, catID)
	if cat == nil {
		utils.EditOrSend(bot, query.ID, chatID, msgID, "❌ التصنيف غير موجود.", nil, hasPhoto)
		return
	}
	subs, _ := services.GetSubjects(ctx, &catID)
	if len(subs) == 0 {
		utils.EditOrSend(bot, query.ID, chatID, msgID, fmt.Sprintf("❌ لا توجد مواد في تصنيف «%s». أضف مادة أولاً.", cat.Name), nil, hasPhoto)
		return
	}

	state := GetStateForBot(bot, userID)
	state.Mu.Lock()
	state.UploadLoc = &models.UploadLocation{CatID: catID}
	state.Mu.Unlock()

	kb := keyboards.UploadSubjectsKeyboard(subs, catID)
	text := fmt.Sprintf("📂 اختر المادة في تصنيف «%s»:", cat.Name)
	utils.EditOrSend(bot, query.ID, chatID, msgID, text, &kb, hasPhoto)
}

func handleUploadSubSelect(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, subID int) {
	ctx := archiveContext(bot)
	userID := query.From.ID

	state := GetStateForBot(bot, userID)
	state.Mu.Lock()
	if state.UploadLoc == nil {
		state.Mu.Unlock()
		return
	}
	catID := state.UploadLoc.CatID
	state.Mu.Unlock()

	subs, _ := services.GetSubjects(ctx, &catID)
	found := false
	for _, s := range subs {
		if s.SubID == subID {
			found = true
			break
		}
	}
	if !found {
		msg := tgbotapi.NewMessage(query.Message.Chat.ID, "❌ المادة غير موجودة.")
		bot.Send(msg)
		return
	}

	state.Mu.Lock()
	if state.UploadLoc == nil {
		state.Mu.Unlock()
		return
	}
	state.UploadLoc.SubID = subID
	state.Mu.Unlock()
	saveAllFiles(bot, query.Message.Chat.ID, userID, state)
}

// ── Parsing helpers ─────────────────────────────────────────

func parseID(data, prefix string) int {
	s := strings.TrimPrefix(data, prefix)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
