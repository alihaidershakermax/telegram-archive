package services

import (
	"testing"

	"telegram-archive-bot/models"
)

func TestFactoryRanksExposeExpectedRoles(t *testing.T) {
	for _, rank := range []string{"owner", "admin", "editor", "viewer"} {
		if _, ok := RankLevels[rank]; !ok {
			t.Fatalf("missing rank %q", rank)
		}
		if RankLabel(rank) == "👤 مستخدم عادي" {
			t.Fatalf("missing label for rank %q", rank)
		}
	}
}

func TestAPIKeyPermissionsAreExact(t *testing.T) {
	key := &models.APIKey{Permissions: []string{"archive:read", "bot:analytics"}}
	if !HasAPIKeyPermission(key, "archive:read") || !HasAPIKeyPermission(key, "bot:analytics") {
		t.Fatal("expected declared permissions")
	}
	if HasAPIKeyPermission(key, "archive:delete") {
		t.Fatal("undeclared permission must be denied")
	}
}
