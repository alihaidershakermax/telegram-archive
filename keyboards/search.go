package keyboards

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/config"
	"telegram-archive-bot/models"
)

func SearchResultsKeyboard(files []models.FileRow, page int, hasPrevious, hasNext bool) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(files)+2)
	for _, file := range files {
		icon := config.FileTypeIcons[file.FileType]
		if icon == "" {
			icon = "📄"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%s %s", icon, file.Name), fmt.Sprintf("file_%d", file.FileID)),
		))
	}
	if hasPrevious || hasNext {
		buttons := make([]tgbotapi.InlineKeyboardButton, 0, 2)
		if hasPrevious {
			buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("◀️ السابق", fmt.Sprintf("search_page_%d", page-1)))
		}
		if hasNext {
			buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("التالي ▶️", fmt.Sprintf("search_page_%d", page+1)))
		}
		rows = append(rows, buttons)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔎 بحث جديد", "search_start"),
		tgbotapi.NewInlineKeyboardButtonData("🏠 الرئيسية", "back_main"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}
