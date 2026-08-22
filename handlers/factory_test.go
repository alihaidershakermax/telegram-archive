package handlers

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/config"
)

func TestHandoffIDValidation(t *testing.T) {
	if got, err := parsePositiveInt64("123456789"); err != nil || got != 123456789 {
		t.Fatalf("expected valid Telegram user id, got %d/%v", got, err)
	}
	for _, value := range []string{"0", "-1", "abc"} {
		if _, err := parsePositiveInt64(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestParentBotOwnerOnlyFactoryAccess(t *testing.T) {
	oldCfg := config.Cfg
	config.Cfg = &config.Config{OwnerID: 42}
	t.Cleanup(func() { config.Cfg = oldCfg })

	parent := &tgbotapi.BotAPI{Self: tgbotapi.User{ID: 100}}
	child := &tgbotapi.BotAPI{Self: tgbotapi.User{ID: 200}}
	SetStorageBot(parent)
	t.Cleanup(func() { SetStorageBot(nil) })

	if !isParentBotOwner(parent, 42) {
		t.Fatal("parent owner should manage Bot Factory")
	}
	if isParentBotOwner(child, 42) {
		t.Fatal("parent owner must not see Bot Factory controls on a child bot")
	}
	if isParentBotOwner(parent, 99) {
		t.Fatal("non-owner must not manage Bot Factory")
	}
}
