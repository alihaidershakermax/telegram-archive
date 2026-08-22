package factory

import (
	"testing"
	"time"

	"telegram-archive-bot/models"
)

func TestBotScorePrefersHealthyLowLoadWorker(t *testing.T) {
	good := models.ManagedBot{
		LastSeenAt:        time.Now().UTC(),
		LastLatencyMS:     25,
		ActiveRequests:    1,
		ConsecutiveErrors: 0,
		TotalErrors:       1,
	}
	busy := models.ManagedBot{
		LastSeenAt:        time.Now().Add(-2 * time.Minute).UTC(),
		LastLatencyMS:     600,
		ActiveRequests:    8,
		ConsecutiveErrors: 2,
		TotalErrors:       20,
	}
	if botScore(good) <= botScore(busy) {
		t.Fatalf("expected healthy low-load bot to score higher: good=%f busy=%f", botScore(good), botScore(busy))
	}
}

func TestBotNamespaceIsStableAndScoped(t *testing.T) {
	database, folder := botNamespace("Archive_Bot", 12345)
	if database != "archive_bot_12345" {
		t.Fatalf("unexpected database namespace: %q", database)
	}
	if folder != "bots/archive_bot_12345/" {
		t.Fatalf("unexpected storage folder: %q", folder)
	}
	otherDatabase, otherFolder := botNamespace("Archive_Bot", 67890)
	if database == otherDatabase || folder == otherFolder {
		t.Fatal("different Telegram bots must not share a namespace")
	}
}
