package keyboards

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
)

// AdminPanelKeyboard returns the admin panel keyboard based on permissions.
func AdminPanelKeyboard(perms map[string]bool) tgbotapi.InlineKeyboardMarkup {
	var buttons []tgbotapi.InlineKeyboardButton
	if perms["view_stats"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("📊 الإحصائيات", "panel_stats"))
	}
	if perms["manage_users"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("👥 المستخدمين", "panel_users"))
	}
	if perms["manage_admins"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("👑 الإدارة", "panel_admins"))
	}
	if perms["manage_content"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("📁 إدارة المحتوى", "panel_content"))
	}
	if perms["broadcast"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("📢 إذاعة", "panel_broadcast"))
	}
	if perms["manage_settings"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("🎨 رسالة الترحيب", "panel_welcome"))
	}
	if perms["view_logs"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("📋 سجل النشاط", "panel_logs"))
	}
	if perms["manage_maintenance"] {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("🔧 الصيانة", "panel_maint"))
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(buttons); i += 2 {
		end := i + 2
		if end > len(buttons) {
			end = len(buttons)
		}
		rows = append(rows, buttons[i:end])
	}
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminContentKeyboard returns the content management keyboard with categories.
func AdminContentKeyboard(categories []models.Category) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(categories); i += 2 {
		end := i + 2
		if end > len(categories) {
			end = len(categories)
		}
		pair := categories[i:end]

		// Name row
		var nameRow []tgbotapi.InlineKeyboardButton
		for _, c := range pair {
			nameRow = append(nameRow, tgbotapi.NewInlineKeyboardButtonData(c.Name, fmt.Sprintf("panel_content_cat_%d", c.CatID)))
		}
		rows = append(rows, nameRow)

		// Control row (up/down/delete)
		var controlRow []tgbotapi.InlineKeyboardButton
		for _, c := range pair {
			controlRow = append(controlRow,
				tgbotapi.NewInlineKeyboardButtonData("⬆️", fmt.Sprintf("panel_up_cat_%d", c.CatID)),
				tgbotapi.NewInlineKeyboardButtonData("⬇️", fmt.Sprintf("panel_down_cat_%d", c.CatID)),
				tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("panel_del_cat_%d", c.CatID)),
			)
		}
		rows = append(rows, controlRow)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ إضافة تصنيف", "panel_new_cat"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", "adm_back"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminContentSubjectsKeyboard returns the subjects management keyboard.
func AdminContentSubjectsKeyboard(subjects []models.Subject, catID int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(subjects); i += 2 {
		end := i + 2
		if end > len(subjects) {
			end = len(subjects)
		}
		pair := subjects[i:end]

		var nameRow []tgbotapi.InlineKeyboardButton
		for _, s := range pair {
			nameRow = append(nameRow, tgbotapi.NewInlineKeyboardButtonData(s.Name, fmt.Sprintf("panel_content_sub_%d", s.SubID)))
		}
		rows = append(rows, nameRow)

		var controlRow []tgbotapi.InlineKeyboardButton
		for _, s := range pair {
			controlRow = append(controlRow,
				tgbotapi.NewInlineKeyboardButtonData("⬆️", fmt.Sprintf("panel_up_sub_%d", s.SubID)),
				tgbotapi.NewInlineKeyboardButtonData("⬇️", fmt.Sprintf("panel_down_sub_%d", s.SubID)),
				tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("panel_del_sub_%d", s.SubID)),
			)
		}
		rows = append(rows, controlRow)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ إضافة مادة", fmt.Sprintf("panel_new_sub_%d", catID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", "panel_content"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminContentFilesKeyboard returns the files management keyboard.
func AdminContentFilesKeyboard(files []models.File, subjectID, catID int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(files); i += 2 {
		end := i + 2
		if end > len(files) {
			end = len(files)
		}
		pair := files[i:end]

		var nameRow []tgbotapi.InlineKeyboardButton
		for _, f := range pair {
			nameRow = append(nameRow, tgbotapi.NewInlineKeyboardButtonData(f.Name, fmt.Sprintf("file_%d", f.FileID)))
		}
		rows = append(rows, nameRow)

		var controlRow []tgbotapi.InlineKeyboardButton
		for _, f := range pair {
			controlRow = append(controlRow,
				tgbotapi.NewInlineKeyboardButtonData("⬆️", fmt.Sprintf("panel_up_file_%d", f.FileID)),
				tgbotapi.NewInlineKeyboardButtonData("⬇️", fmt.Sprintf("panel_down_file_%d", f.FileID)),
				tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("panel_del_file_%d", f.FileID)),
			)
		}
		rows = append(rows, controlRow)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", fmt.Sprintf("panel_content_cat_%d", catID)),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// UploadCategoriesKeyboard returns the category picker for file uploads.
func UploadCategoriesKeyboard(categories []models.Category) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(categories); i += 2 {
		end := i + 2
		if end > len(categories) {
			end = len(categories)
		}
		pair := categories[i:end]
		var row []tgbotapi.InlineKeyboardButton
		for _, c := range pair {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(c.Name, fmt.Sprintf("uploc_cat_%d", c.CatID)))
		}
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ إلغاء", "cancel_upload"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// UploadSubjectsKeyboard returns the subject picker for file uploads.
func UploadSubjectsKeyboard(subjects []models.Subject, catID int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(subjects); i += 2 {
		end := i + 2
		if end > len(subjects) {
			end = len(subjects)
		}
		pair := subjects[i:end]
		var row []tgbotapi.InlineKeyboardButton
		for _, s := range pair {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(s.Name, fmt.Sprintf("uploc_sub_%d", s.SubID)))
		}
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع", "uploc_back_cats"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminUsersKeyboard returns the paginated users list keyboard.
func AdminUsersKeyboard(users []models.User, page, totalPages int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, u := range users {
		name := u.FirstName
		if name == "" {
			name = u.Username
		}
		if name == "" {
			name = fmt.Sprintf("ID:%d", u.UserID)
		}
		status := "🟢"
		if u.IsBanned {
			status = "🔴"
		} else if u.IsMuted {
			status = "🔇"
		}
		label := fmt.Sprintf("%s %s", status, name)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("panel_user_%d", u.UserID)),
		))
	}

	var navRow []tgbotapi.InlineKeyboardButton
	if page > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️", fmt.Sprintf("panel_users_%d", page-1)))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📄 %d/%d", page, totalPages), "noop"))
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️", fmt.Sprintf("panel_users_%d", page+1)))
	}
	rows = append(rows, navRow)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", "adm_back"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminUserProfileKeyboard returns the user profile action keyboard.
func AdminUserProfileKeyboard(uid int64, banned, muted bool, targetRank string, canManageUsers, canManageAdmins bool, page int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	if targetRank == "owner" {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 المالك", "noop"),
		))
	} else {
		if canManageUsers {
			if banned {
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ إلغاء الحظر", fmt.Sprintf("panel_unban_%d", uid)),
				))
			} else {
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🚫 حظر", fmt.Sprintf("panel_ban_%d", uid)),
				))
			}
			if muted {
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🔊 إلغاء الكتم", fmt.Sprintf("panel_unmute_%d", uid)),
				))
			} else {
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🔕 كتم", fmt.Sprintf("panel_mute_%d", uid)),
				))
			}
		}
		if canManageAdmins {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏅 تغيير الرتبة", fmt.Sprintf("panel_rank_%d", uid)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", fmt.Sprintf("panel_users_%d", page)),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminRankPickerKeyboard returns the rank selection keyboard.
func AdminRankPickerKeyboard(uid int64, actorLevel, page int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, rank := range []string{"moderator", "admin", "super_admin"} {
		if services.RankLevels[rank] < actorLevel {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					services.RankLabels[rank],
					fmt.Sprintf("panel_setrank_%d_%s", uid, rank),
				),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ إزالة من الإدارة", fmt.Sprintf("panel_setrank_%d_none", uid)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", fmt.Sprintf("panel_user_%d", uid)),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminLogsKeyboard returns the paginated logs keyboard.
func AdminLogsKeyboard(page, totalPages int) tgbotapi.InlineKeyboardMarkup {
	var navRow []tgbotapi.InlineKeyboardButton
	if page > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️", fmt.Sprintf("panel_logs_%d", page-1)))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📄 %d/%d", page, totalPages), "noop"))
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️", fmt.Sprintf("panel_logs_%d", page+1)))
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	rows = append(rows, navRow)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", "adm_back"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AdminWelcomeKeyboard returns the welcome settings keyboard.
func AdminWelcomeKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 تغيير النص", "panel_welcome_text"),
			tgbotapi.NewInlineKeyboardButtonData("🖼️ تغيير الصورة", "panel_welcome_photo"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👁️ معاينة", "panel_welcome_preview"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", "adm_back"),
		),
	)
}

// AdminAdminsKeyboard returns the admin management keyboard.
func AdminAdminsKeyboard(admins []models.Admin, actorLevel int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
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
		lvl := services.RankLevels[rank]
		label := fmt.Sprintf("%s %s", services.RankLabels[rank], name)
		if lvl >= actorLevel {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, "noop"),
			))
		} else {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("panel_rank_%d", a.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ إضافة أدمن", "panel_add_admin"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 العودة", "adm_back"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}
