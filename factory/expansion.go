package factory

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

const currentExpansionSchema = 1

type AutoExpansionController struct {
	cfg      *config.Config
	owner    string
	cancel   context.CancelFunc
	stop     chan struct{}
	stopOnce sync.Once
}

func StartAutoExpansion(parent context.Context, cfg *config.Config) *AutoExpansionController {
	ctx, cancel := context.WithCancel(parent)
	controller := &AutoExpansionController{cfg: cfg, owner: fmt.Sprintf("parent-%d", time.Now().UnixNano()), cancel: cancel, stop: make(chan struct{})}
	if !cfg.DBAutoExpansion {
		log.Println("Database auto-expansion disabled")
		return controller
	}
	go controller.loop(ctx)
	return controller
}

func (c *AutoExpansionController) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() {
		c.cancel()
		close(c.stop)
	})
}

func (c *AutoExpansionController) loop(ctx context.Context) {
	poll := time.Duration(c.cfg.DBExpansionPollSeconds) * time.Second
	if poll < 10*time.Second {
		poll = 10 * time.Second
	}
	_ = c.RunOnce(ctx)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.RunOnce(ctx); err != nil {
				log.Printf("Database auto-expansion scan failed: %v", err)
			}
		}
	}
}

// RunOnce is intentionally idempotent and safe to call from tests or an operator endpoint.
func (c *AutoExpansionController) RunOnce(ctx context.Context) error {
	if c == nil || c.cfg == nil || !c.cfg.DBAutoExpansion {
		return nil
	}
	cursor, err := db.Col("managed_bots").Find(ctx, bson.M{"status": models.ManagedBotActive}, options.Find().SetLimit(1000))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var bot models.ManagedBot
		if err := cursor.Decode(&bot); err != nil {
			continue
		}
		if err := c.expandBot(ctx, bot); err != nil {
			log.Printf("Auto-expansion failed for bot %d: %v", bot.TelegramBotID, err)
		}
	}
	return cursor.Err()
}

func (c *AutoExpansionController) expandBot(ctx context.Context, bot models.ManagedBot) error {
	now := time.Now().UTC()
	state := models.ExpansionState{BotID: bot.TelegramBotID, DatabaseName: bot.DatabaseName, SchemaVersion: 0, Status: models.ExpansionReady, CapacityTier: "standard"}
	_, err := db.Col("expansion_states").UpdateOne(ctx, bson.M{"_id": bot.TelegramBotID}, bson.M{"$setOnInsert": state}, options.Update().SetUpsert(true))
	if err != nil {
		return err
	}
	// Compose lock and cooldown clauses explicitly so two controllers cannot expand one bot concurrently.
	filter := bson.M{"_id": bot.TelegramBotID, "$and": []bson.M{
		{"$or": []bson.M{{"lock_until": bson.M{"$lte": now}}, {"lock_until": bson.M{"$exists": false}}}},
		{"$or": []bson.M{{"next_eligible_at": bson.M{"$lte": now}}, {"next_eligible_at": bson.M{"$exists": false}}}},
	}}
	leaseUntil := now.Add(time.Duration(c.cfg.DBExpansionLockSeconds) * time.Second)
	var claimed models.ExpansionState
	err = db.Col("expansion_states").FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"status": models.ExpansionRunning, "lock_owner": c.owner, "lock_until": leaseUntil, "last_run_at": now, "database_name": bot.DatabaseName, "schema_version": currentExpansionSchema}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&claimed)
	if err != nil {
		if errors.Is(err, mongoErrNoDocuments()) {
			return nil
		}
		return err
	}

	botCtx := db.WithBotDatabase(ctx, bot.TelegramBotID)
	if err := ensureBotSchema(botCtx); err != nil {
		return c.fail(ctx, bot.TelegramBotID, err)
	}
	if _, err := db.Col("expansion_states").UpdateOne(ctx, bson.M{"_id": bot.TelegramBotID, "lock_owner": c.owner}, bson.M{"$set": bson.M{"schema_version": currentExpansionSchema}}); err != nil {
		return c.fail(ctx, bot.TelegramBotID, err)
	}
	users, err := db.ColScoped(botCtx, "users").CountDocuments(botCtx, bson.M{})
	if err != nil {
		return c.fail(ctx, bot.TelegramBotID, err)
	}
	files, err := db.ColScoped(botCtx, "files").CountDocuments(botCtx, bson.M{})
	if err != nil {
		return c.fail(ctx, bot.TelegramBotID, err)
	}
	storageBytes, err := totalFileBytes(botCtx)
	if err != nil {
		return c.fail(ctx, bot.TelegramBotID, err)
	}
	tier := capacityTier(users, files, storageBytes, c.cfg)
	completed := time.Now().UTC()
	_, err = db.Col("expansion_states").UpdateOne(ctx, bson.M{"_id": bot.TelegramBotID, "lock_owner": c.owner}, bson.M{"$set": bson.M{
		"status": models.ExpansionComplete, "users_count": users, "files_count": files, "storage_bytes": storageBytes,
		"capacity_tier": tier, "last_completed_at": completed, "next_eligible_at": completed.Add(time.Duration(c.cfg.DBExpansionCooldownSeconds) * time.Second),
		"lock_until": time.Time{}, "last_error": "",
	}, "$inc": bson.M{"expansion_count": 1}})
	return err
}

func (c *AutoExpansionController) fail(ctx context.Context, botID int64, cause error) error {
	_, _ = db.Col("expansion_states").UpdateOne(ctx, bson.M{"_id": botID, "lock_owner": c.owner}, bson.M{"$set": bson.M{"status": models.ExpansionFailed, "last_error": cause.Error(), "lock_until": time.Time{}, "next_eligible_at": time.Now().UTC().Add(time.Minute)}})
	return cause
}

func ensureBotSchema(ctx context.Context) error {
	// Index creation is idempotent. The migration registry makes the same guarantee
	// explicit and provides a recovery point for future additive migrations.
	db.EnsureIndexesForContext(ctx)
	_, err := db.ColScoped(ctx, "schema_migrations").UpdateOne(ctx, bson.M{"_id": "001_base_indexes"}, bson.M{"$set": bson.M{
		"version": currentExpansionSchema, "applied_at": time.Now().UTC(),
	}}, options.Update().SetUpsert(true))
	return err
}

func totalFileBytes(ctx context.Context) (int64, error) {
	pipeline := []bson.D{{{Key: "$group", Value: bson.D{{Key: "_id", Value: nil}, {Key: "total", Value: bson.D{{Key: "$sum", Value: "$file_size"}}}}}}}
	cursor, err := db.ColScoped(ctx, "files").Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	if !cursor.Next(ctx) {
		return 0, cursor.Err()
	}
	var row struct {
		Total int64 `bson:"total"`
	}
	if err := cursor.Decode(&row); err != nil {
		return 0, err
	}
	return row.Total, nil
}

func capacityTier(users, files, bytes int64, cfg *config.Config) string {
	if (cfg.DBExpansionMaxDocs > 0 && (users >= cfg.DBExpansionMaxDocs || files >= cfg.DBExpansionMaxDocs)) || (cfg.DBExpansionMaxBytes > 0 && bytes >= cfg.DBExpansionMaxBytes) {
		return "expanded"
	}
	return "standard"
}

// mongoErrNoDocuments is isolated to keep the controller's error branch readable.
func mongoErrNoDocuments() error { return mongo.ErrNoDocuments }

// ExpansionState returns the latest state for a managed bot.
func ExpansionState(ctx context.Context, botID int64) (*models.ExpansionState, error) {
	var state models.ExpansionState
	if err := db.Col("expansion_states").FindOne(ctx, bson.M{"_id": botID}).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *AutoExpansionController) String() string { return strings.TrimSpace(c.owner) }
