package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

func UpsertAIIndex(ctx context.Context, fileID int, sourceText, summary string, tags []string) (*models.AIIndex, error) {
	if fileID <= 0 || strings.TrimSpace(sourceText) == "" {
		return nil, errors.New("file_id and source text are required")
	}
	digest := sha256.Sum256([]byte(sourceText))
	now := time.Now().UTC()
	record := models.AIIndex{FileID: fileID, Summary: strings.TrimSpace(summary), Tags: tags, ContentHash: hex.EncodeToString(digest[:]), UpdatedAt: now}
	_, err := db.ColScoped(ctx, "ai_indexes").UpdateOne(ctx, bson.M{"file_id": fileID}, bson.M{"$set": record}, options.Update().SetUpsert(true))
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func GetAIIndex(ctx context.Context, fileID int) (*models.AIIndex, error) {
	var record models.AIIndex
	if err := db.ColScoped(ctx, "ai_indexes").FindOne(ctx, bson.M{"file_id": fileID}).Decode(&record); err != nil {
		return nil, err
	}
	return &record, nil
}
