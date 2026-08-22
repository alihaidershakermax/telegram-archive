package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

var backupCollections = []string{
	"users", "categories", "subjects", "files", "file_ratings", "subject_subscriptions",
	"shared_files", "bot_settings", "admins", "admin_logs", "counters",
}

// CreateBotBackup snapshots one bot database; factory metadata remains outside the snapshot.
func CreateBotBackup(ctx context.Context, botID int64) (*models.BotBackup, error) {
	if botID <= 0 {
		return nil, errors.New("bot_id is required")
	}
	now := time.Now().UTC()
	backup := models.BotBackup{ID: primitive.NewObjectID().Hex(), BotID: botID, Status: models.BackupPending, Collections: map[string]int{}, CreatedAt: now}
	if _, err := db.Col("bot_backups").InsertOne(ctx, backup); err != nil {
		return nil, err
	}
	backupCtx := db.WithBotDatabase(ctx, botID)
	for _, collection := range backupCollections {
		cursor, err := db.ColScoped(backupCtx, collection).Find(backupCtx, bson.M{})
		if err != nil {
			return markBackupFailed(ctx, backup.ID, fmt.Errorf("%s: %w", collection, err))
		}
		count := 0
		for cursor.Next(backupCtx) {
			var document bson.M
			if err := cursor.Decode(&document); err != nil {
				cursor.Close(backupCtx)
				return markBackupFailed(ctx, backup.ID, fmt.Errorf("%s decode: %w", collection, err))
			}
			_, err := db.Col("bot_backup_data").InsertOne(ctx, bson.M{
				"backup_id":  backup.ID,
				"collection": collection,
				"document":   document,
			})
			if err != nil {
				cursor.Close(backupCtx)
				return markBackupFailed(ctx, backup.ID, fmt.Errorf("%s write: %w", collection, err))
			}
			count++
		}
		if err := cursor.Err(); err != nil {
			cursor.Close(backupCtx)
			return markBackupFailed(ctx, backup.ID, fmt.Errorf("%s cursor: %w", collection, err))
		}
		cursor.Close(backupCtx)
		backup.Collections[collection] = count
	}
	completed := time.Now().UTC()
	_, err := db.Col("bot_backups").UpdateOne(ctx, bson.M{"_id": backup.ID}, bson.M{"$set": bson.M{
		"status": models.BackupCompleted, "collections": backup.Collections, "completed_at": completed,
	}})
	if err != nil {
		return nil, err
	}
	backup.Status = models.BackupCompleted
	backup.CompletedAt = &completed
	return &backup, nil
}

func ListBotBackups(ctx context.Context, botID int64) ([]models.BotBackup, error) {
	cursor, err := db.Col("bot_backups").Find(ctx, bson.M{"bot_id": botID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(30))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var rows []models.BotBackup
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []models.BotBackup{}
	}
	return rows, nil
}

// RestoreBotBackup replaces only the selected bot's isolated collections.
func RestoreBotBackup(ctx context.Context, botID int64, backupID string) error {
	if botID <= 0 || backupID == "" {
		return errors.New("bot_id and backup_id are required")
	}
	var backup models.BotBackup
	if err := db.Col("bot_backups").FindOne(ctx, bson.M{"_id": backupID, "bot_id": botID, "status": models.BackupCompleted}).Decode(&backup); err != nil {
		return err
	}
	backupCtx := db.WithBotDatabase(ctx, botID)
	for _, collection := range backupCollections {
		if _, err := db.ColScoped(backupCtx, collection).DeleteMany(backupCtx, bson.M{}); err != nil {
			return fmt.Errorf("clear %s: %w", collection, err)
		}
		cursor, err := db.Col("bot_backup_data").Find(ctx, bson.M{"backup_id": backupID, "collection": collection})
		if err != nil {
			return fmt.Errorf("read %s: %w", collection, err)
		}
		for cursor.Next(ctx) {
			var row struct {
				Document bson.M `bson:"document"`
			}
			if err := cursor.Decode(&row); err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("decode %s: %w", collection, err)
			}
			if _, err := db.ColScoped(backupCtx, collection).InsertOne(backupCtx, row.Document); err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("restore %s: %w", collection, err)
			}
		}
		if err := cursor.Err(); err != nil {
			cursor.Close(ctx)
			return err
		}
		cursor.Close(ctx)
	}
	return nil
}

func markBackupFailed(ctx context.Context, id string, err error) (*models.BotBackup, error) {
	message := err.Error()
	_, updateErr := db.Col("bot_backups").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": models.BackupFailed, "error": message}})
	if updateErr != nil {
		return nil, updateErr
	}
	return nil, err
}
