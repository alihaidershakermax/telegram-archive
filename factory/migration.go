package factory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

// NamespaceMigration records an online move while preserving the source route.
type NamespaceMigration struct {
	ID          string    `bson:"_id" json:"id"`
	BotID       int64     `bson:"bot_id" json:"bot_id"`
	Source      string    `bson:"source_cluster" json:"source_cluster"`
	Target      string    `bson:"target_cluster" json:"target_cluster"`
	Status      string    `bson:"status" json:"status"`
	Collections int       `bson:"collections" json:"collections"`
	Documents   int64     `bson:"documents" json:"documents"`
	Checksum    string    `bson:"checksum" json:"checksum"`
	LastError   string    `bson:"last_error,omitempty" json:"last_error,omitempty"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}

const (
	MigrationPending   = "pending"
	MigrationCopying   = "copying"
	MigrationCutover   = "cutover"
	MigrationCompleted = "completed"
	MigrationFailed    = "failed"
)

func (m *Manager) ListNamespaceMigrations(ctx context.Context, botID int64) ([]NamespaceMigration, error) {
	filter := bson.M{}
	if botID != 0 {
		filter["bot_id"] = botID
	}
	cursor, err := db.Col("namespace_migrations").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	rows := make([]NamespaceMigration, 0)
	for cursor.Next(ctx) {
		var row NamespaceMigration
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, cursor.Err()
}

func (m *Manager) MigrateBotNamespace(ctx context.Context, botID int64, targetClusterID string) error {
	if botID == 0 || targetClusterID == "" {
		return errors.New("bot id and target cluster are required")
	}
	var bot models.ManagedBot
	if err := db.Col("managed_bots").FindOne(ctx, bson.M{"telegram_bot_id": botID}).Decode(&bot); err != nil {
		return ErrNotFound
	}
	if bot.ClusterID == targetClusterID {
		return errors.New("bot is already assigned to target cluster")
	}
	target := db.DatabaseForCluster(targetClusterID, bot.DatabaseName)
	if target == nil {
		return errors.New("target cluster is not connected")
	}
	source := db.DatabaseForCluster(bot.ClusterID, bot.DatabaseName)
	if source == nil {
		return errors.New("source database is not connected")
	}
	job := &NamespaceMigration{ID: newID(), BotID: botID, Source: bot.ClusterID, Target: targetClusterID, Status: MigrationCopying, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := db.Col("namespace_migrations").InsertOne(ctx, job); err != nil {
		return err
	}
	if err := copyDatabase(ctx, source, target, job); err != nil {
		m.failMigration(ctx, job.ID, err)
		return err
	}
	m.stopWorker(bot.ID)
	// The short cutover pass runs while updates are stopped, so the target is a
	// complete snapshot even for legacy documents without updated_at.
	if err := copyDatabase(ctx, source, target, job); err != nil {
		m.failMigration(ctx, job.ID, err)
		_ = m.startRecord(ctx, bot)
		return err
	}
	if _, err := db.Col("managed_bots").UpdateOne(ctx, bson.M{"_id": bot.ID}, bson.M{"$set": bson.M{"cluster_id": targetClusterID, "updated_at": time.Now().UTC()}}); err != nil {
		_ = m.startRecord(ctx, bot)
		m.failMigration(ctx, job.ID, err)
		return err
	}
	db.SetBotClusterRoute(botID, targetClusterID)
	job.Status = MigrationCompleted
	job.UpdatedAt = time.Now().UTC()
	_, _ = db.Col("namespace_migrations").ReplaceOne(ctx, bson.M{"_id": job.ID}, job)
	bot.ClusterID = targetClusterID
	if err := m.startRecord(ctx, bot); err != nil {
		return err
	}
	return nil
}

func copyDatabase(ctx context.Context, source, target *mongo.Database, job *NamespaceMigration) error {
	collections, err := source.ListCollectionNames(ctx, bson.M{"name": bson.M{"$not": bson.M{"$regex": "^system\\."}}})
	if err != nil {
		return err
	}
	var total int64
	hash := sha256.New()
	for _, name := range collections {
		cursor, err := source.Collection(name).Find(ctx, bson.M{}, options.Find().SetBatchSize(500))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		for cursor.Next(ctx) {
			var raw bson.Raw
			if err := cursor.Decode(&raw); err != nil {
				_ = cursor.Close(ctx)
				return err
			}
			id := raw.Lookup("_id")
			if id.Type == 0 {
				continue
			}
			if _, err := target.Collection(name).ReplaceOne(ctx, bson.M{"_id": id}, raw, options.Replace().SetUpsert(true)); err != nil {
				_ = cursor.Close(ctx)
				return fmt.Errorf("write %s: %w", name, err)
			}
			total++
			_, _ = hash.Write(raw)
		}
		if err := cursor.Err(); err != nil {
			_ = cursor.Close(ctx)
			return err
		}
		_ = cursor.Close(ctx)
	}
	job.Documents = total
	job.Collections = len(collections)
	job.Checksum = hex.EncodeToString(hash.Sum(nil))
	job.UpdatedAt = time.Now().UTC()
	_, _ = db.Col("namespace_migrations").ReplaceOne(ctx, bson.M{"_id": job.ID}, job)
	return nil
}

func (m *Manager) failMigration(ctx context.Context, id string, cause error) {
	_, _ = db.Col("namespace_migrations").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": MigrationFailed, "last_error": cause.Error(), "updated_at": time.Now().UTC()}})
}
