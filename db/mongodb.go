package db

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
)

var (
	client           *mongo.Client
	db               *mongo.Database
	externalMu       sync.RWMutex
	externalClients  = map[string]*mongo.Client{}
	botClusterRoutes = map[int64]string{}
)

type botDatabaseKey struct{}

// WithBotDatabase scopes archive queries to a dedicated MongoDB database.
// The primary bot keeps using the configured legacy database when no scope is set.
type botClusterKey struct{}

func WithBotDatabase(ctx context.Context, telegramBotID int64) context.Context {
	if telegramBotID == 0 {
		return ctx
	}
	ctx = context.WithValue(ctx, botDatabaseKey{}, fmt.Sprintf("archive_bot_%d", telegramBotID))
	externalMu.RLock()
	clusterID := botClusterRoutes[telegramBotID]
	externalMu.RUnlock()
	if clusterID != "" {
		ctx = context.WithValue(ctx, botClusterKey{}, clusterID)
	}
	return ctx
}

func ScopeKey(ctx context.Context) string {
	if name, ok := ctx.Value(botDatabaseKey{}).(string); ok && name != "" {
		return name
	}
	return "primary"
}

func IsScoped(ctx context.Context) bool {
	return ScopeKey(ctx) != "primary"
}

func databaseForContext(ctx context.Context) *mongo.Database {
	name, _ := ctx.Value(botDatabaseKey{}).(string)
	if name != "" {
		clusterID, _ := ctx.Value(botClusterKey{}).(string)
		externalMu.RLock()
		external := externalClients[clusterID]
		primary := client
		externalMu.RUnlock()
		if clusterID != "" && external != nil {
			return external.Database(name)
		}
		if primary != nil {
			return primary.Database(name)
		}
	}
	return db
}

// RegisterExternalClient verifies and keeps a separate MongoDB cluster connection.
func RegisterExternalClient(ctx context.Context, clusterID, uri string) error {
	pingCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	candidate, err := mongo.Connect(pingCtx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(8*time.Second).SetConnectTimeout(8*time.Second))
	if err != nil {
		return err
	}
	if err := candidate.Ping(pingCtx, nil); err != nil {
		_ = candidate.Disconnect(context.Background())
		return err
	}
	externalMu.Lock()
	old := externalClients[clusterID]
	externalClients[clusterID] = candidate
	externalMu.Unlock()
	if old != nil {
		_ = old.Disconnect(context.Background())
	}
	return nil
}

func DatabaseForCluster(clusterID, databaseName string) *mongo.Database {
	if databaseName == "" {
		return nil
	}
	externalMu.RLock()
	defer externalMu.RUnlock()
	if clusterID == "" {
		if client == nil {
			return nil
		}
		return client.Database(databaseName)
	}
	external := externalClients[clusterID]
	if external == nil {
		return nil
	}
	return external.Database(databaseName)
}

func SetBotClusterRoute(botID int64, clusterID string) {
	externalMu.Lock()
	defer externalMu.Unlock()
	if clusterID == "" {
		delete(botClusterRoutes, botID)
		return
	}
	botClusterRoutes[botID] = clusterID
}

// ColScoped returns a collection from the bot-specific database when the
// context carries a managed-bot scope, otherwise it returns the primary DB collection.
func ColScoped(ctx context.Context, name string) *mongo.Collection {
	return databaseForContext(ctx).Collection(name)
}

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	externalMu.Lock()
	primary := client
	client = nil
	externals := make([]*mongo.Client, 0, len(externalClients))
	for clusterID, external := range externalClients {
		if external != nil {
			externals = append(externals, external)
		}
		delete(externalClients, clusterID)
	}
	botClusterRoutes = make(map[int64]string)
	externalMu.Unlock()

	if primary != nil {
		_ = primary.Disconnect(ctx)
	}
	for _, external := range externals {
		_ = external.Disconnect(ctx)
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
	{"group_configs", bson.D{{Key: "bot_id", Value: 1}, {Key: "chat_id", Value: 1}}, true},
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
	{"subject_subscriptions", bson.D{{Key: "bot_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "subject_id", Value: 1}}, true},
	{"subject_subscriptions", bson.D{{Key: "bot_id", Value: 1}, {Key: "subject_id", Value: 1}}, false},
	{"vault_items", bson.D{{Key: "bot_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "file_id", Value: 1}}, true},
	{"vault_items", bson.D{{Key: "bot_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "added_at", Value: -1}}, false},
	{"subject_notifications", bson.D{{Key: "bot_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "file_id", Value: 1}}, true},
	{"shared_files", bson.D{{Key: "share_hash", Value: 1}}, true},
	{"shared_files", bson.D{{Key: "expires_at", Value: 1}}, false},
	{"bot_settings", bson.D{{Key: "key", Value: 1}}, true},
	{"managed_bots", bson.D{{Key: "owner_id", Value: 1}, {Key: "created_at", Value: -1}}, false},
	{"managed_bots", bson.D{{Key: "token_hash", Value: 1}}, true},
	{"managed_bots", bson.D{{Key: "telegram_bot_id", Value: 1}}, true},
	{"managed_bots", bson.D{{Key: "status", Value: 1}, {Key: "updated_at", Value: -1}}, false},
	{"worker_leases", bson.D{{Key: "lease_until", Value: 1}}, false},
	{"admin_logs", bson.D{{Key: "admin_id", Value: 1}, {Key: "timestamp", Value: -1}}, false},
	{"admin_logs", bson.D{{Key: "timestamp", Value: -1}}, false},
	{"storage_jobs", bson.D{{Key: "status", Value: 1}, {Key: "next_attempt_at", Value: 1}}, false},
	{"storage_jobs", bson.D{{Key: "bot_id", Value: 1}, {Key: "created_at", Value: -1}}, false},
	{"api_keys", bson.D{{Key: "key_hash", Value: 1}}, true},
	{"api_keys", bson.D{{Key: "bot_id", Value: 1}, {Key: "created_at", Value: -1}}, false},
	{"bot_backups", bson.D{{Key: "bot_id", Value: 1}, {Key: "created_at", Value: -1}}, false},
	{"bot_backup_data", bson.D{{Key: "backup_id", Value: 1}, {Key: "collection", Value: 1}}, false},
	{"ai_usage", bson.D{{Key: "created_at", Value: -1}}, false},
	{"ai_usage", bson.D{{Key: "operation", Value: 1}, {Key: "created_at", Value: -1}}, false},
	{"ai_indexes", bson.D{{Key: "file_id", Value: 1}}, true},
	{"expansion_states", bson.D{{Key: "status", Value: 1}, {Key: "next_eligible_at", Value: 1}}, false},
	{"storage_clusters", bson.D{{Key: "name", Value: 1}}, true},
	{"storage_clusters", bson.D{{Key: "status", Value: 1}, {Key: "updated_at", Value: -1}}, false},
}

// EnsureIndexes creates all required MongoDB indexes in the primary database.
func EnsureIndexes() {
	EnsureIndexesForContext(context.Background())
}

// EnsureIndexesForContext creates the same indexes in the database selected by ctx.
func EnsureIndexesForContext(ctx context.Context) {
	for _, spec := range indexSpecs {
		model := mongo.IndexModel{Keys: spec.Keys}
		if spec.Unique {
			model.Options = options.Index().SetUnique(true)
		}
		_, err := ColScoped(ctx, spec.Collection).Indexes().CreateOne(ctx, model)
		if err != nil {
			log.Printf("Warning: Index create failed (%s, %s): %v", ScopeKey(ctx), spec.Collection, err)
		}
	}
	log.Printf("MongoDB indexes ensured for %s", ScopeKey(ctx))
}

// GetNextID atomically increments and returns the next ID for a collection.
func GetNextID(ctx context.Context, collectionName string) (int, error) {
	return GetNextIDScoped(ctx, collectionName)
}

// GetNextIDScoped keeps auto-increment counters inside each bot database.
func GetNextIDScoped(ctx context.Context, collectionName string) (int, error) {
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var result struct {
		Seq int `bson:"seq"`
	}
	err := ColScoped(ctx, "counters").FindOneAndUpdate(
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
