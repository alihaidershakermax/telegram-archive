package services

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"

	"telegram-archive-bot/db"
)

var ErrQuotaExceeded = errors.New("bot quota exceeded")

// Usage is calculated from the bot's isolated database.
type Usage struct {
	Users        int64 `json:"users"`
	Files        int64 `json:"files"`
	StorageBytes int64 `json:"storage_bytes"`
}

func GetUsage(ctx context.Context) (Usage, error) {
	users, err := db.ColScoped(ctx, "users").CountDocuments(ctx, bson.M{})
	if err != nil {
		return Usage{}, err
	}
	files, err := db.ColScoped(ctx, "files").CountDocuments(ctx, bson.M{})
	if err != nil {
		return Usage{}, err
	}
	cursor, err := db.ColScoped(ctx, "files").Aggregate(ctx, []bson.M{
		{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$file_size"}}},
	})
	if err != nil {
		return Usage{}, err
	}
	defer cursor.Close(ctx)
	var totals []struct {
		Total int64 `bson:"total"`
	}
	if err := cursor.All(ctx, &totals); err != nil {
		return Usage{}, err
	}
	var bytes int64
	if len(totals) > 0 {
		bytes = totals[0].Total
	}
	return Usage{Users: users, Files: files, StorageBytes: bytes}, nil
}

func CheckUserCapacity(ctx context.Context, maxUsers int64) error {
	if maxUsers <= 0 {
		return nil
	}
	count, err := db.ColScoped(ctx, "users").CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count >= maxUsers {
		return fmt.Errorf("%w: maximum users reached (%d)", ErrQuotaExceeded, maxUsers)
	}
	return nil
}

func CheckFileCapacity(ctx context.Context, maxFiles, maxBytes, newBytes int64) error {
	if maxFiles <= 0 && maxBytes <= 0 {
		return nil
	}
	usage, err := GetUsage(ctx)
	if err != nil {
		return err
	}
	if maxFiles > 0 && usage.Files >= maxFiles {
		return fmt.Errorf("%w: maximum files reached (%d)", ErrQuotaExceeded, maxFiles)
	}
	if maxBytes > 0 && newBytes > 0 && usage.StorageBytes > maxBytes-newBytes {
		return fmt.Errorf("%w: storage limit reached (%d bytes)", ErrQuotaExceeded, maxBytes)
	}
	return nil
}
