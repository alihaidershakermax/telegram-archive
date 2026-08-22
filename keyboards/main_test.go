package keyboards

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestMainMenuRoleVisibility(t *testing.T) {
	public := MainMenuKeyboardForRole(false, false)
	admin := MainMenuKeyboardForRole(true, false)
	owner := MainMenuKeyboardForRole(true, true)
	contains := func(menu tgbotapi.InlineKeyboardMarkup, callback string) bool {
		for _, row := range menu.InlineKeyboard {
			for _, button := range row {
				if button.CallbackData != nil && *button.CallbackData == callback {
					return true
				}
			}
		}
		return false
	}
	if contains(public, "panel") || contains(public, "factory_info") {
		t.Fatal("public menu must hide management controls")
	}
	if !contains(admin, "panel") || contains(admin, "factory_info") {
		t.Fatal("admin menu must show only the admin panel")
	}
	if !contains(owner, "panel") || !contains(owner, "factory_info") {
		t.Fatal("parent owner menu must show both controls")
	}
}
