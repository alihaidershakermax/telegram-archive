package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

var allowedAPIKeyPermissions = map[string]bool{
	"archive:read":   true,
	"archive:write":  true,
	"archive:delete": true,
	"bot:analytics":  true,
	"bot:settings":   true,
}

// CreateAPIKey returns the metadata and the raw key exactly once.
func CreateAPIKey(ctx context.Context, botID int64, name string, permissions []string) (models.APIKey, string, error) {
	if botID <= 0 {
		return models.APIKey{}, "", errors.New("bot_id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return models.APIKey{}, "", errors.New("key name must be 1-80 characters")
	}
	cleanPermissions := make([]string, 0, len(permissions))
	seen := make(map[string]bool)
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" || seen[permission] {
			continue
		}
		if !allowedAPIKeyPermissions[permission] {
			return models.APIKey{}, "", fmt.Errorf("unsupported API key permission: %s", permission)
		}
		seen[permission] = true
		cleanPermissions = append(cleanPermissions, permission)
	}
	if len(cleanPermissions) == 0 {
		return models.APIKey{}, "", errors.New("at least one permission is required")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return models.APIKey{}, "", err
	}
	raw := "taf_" + hex.EncodeToString(buf)
	hash := hashAPIKey(raw)
	now := time.Now().UTC()
	record := models.APIKey{
		ID:          hex.EncodeToString(buf[:12]),
		BotID:       botID,
		Name:        name,
		KeyHash:     hash,
		Prefix:      raw[:12],
		Permissions: cleanPermissions,
		CreatedAt:   now,
	}
	if _, err := db.Col("api_keys").InsertOne(ctx, record); err != nil {
		return models.APIKey{}, "", err
	}
	return record, raw, nil
}

func ListAPIKeys(ctx context.Context, botID int64) ([]models.APIKey, error) {
	cursor, err := db.Col("api_keys").Find(ctx, bson.M{"bot_id": botID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var rows []models.APIKey
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []models.APIKey{}
	}
	return rows, nil
}

func RevokeAPIKey(ctx context.Context, botID int64, id string) error {
	now := time.Now().UTC()
	result, err := db.Col("api_keys").UpdateOne(ctx, bson.M{"_id": id, "bot_id": botID, "revoked_at": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"revoked_at": now}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func VerifyAPIKey(ctx context.Context, raw string) (*models.APIKey, error) {
	if len(raw) < 20 {
		return nil, mongo.ErrNoDocuments
	}
	var record models.APIKey
	err := db.Col("api_keys").FindOne(ctx, bson.M{"key_hash": hashAPIKey(raw), "revoked_at": bson.M{"$exists": false}}).Decode(&record)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func HasAPIKeyPermission(record *models.APIKey, permission string) bool {
	if record == nil {
		return false
	}
	for _, value := range record.Permissions {
		if value == permission {
			return true
		}
	}
	return false
}

func hashAPIKey(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
