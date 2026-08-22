package services

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"telegram-archive-bot/db"
)

// RecordAIUsage keeps operational AI usage inside the current bot namespace.
func RecordAIUsage(ctx context.Context, operation string, inputChars int, success bool) error {
	if inputChars < 0 {
		inputChars = 0
	}
	_, err := db.ColScoped(ctx, "ai_usage").InsertOne(ctx, bson.M{
		"operation":   operation,
		"input_chars": inputChars,
		"success":     success,
		"created_at":  time.Now().UTC(),
	})
	return err
}

func GetAIUsageCount(ctx context.Context, since time.Time) (int64, error) {
	return db.ColScoped(ctx, "ai_usage").CountDocuments(ctx, bson.M{"created_at": bson.M{"$gte": since}})
}
