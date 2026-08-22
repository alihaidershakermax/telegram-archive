package services

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

func GetOrCreateGroup(ctx context.Context, botID, chatID int64, title string) (*models.GroupConfig, error) {
	if botID == 0 || chatID == 0 {
		return nil, mongo.ErrNoDocuments
	}
	filter := bson.M{"bot_id": botID, "chat_id": chatID}
	var row models.GroupConfig
	err := db.ColScoped(ctx, "group_configs").FindOne(ctx, filter).Decode(&row)
	if err == nil {
		return &row, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}
	now := time.Now().UTC()
	row = models.GroupConfig{ID: primitive.NewObjectID().Hex(), BotID: botID, ChatID: chatID, Title: strings.TrimSpace(title), Enabled: true, CreatedAt: now, UpdatedAt: now}
	_, err = db.ColScoped(ctx, "group_configs").UpdateOne(ctx, filter, bson.M{"$setOnInsert": row}, options.Update().SetUpsert(true))
	if err != nil {
		return nil, err
	}
	if err := db.ColScoped(ctx, "group_configs").FindOne(ctx, filter).Decode(&row); err != nil {
		return nil, err
	}
	return &row, nil
}

func SetGroupEnabled(ctx context.Context, botID, chatID int64, enabled bool) error {
	_, err := db.ColScoped(ctx, "group_configs").UpdateOne(ctx, bson.M{"bot_id": botID, "chat_id": chatID}, bson.M{"$set": bson.M{"enabled": enabled, "updated_at": time.Now().UTC()}}, options.Update().SetUpsert(true))
	return err
}
