package services

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

func SubscribeToSubject(ctx context.Context, botID, userID int64, subjectID int) error {
	if botID <= 0 || userID <= 0 || subjectID <= 0 {
		return fmt.Errorf("invalid subscription scope")
	}
	now := time.Now().UTC()
	_, err := db.ColScoped(ctx, "subject_subscriptions").UpdateOne(ctx,
		bson.M{"bot_id": botID, "user_id": userID, "subject_id": subjectID},
		bson.M{"$setOnInsert": models.SubjectSubscription{ID: primitive.NewObjectID().Hex(), BotID: botID, UserID: userID, SubjectID: subjectID, CreatedAt: now}},
		options.Update().SetUpsert(true))
	return err
}

func UnsubscribeFromSubject(ctx context.Context, botID, userID int64, subjectID int) error {
	if botID <= 0 || userID <= 0 || subjectID <= 0 {
		return fmt.Errorf("invalid subscription scope")
	}
	_, err := db.ColScoped(ctx, "subject_subscriptions").DeleteOne(ctx, bson.M{"bot_id": botID, "user_id": userID, "subject_id": subjectID})
	return err
}

func ListSubjectSubscriptions(ctx context.Context, botID, userID int64) ([]models.SubjectSubscription, error) {
	cur, err := db.ColScoped(ctx, "subject_subscriptions").Find(ctx, bson.M{"bot_id": botID, "user_id": userID}, options.Find().SetLimit(100).SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []models.SubjectSubscription
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func ListVault(ctx context.Context, botID, userID int64) ([]models.VaultItem, error) {
	cur, err := db.ColScoped(ctx, "vault_items").Find(ctx, bson.M{"bot_id": botID, "user_id": userID}, options.Find().SetLimit(100).SetSort(bson.D{{Key: "added_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []models.VaultItem
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func AddToVault(ctx context.Context, botID, userID int64, fileID int) error {
	if botID <= 0 || userID <= 0 || fileID <= 0 {
		return fmt.Errorf("invalid vault scope")
	}
	_, err := db.ColScoped(ctx, "vault_items").UpdateOne(ctx,
		bson.M{"bot_id": botID, "user_id": userID, "file_id": fileID},
		bson.M{"$setOnInsert": models.VaultItem{ID: primitive.NewObjectID().Hex(), BotID: botID, UserID: userID, FileID: fileID, AddedAt: time.Now().UTC()}}, options.Update().SetUpsert(true))
	return err
}

func NotifySubjectSubscribers(ctx context.Context, bot *tgbotapi.BotAPI, botID int64, file models.File) error {
	if bot == nil || file.FileID <= 0 || file.SubjectID <= 0 {
		return fmt.Errorf("invalid notification input")
	}
	cur, err := db.ColScoped(ctx, "subject_subscriptions").Find(ctx, bson.M{"bot_id": botID, "subject_id": file.SubjectID}, options.Find().SetLimit(1000).SetProjection(bson.M{"user_id": 1}))
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	var subs []models.SubjectSubscription
	if err := cur.All(ctx, &subs); err != nil {
		return err
	}
	for _, sub := range subs {
		// Claim the notification atomically before sending. A Find-then-Send-
		// Insert sequence can deliver duplicates when two upload workers race.
		notification := models.SubjectNotification{
			ID:        primitive.NewObjectID().Hex(),
			BotID:     botID,
			UserID:    sub.UserID,
			SubjectID: file.SubjectID,
			FileID:    file.FileID,
			SentAt:    time.Now().UTC(),
		}
		claim, claimErr := db.ColScoped(ctx, "subject_notifications").UpdateOne(ctx,
			bson.M{"bot_id": botID, "user_id": sub.UserID, "file_id": file.FileID},
			bson.M{"$setOnInsert": notification},
			options.Update().SetUpsert(true),
		)
		if claimErr != nil {
			return claimErr
		}
		if claim.UpsertedCount == 0 {
			continue
		}
		if _, sendErr := bot.Send(tgbotapi.NewMessage(sub.UserID, fmt.Sprintf("📚 ملف جديد في المادة\\n\\n%s\\n\\nاستخدم /start لفتح الأرشيف.", file.Name))); sendErr != nil {
			// Let a later upload-queue retry claim it again if delivery failed.
			_, _ = db.ColScoped(ctx, "subject_notifications").DeleteOne(ctx, bson.M{"_id": notification.ID})
		}
	}
	return nil
}
