package factory

import (
	"testing"

	"telegram-archive-bot/config"
)

func TestCapacityTierExpandsOnDocumentThreshold(t *testing.T) {
	cfg := &config.Config{DBExpansionMaxDocs: 100, DBExpansionMaxBytes: 0}
	if got := capacityTier(99, 20, 0, cfg); got != "standard" {
		t.Fatalf("expected standard tier, got %q", got)
	}
	if got := capacityTier(100, 20, 0, cfg); got != "expanded" {
		t.Fatalf("expected expanded tier, got %q", got)
	}
}

func TestCapacityTierExpandsOnByteThreshold(t *testing.T) {
	cfg := &config.Config{DBExpansionMaxDocs: 0, DBExpansionMaxBytes: 1000}
	if got := capacityTier(1, 1, 999, cfg); got != "standard" {
		t.Fatalf("expected standard tier, got %q", got)
	}
	if got := capacityTier(1, 1, 1000, cfg); got != "expanded" {
		t.Fatalf("expected expanded tier, got %q", got)
	}
}
