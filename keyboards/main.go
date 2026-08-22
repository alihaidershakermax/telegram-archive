package keyboards

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MainMenuKeyboard returns the public menu without management controls.
func MainMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return MainMenuKeyboardForRole(false, false)
}

// MainMenuKeyboardForRole keeps management controls out of the public menu.
// showFactory is reserved for the owner of the parent bot, never a child bot.
func MainMenuKeyboardForRole(showAdmin, showFactory bool) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📂 الملفات", "view_archive")),
	}
	if showAdmin {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⚙️ لوحة التحكم", "panel")))
	}
	last := []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("👨‍💻 عن المطور", "about")}
	if showFactory {
		last = append(last, tgbotapi.NewInlineKeyboardButtonData("🤖 إدارة البوتات", "factory_info"))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(last...))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}
