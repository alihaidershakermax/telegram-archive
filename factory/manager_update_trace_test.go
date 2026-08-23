package factory

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestManagedUpdateKind(t *testing.T) {
	command := tgbotapi.Update{Message: &tgbotapi.Message{Text: "/start", Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}}}}
	if got := managedUpdateKind(command); got != "command:start" {
		t.Fatalf("command update kind = %q", got)
	}

	callback := tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{ID: "callback"}}
	if got := managedUpdateKind(callback); got != "callback" {
		t.Fatalf("callback update kind = %q", got)
	}

	file := tgbotapi.Update{Message: &tgbotapi.Message{Document: &tgbotapi.Document{FileID: "file"}}}
	if got := managedUpdateKind(file); got != "file" {
		t.Fatalf("file update kind = %q", got)
	}
}
