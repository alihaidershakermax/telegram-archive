package handlers

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/db"
)

func TestArchiveContextScopesManagedBotOnly(t *testing.T) {
	primary := &tgbotapi.BotAPI{Self: tgbotapi.User{ID: 100}}
	managed := &tgbotapi.BotAPI{Self: tgbotapi.User{ID: 200}}
	SetStorageBot(primary)
	t.Cleanup(func() { SetStorageBot(nil) })

	if db.IsScoped(archiveContext(primary)) {
		t.Fatal("primary bot must use the configured database")
	}
	ctx := archiveContext(managed)
	if !db.IsScoped(ctx) {
		t.Fatal("managed bot must use a scoped database")
	}
	if got := db.ScopeKey(ctx); got != "archive_bot_200" {
		t.Fatalf("unexpected managed database scope: %q", got)
	}
}

func TestStateIsolatedByBot(t *testing.T) {
	primary := &tgbotapi.BotAPI{Self: tgbotapi.User{ID: 300}}
	managed := &tgbotapi.BotAPI{Self: tgbotapi.User{ID: 400}}
	primaryState := GetStateForBot(primary, 77)
	managedState := GetStateForBot(managed, 77)
	if primaryState == managedState {
		t.Fatal("the same user must have separate state per bot")
	}
}
