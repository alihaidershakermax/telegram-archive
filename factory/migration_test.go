package factory

import (
	"context"
	"testing"
)

func TestMigrateBotNamespaceValidatesArgumentsBeforeDatabaseAccess(t *testing.T) {
	manager := &Manager{}
	if err := manager.MigrateBotNamespace(context.Background(), 0, ""); err == nil {
		t.Fatal("expected missing bot and target to be rejected")
	}
	if err := manager.MigrateBotNamespace(context.Background(), 123, ""); err == nil {
		t.Fatal("expected missing target to be rejected")
	}
}
