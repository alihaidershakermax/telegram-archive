package keyboards

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/config"
	"telegram-archive-bot/models"
	"telegram-archive-bot/utils"
)

// FilesKeyboard returns the inline keyboard for browsing files in a subject.
func FilesKeyboard(files []models.File, subjectID, catID int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, f := range files {
		icon := config.FileTypeIcons[f.FileType]
		if icon == "" {
			icon = "📄"
		}
		label := fmt.Sprintf("%s %s", icon, f.Name)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("file_%d", f.FileID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع", fmt.Sprintf("back_subs_%d", catID)),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// FileActionsKeyboard returns the inline keyboard for file actions.
func FileActionsKeyboard(fileID int, backData, fileType, name string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📥 تحميل", fmt.Sprintf("download_%d", fileID)),
	))

	if utils.IsImageFile(fileType, name) || utils.IsPDFFile(fileType, name) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🖼 تحميل كصورة", fmt.Sprintf("dlimg_%d", fileID)),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📤 مشاركة", fmt.Sprintf("share_%d", fileID)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع", backData),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}
