package keyboards

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MainMenuKeyboard returns the main menu inline keyboard.
func MainMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📂 الملفات", "view_archive"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👨‍💻 عن المطور", "about"),
			tgbotapi.NewInlineKeyboardButtonData("🤖 إدارة البوتات", "factory_info"),
		),
	)
}
