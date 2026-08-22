package keyboards

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/models"
)

// SubjectsKeyboard returns the inline keyboard for browsing subjects.
func SubjectsKeyboard(subjects []models.Subject) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, s := range subjects {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(s.Name, fmt.Sprintf("sub_%d", s.SubID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع", "back_cats"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}
