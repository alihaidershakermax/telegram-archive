package services

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
)

func TestSetGroupEnabledRejectsInvalidScope(t *testing.T) {
	if err := SetGroupEnabled(context.Background(), 0, -100, true); err != mongo.ErrNoDocuments {
		t.Fatalf("SetGroupEnabled invalid scope error = %v, want mongo.ErrNoDocuments", err)
	}
}

func TestGetOrCreateGroupRejectsInvalidScope(t *testing.T) {
	if _, err := GetOrCreateGroup(context.Background(), 0, -100, "group"); err != mongo.ErrNoDocuments {
		t.Fatalf("GetOrCreateGroup invalid scope error = %v, want mongo.ErrNoDocuments", err)
	}
}
