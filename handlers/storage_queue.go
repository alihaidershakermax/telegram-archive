package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

const (
	defaultStoragePollSeconds = 5
	defaultStorageMaxAttempts = 5
	defaultStorageRetryBase   = 5
	defaultStorageBatchSize   = 10
	maxStorageErrorLength     = 500
)

// StartStorageQueue starts one process-local consumer for durable primary-bot
// delivery jobs. MongoDB claiming remains atomic, so a second process cannot
// process the same job at the same time.
func StartStorageQueue(ctx context.Context, cfg *config.Config) {
	pollSeconds, maxAttempts, retryBase, batchSize := storageQueueConfig(cfg)
	go func() {
		ticker := time.NewTicker(time.Duration(pollSeconds) * time.Second)
		defer ticker.Stop()
		for {
			if err := drainStorageQueue(ctx, maxAttempts, retryBase, batchSize); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("storage queue drain warning: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func storageQueueConfig(cfg *config.Config) (pollSeconds, maxAttempts, retryBase, batchSize int) {
	pollSeconds, maxAttempts, retryBase, batchSize = defaultStoragePollSeconds, defaultStorageMaxAttempts, defaultStorageRetryBase, defaultStorageBatchSize
	if cfg == nil {
		return
	}
	if cfg.StorageQueuePollSeconds > 0 {
		pollSeconds = cfg.StorageQueuePollSeconds
	}
	if cfg.StorageMaxAttempts > 0 {
		maxAttempts = cfg.StorageMaxAttempts
	}
	if cfg.StorageRetryBaseSeconds > 0 {
		retryBase = cfg.StorageRetryBaseSeconds
	}
	if cfg.StorageQueueBatchSize > 0 {
		batchSize = cfg.StorageQueueBatchSize
	}
	return
}

// QueueStorageDelivery persists a primary-bot file delivery request. The
// canonical file ID is internal data and is deliberately omitted from JSON.
func QueueStorageDelivery(ctx context.Context, botID, chatID int64, fileID, fileType, caption string) (string, error) {
	if botID <= 0 || chatID == 0 || strings.TrimSpace(fileID) == "" {
		return "", fmt.Errorf("storage delivery requires bot_id, chat_id, and file_id")
	}
	now := time.Now().UTC()
	job := models.StorageJob{
		ID:             primitive.NewObjectID().Hex(),
		BotID:          botID,
		ChatID:         chatID,
		TelegramFileID: fileID,
		FileType:       fileType,
		Caption:        caption,
		Status:         models.StorageJobPending,
		NextAttemptAt:  now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, err := db.Col("storage_jobs").InsertOne(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

// SendStorageFile attempts immediate delivery through the primary Storage
// Gateway and queues the same request when Telegram returns a transient error.
func SendStorageFile(ctx context.Context, originatingBot *tgbotapi.BotAPI, chatID int64, fileID, fileType, caption string) (queued bool, err error) {
	sender := storageBot(originatingBot)
	if sender == nil {
		return false, fmt.Errorf("primary storage bot is not configured")
	}
	if _, err = sender.Send(storageMedia(chatID, fileID, fileType, caption)); err == nil {
		return false, nil
	}
	botID := int64(0)
	if originatingBot != nil {
		botID = originatingBot.Self.ID
	}
	if botID <= 0 {
		botID = sender.Self.ID
	}
	if botID <= 0 {
		return false, err
	}
	if _, queueErr := QueueStorageDelivery(ctx, botID, chatID, fileID, fileType, caption); queueErr != nil {
		return false, fmt.Errorf("send failed: %v; queue failed: %w", err, queueErr)
	}
	return true, err
}

func storageMedia(chatID int64, fileID, fileType, caption string) tgbotapi.Chattable {
	file := tgbotapi.FileID(fileID)
	switch fileType {
	case "photo":
		media := tgbotapi.NewPhoto(chatID, file)
		media.Caption = caption
		return media
	case "video":
		media := tgbotapi.NewVideo(chatID, file)
		media.Caption = caption
		return media
	case "audio":
		media := tgbotapi.NewAudio(chatID, file)
		media.Caption = caption
		return media
	case "voice":
		media := tgbotapi.NewVoice(chatID, file)
		media.Caption = caption
		return media
	case "animation":
		media := tgbotapi.NewAnimation(chatID, file)
		media.Caption = caption
		return media
	default:
		media := tgbotapi.NewDocument(chatID, file)
		media.Caption = caption
		return media
	}
}

func drainStorageQueue(ctx context.Context, maxAttempts, retryBase, batchSize int) error {
	for i := 0; i < batchSize; i++ {
		job, err := claimStorageJob(ctx)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := deliverStorageJob(ctx, job); err != nil {
			if updateErr := failStorageJob(ctx, job, err, maxAttempts, retryBase); updateErr != nil {
				log.Printf("storage job %s failure state update: %v", job.ID, updateErr)
			}
			continue
		}
		if _, err := db.Col("storage_jobs").UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{"$set": bson.M{
			"status":       models.StorageJobSent,
			"updated_at":   time.Now().UTC(),
			"completed_at": time.Now().UTC(),
			"last_error":   "",
		}}); err != nil {
			return err
		}
	}
	return nil
}

func claimStorageJob(ctx context.Context) (models.StorageJob, error) {
	var job models.StorageJob
	now := time.Now().UTC()
	filter := bson.M{
		"$or": []bson.M{
			{"status": bson.M{"$in": []string{models.StorageJobPending, models.StorageJobRetrying}}, "next_attempt_at": bson.M{"$lte": now}},
			{"status": models.StorageJobProcessing, "updated_at": bson.M{"$lte": now.Add(-2 * time.Minute)}},
		},
	}
	update := bson.M{"$set": bson.M{
		"status":     models.StorageJobProcessing,
		"updated_at": now,
	}}
	err := db.Col("storage_jobs").FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetSort(bson.D{{Key: "next_attempt_at", Value: 1}, {Key: "created_at", Value: 1}}).SetReturnDocument(options.After)).Decode(&job)
	return job, err
}

func deliverStorageJob(ctx context.Context, job models.StorageJob) error {
	sender := storageBot(nil)
	if sender == nil {
		return fmt.Errorf("primary storage bot is not configured")
	}
	_, err := sender.Send(storageMedia(job.ChatID, job.TelegramFileID, job.FileType, job.Caption))
	return err
}

func failStorageJob(ctx context.Context, job models.StorageJob, deliveryErr error, maxAttempts, retryBase int) error {
	attempts := job.Attempts + 1
	now := time.Now().UTC()
	status := models.StorageJobRetrying
	set := bson.M{
		"attempts":   attempts,
		"updated_at": now,
		"last_error": truncateStorageError(deliveryErr),
	}
	if attempts >= maxAttempts {
		status = models.StorageJobDead
		set["status"] = status
	} else {
		backoff := time.Duration(retryBase) * time.Second * time.Duration(1<<minStorageInt(attempts-1, 10))
		set["status"] = status
		set["next_attempt_at"] = now.Add(backoff)
	}
	_, err := db.Col("storage_jobs").UpdateOne(ctx, bson.M{"_id": job.ID, "status": models.StorageJobProcessing}, bson.M{"$set": set})
	if status == models.StorageJobDead {
		log.Printf("storage job %s moved to dead-letter after %d attempts", job.ID, attempts)
	}
	return err
}

func truncateStorageError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > maxStorageErrorLength {
		return message[:maxStorageErrorLength]
	}
	return message
}

func minStorageInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
