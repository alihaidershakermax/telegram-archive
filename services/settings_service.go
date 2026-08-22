package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

// GetWelcomeSettings returns the current welcome message and photo.
func GetWelcomeSettings(ctx context.Context) models.WelcomeSettings {
	settings := models.WelcomeSettings{
		Message: config.WelcomeMessage,
		Photo:   config.Cfg.WelcomePhoto,
	}

	var textRow models.BotSetting
	err := db.Col("bot_settings").FindOne(ctx, bson.M{"key": "welcome_message"}).Decode(&textRow)
	if err == nil && textRow.Value != "" {
		settings.Message = textRow.Value
	}

	var photoRow models.BotSetting
	err = db.Col("bot_settings").FindOne(ctx, bson.M{"key": "welcome_photo"}).Decode(&photoRow)
	if err == nil && photoRow.Value != "" {
		settings.Photo = photoRow.Value
	}

	return settings
}

// SetWelcomeMessage updates the welcome message text.
func SetWelcomeMessage(ctx context.Context, text string) error {
	opts := options.Update().SetUpsert(true)
	_, err := db.Col("bot_settings").UpdateOne(ctx,
		bson.M{"key": "welcome_message"},
		bson.M{"$set": bson.M{"value": text}},
		opts,
	)
	return err
}

// SetWelcomePhoto updates the welcome photo file ID.
func SetWelcomePhoto(ctx context.Context, fileID string) error {
	opts := options.Update().SetUpsert(true)
	_, err := db.Col("bot_settings").UpdateOne(ctx,
		bson.M{"key": "welcome_photo"},
		bson.M{"$set": bson.M{"value": fileID}},
		opts,
	)
	return err
}
