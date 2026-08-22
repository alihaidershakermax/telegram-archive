package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"telegram-archive-bot/db"
)

// CreateShareLink creates a cryptographically random share token for a file.
func CreateShareLink(ctx context.Context, telegramFileID, fileType string, createdBy int64, expiresDays int) (string, error) {
	if expiresDays <= 0 {
		expiresDays = 7
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	hash := hex.EncodeToString(tokenBytes)

	_, err := db.Col("shared_files").InsertOne(ctx, bson.M{
		"share_hash":       hash,
		"telegram_file_id": telegramFileID,
		"file_type":        fileType,
		"expires_at":       time.Now().UTC().Add(time.Duration(expiresDays) * 24 * time.Hour),
		"created_by":       createdBy,
	})
	if err != nil {
		return "", err
	}
	return hash, nil
}

// GetShareLink retrieves the Telegram file ID and original file type.
func GetShareLink(ctx context.Context, shareHash string) (string, string, error) {
	var doc struct {
		TelegramFileID string    `bson:"telegram_file_id"`
		FileType       string    `bson:"file_type"`
		ExpiresAt      time.Time `bson:"expires_at"`
	}
	if err := db.Col("shared_files").FindOne(ctx, bson.M{"share_hash": shareHash}).Decode(&doc); err != nil {
		return "", "", err
	}
	if doc.ExpiresAt.Before(time.Now().UTC()) {
		_, _ = db.Col("shared_files").DeleteOne(ctx, bson.M{"share_hash": shareHash})
		return "", "", nil
	}
	return doc.TelegramFileID, doc.FileType, nil
}
