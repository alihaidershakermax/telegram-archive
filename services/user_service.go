package services

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

// SaveUser upserts a user record based on Telegram user info.
func SaveUser(ctx context.Context, userID int64, username, firstName string) error {
	opts := options.Update().SetUpsert(true)
	_, err := db.ColScoped(ctx, "users").UpdateOne(ctx,
		bson.M{"user_id": userID},
		bson.M{
			"$set": bson.M{
				"username":     username,
				"first_name":   firstName,
				"last_seen_at": time.Now().UTC(),
			},
			"$setOnInsert": bson.M{
				"user_id":    userID,
				"is_banned":  false,
				"is_muted":   false,
				"created_at": time.Now().UTC(),
			},
		},
		opts,
	)
	return err
}

// GetUser returns a user by their Telegram user ID.
func GetUser(ctx context.Context, userID int64) (*models.User, error) {
	var user models.User
	err := db.ColScoped(ctx, "users").FindOne(ctx, bson.M{"user_id": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IsBanned checks if a user is banned.
func IsBanned(ctx context.Context, userID int64) bool {
	var user struct {
		IsBanned bool `bson:"is_banned"`
	}
	err := db.ColScoped(ctx, "users").FindOne(ctx, bson.M{"user_id": userID}).Decode(&user)
	if err != nil {
		return false
	}
	return user.IsBanned
}

// Ban bans a user.
func Ban(ctx context.Context, userID int64) error {
	_, err := db.ColScoped(ctx, "users").UpdateOne(ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": bson.M{"is_banned": true}},
	)
	return err
}

// Unban unbans a user.
func Unban(ctx context.Context, userID int64) error {
	_, err := db.ColScoped(ctx, "users").UpdateOne(ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": bson.M{"is_banned": false}},
	)
	return err
}

// IsMuted checks if a user is muted.
func IsMuted(ctx context.Context, userID int64) bool {
	var user struct {
		IsMuted bool `bson:"is_muted"`
	}
	err := db.ColScoped(ctx, "users").FindOne(ctx, bson.M{"user_id": userID}).Decode(&user)
	if err != nil {
		return false
	}
	return user.IsMuted
}

// Mute mutes a user.
func Mute(ctx context.Context, userID int64) error {
	_, err := db.ColScoped(ctx, "users").UpdateOne(ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": bson.M{"is_muted": true}},
	)
	return err
}

// Unmute unmutes a user.
func Unmute(ctx context.Context, userID int64) error {
	_, err := db.ColScoped(ctx, "users").UpdateOne(ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": bson.M{"is_muted": false}},
	)
	return err
}

// GetUsersPage returns a paginated list of users.
func GetUsersPage(ctx context.Context, page, perPage int) ([]models.User, error) {
	skip := int64((page - 1) * perPage)
	opts := options.Find().
		SetSort(bson.D{{Key: "user_id", Value: 1}}).
		SetSkip(skip).
		SetLimit(int64(perPage))
	cursor, err := db.ColScoped(ctx, "users").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var rows []models.User
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// GetUsersCount returns total user count.
func GetUsersCount(ctx context.Context) (int64, error) {
	return db.ColScoped(ctx, "users").CountDocuments(ctx, bson.M{})
}

// GetAllUserIDs returns all user IDs.
func GetAllUserIDs(ctx context.Context) ([]int64, error) {
	cursor, err := db.ColScoped(ctx, "users").Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"user_id": 1}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		UserID int64 `bson:"user_id"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.UserID
	}
	return ids, nil
}
