package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
)

var (
	client *mongo.Client
	db     *mongo.Database
)

// Init connects to MongoDB and stores the client/db globally.
func Init() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options.Client().ApplyURI(config.Cfg.MongoURI)

	var err error
	client, err = mongo.Connect(ctx, opts)
	if err != nil {
		log.Fatalf("MongoDB connect error: %v", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}

	db = client.Database(config.Cfg.DBName)
	log.Println("MongoDB connection ready")
}

// GetDB returns the database instance.
func GetDB() *mongo.Database {
	return db
}

// Col is a shortcut to get a collection.
func Col(name string) *mongo.Collection {
	return db.Collection(name)
}

// Close disconnects from MongoDB.
func Close() {
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	}
}

// indexSpec describes one index to create.
type indexSpec struct {
	Collection string
	Keys       bson.D
	Unique     bool
}

var indexSpecs = []indexSpec{
	{"users", bson.D{{Key: "user_id", Value: 1}}, true},
	{"categories", bson.D{{Key: "cat_id", Value: 1}}, true},
	{"categories", bson.D{{Key: "order", Value: 1}}, false},
	{"subjects", bson.D{{Key: "sub_id", Value: 1}}, true},
	{"subjects", bson.D{{Key: "category_id", Value: 1}}, false},
	{"files", bson.D{{Key: "file_id", Value: 1}}, true},
	{"files", bson.D{{Key: "subject_id", Value: 1}}, false},
	{"files", bson.D{{Key: "name", Value: 1}}, false},
	{"files", bson.D{{Key: "downloads", Value: -1}}, false},
	{"files", bson.D{{Key: "upload_date", Value: -1}}, false},
	{"file_ratings", bson.D{{Key: "user_id", Value: 1}, {Key: "file_id", Value: 1}}, true},
	{"file_ratings", bson.D{{Key: "file_id", Value: 1}}, false},
	{"subject_subscriptions", bson.D{{Key: "user_id", Value: 1}, {Key: "subject_id", Value: 1}}, true},
	{"subject_subscriptions", bson.D{{Key: "subject_id", Value: 1}}, false},
	{"shared_files", bson.D{{Key: "share_hash", Value: 1}}, true},
	{"shared_files", bson.D{{Key: "expires_at", Value: 1}}, false},
	{"bot_settings", bson.D{{Key: "key", Value: 1}}, true},
	{"managed_bots", bson.D{{Key: "owner_id", Value: 1}, {Key: "created_at", Value: -1}}, false},
	{"managed_bots", bson.D{{Key: "token_hash", Value: 1}}, true},
	{"managed_bots", bson.D{{Key: "status", Value: 1}, {Key: "updated_at", Value: -1}}, false},
	{"admin_logs", bson.D{{Key: "admin_id", Value: 1}, {Key: "timestamp", Value: -1}}, false},
	{"admin_logs", bson.D{{Key: "timestamp", Value: -1}}, false},
}

// EnsureIndexes creates all required MongoDB indexes.
func EnsureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, spec := range indexSpecs {
		model := mongo.IndexModel{
			Keys: spec.Keys,
		}
		if spec.Unique {
			model.Options = options.Index().SetUnique(true)
		}
		_, err := Col(spec.Collection).Indexes().CreateOne(ctx, model)
		if err != nil {
			log.Printf("Warning: Index create failed (%s): %v", spec.Collection, err)
		}
	}
	log.Println("MongoDB indexes ensured")
}

// GetNextID atomically increments and returns the next ID for a collection.
func GetNextID(ctx context.Context, collectionName string) (int, error) {
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var result struct {
		Seq int `bson:"seq"`
	}
	err := Col("counters").FindOneAndUpdate(
		ctx,
		bson.M{"_id": collectionName},
		bson.M{"$inc": bson.M{"seq": 1}},
		opts,
	).Decode(&result)
	if err != nil {
		return 0, err
	}
	return result.Seq, nil
}

// InitCounters ensures counter documents exist for each tracked collection.
func InitCounters() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, name := range []string{"users", "categories", "subjects", "files"} {
		opts := options.Update().SetUpsert(true)
		_, err := Col("counters").UpdateOne(
			ctx,
			bson.M{"_id": name},
			bson.M{"$setOnInsert": bson.M{"seq": 1}},
			opts,
		)
		if err != nil {
			log.Printf("Warning: InitCounters failed for %s: %v", name, err)
		}
	}
}
