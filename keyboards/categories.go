package keyboards

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/models"
)

// CategoriesKeyboard returns the inline keyboard for browsing categories.
func CategoriesKeyboard(categories []models.Category) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range categories {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(c.Name, fmt.Sprintf("cat_%d", c.CatID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 الرجوع", "back_main"),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}
