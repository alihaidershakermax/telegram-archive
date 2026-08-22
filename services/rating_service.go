package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
)

// RateFile sets or updates a user's rating for a file.
func RateFile(ctx context.Context, userID int64, fileID, stars int) error {
	opts := options.Update().SetUpsert(true)
	_, err := db.ColScoped(ctx, "file_ratings").UpdateOne(ctx,
		bson.M{"user_id": userID, "file_id": fileID},
		bson.M{"$set": bson.M{"user_id": userID, "file_id": fileID, "stars": stars}},
		opts,
	)
	return err
}

// FileRatingResult holds the average rating and count.
type FileRatingResult struct {
	Avg   float64
	Count int
}

// GetFileRating returns the average rating for a file.
func GetFileRating(ctx context.Context, fileID int) (FileRatingResult, error) {
	cursor, err := db.ColScoped(ctx, "file_ratings").Find(ctx, bson.M{"file_id": fileID})
	if err != nil {
		return FileRatingResult{}, err
	}
	var rows []struct {
		Stars int `bson:"stars"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return FileRatingResult{}, err
	}
	if len(rows) == 0 {
		return FileRatingResult{}, nil
	}
	sum := 0
	for _, r := range rows {
		sum += r.Stars
	}
	return FileRatingResult{
		Avg:   float64(sum) / float64(len(rows)),
		Count: len(rows),
	}, nil
}

// GetUserRating returns a specific user's rating for a file.
func GetUserRating(ctx context.Context, userID int64, fileID int) (int, error) {
	var row struct {
		Stars int `bson:"stars"`
	}
	err := db.ColScoped(ctx, "file_ratings").FindOne(ctx, bson.M{"user_id": userID, "file_id": fileID}).Decode(&row)
	if err != nil {
		return 0, err
	}
	return row.Stars, nil
}
