package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"telegram-archive-bot/db"
)

// StorageQueueStats contains operational counters without exposing file IDs.
type StorageQueueStats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Retrying   int64 `json:"retrying"`
	Sent       int64 `json:"sent"`
	Dead       int64 `json:"dead"`
}

func GetStorageQueueStats(ctx context.Context, botID int64) (StorageQueueStats, error) {
	filter := bson.M{}
	if botID > 0 {
		filter["bot_id"] = botID
	}
	count := func(status string) (int64, error) {
		query := bson.M{"status": status}
		for key, value := range filter {
			query[key] = value
		}
		return db.Col("storage_jobs").CountDocuments(ctx, query)
	}
	var stats StorageQueueStats
	var err error
	if stats.Pending, err = count("pending"); err != nil {
		return StorageQueueStats{}, err
	}
	if stats.Processing, err = count("processing"); err != nil {
		return StorageQueueStats{}, err
	}
	if stats.Retrying, err = count("retrying"); err != nil {
		return StorageQueueStats{}, err
	}
	if stats.Sent, err = count("sent"); err != nil {
		return StorageQueueStats{}, err
	}
	if stats.Dead, err = count("dead"); err != nil {
		return StorageQueueStats{}, err
	}
	return stats, nil
}
