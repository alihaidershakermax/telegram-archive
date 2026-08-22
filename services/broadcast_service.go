package services

import (
	"context"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson"

	"telegram-archive-bot/db"
)

// BroadcastResult holds the outcome of a broadcast operation.
type BroadcastResult struct {
	Success int
	Failed  int
}

// SendBroadcast sends a text message to all non-banned users using a streaming cursor.
func SendBroadcast(ctx context.Context, bot *tgbotapi.BotAPI, message string, delay time.Duration) BroadcastResult {
	cursor, err := db.Col("users").Find(ctx, bson.M{"is_banned": bson.M{"$ne": true}})
	if err != nil {
		log.Printf("Broadcast: failed to fetch users: %v", err)
		return BroadcastResult{}
	}
	defer cursor.Close(ctx)

	result := BroadcastResult{}
	for cursor.Next(ctx) {
		var user struct {
			UserID int64 `bson:"user_id"`
		}
		if err := cursor.Decode(&user); err != nil {
			result.Failed++
			log.Printf("Broadcast decode failed: %v", err)
			continue
		}
		_, err := bot.Send(tgbotapi.NewMessage(user.UserID, message))
		if err != nil {
			result.Failed++
			log.Printf("Broadcast failed for %d: %v", user.UserID, err)
		} else {
			result.Success++
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	if err := cursor.Err(); err != nil {
		log.Printf("Broadcast cursor failed: %v", err)
	}
	log.Printf("Broadcast done: %d success, %d failed", result.Success, result.Failed)
	return result
}
