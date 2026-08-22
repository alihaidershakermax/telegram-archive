package factory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

func (m *Manager) AddStorageCluster(ctx context.Context, name, uri string) (*models.StorageCluster, error) {
	name = strings.TrimSpace(name)
	uri = strings.TrimSpace(uri)
	if name == "" || len(name) > 64 {
		return nil, errors.New("cluster name is required and must be at most 64 characters")
	}
	if err := validateMongoURI(uri); err != nil {
		return nil, err
	}
	if _, err := db.Col("storage_clusters").CountDocuments(ctx, map[string]interface{}{"name": name}); err != nil {
		return nil, errors.New("cannot inspect cluster registry")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	client, err := mongo.Connect(pingCtx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(8*time.Second).SetConnectTimeout(8*time.Second))
	if err != nil {
		return nil, errors.New("cluster connection failed")
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, errors.New("cluster ping failed")
	}
	ciphertext, nonce, err := m.cipher.Encrypt(uri)
	if err != nil {
		return nil, errors.New("cluster credential encryption failed")
	}
	now := time.Now().UTC()
	record := &models.StorageCluster{ID: newID(), Name: name, URICiphertext: ciphertext, URINonce: nonce, DatabasePrefix: "archive_bot_", Status: models.StorageClusterActive, LastHealthAt: now, CreatedAt: now, UpdatedAt: now}
	if _, err := db.Col("storage_clusters").InsertOne(ctx, record); err != nil {
		return nil, errors.New("cluster name is already registered or cannot be saved")
	}
	if err := db.RegisterExternalClient(ctx, record.ID, uri); err != nil {
		_, _ = db.Col("storage_clusters").DeleteOne(ctx, map[string]interface{}{"_id": record.ID})
		return nil, errors.New("cluster connection could not be registered")
	}
	return record, nil
}

func (m *Manager) loadStorageClusters(ctx context.Context) error {
	rows, err := m.ListStorageClusters(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == models.StorageClusterOffline {
			continue
		}
		uri, err := m.cipher.Decrypt(row.URICiphertext, row.URINonce)
		if err != nil {
			return errors.New("stored cluster credential could not be decrypted")
		}
		if err := db.RegisterExternalClient(ctx, row.ID, uri); err != nil {
			_, _ = db.Col("storage_clusters").UpdateOne(ctx, map[string]interface{}{"_id": row.ID}, map[string]interface{}{"$set": map[string]interface{}{"status": models.StorageClusterOffline, "last_error": "health check failed", "updated_at": time.Now().UTC()}})
		}
	}
	return nil
}

func (m *Manager) pickStorageCluster(ctx context.Context) string {
	rows, err := m.ListStorageClusters(ctx)
	if err != nil || len(rows) == 0 {
		return ""
	}
	counts := make(map[string]int)
	cursor, err := db.Col("managed_bots").Find(ctx, map[string]interface{}{"cluster_id": map[string]interface{}{"$ne": ""}}, options.Find().SetProjection(map[string]int{"cluster_id": 1}).SetLimit(10000))
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var row struct {
				ClusterID string `bson:"cluster_id"`
			}
			if cursor.Decode(&row) == nil {
				counts[row.ClusterID]++
			}
		}
	}
	best := ""
	bestCount := int(^uint(0) >> 1)
	for _, row := range rows {
		if row.Status != models.StorageClusterActive || counts[row.ID] >= bestCount {
			continue
		}
		best, bestCount = row.ID, counts[row.ID]
	}
	return best
}

func (m *Manager) ListStorageClusters(ctx context.Context) ([]models.StorageCluster, error) {
	cursor, err := db.Col("storage_clusters").Find(ctx, map[string]interface{}{}, options.Find().SetSort(map[string]int{"created_at": 1}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	rows := make([]models.StorageCluster, 0)
	for cursor.Next(ctx) {
		var row models.StorageCluster
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, cursor.Err()
}

func (m *Manager) DeleteStorageCluster(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("cluster id is required")
	}
	assigned, err := db.Col("managed_bots").CountDocuments(ctx, map[string]interface{}{"cluster_id": id})
	if err != nil {
		return errors.New("cannot verify cluster assignments")
	}
	if assigned > 0 {
		return errors.New("cluster has assigned bots; drain it before removal")
	}
	result, err := db.Col("storage_clusters").DeleteOne(ctx, map[string]interface{}{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *Manager) SetStorageClusterStatus(ctx context.Context, id, status string) error {
	if status != models.StorageClusterActive && status != models.StorageClusterDraining && status != models.StorageClusterOffline {
		return errors.New("invalid cluster status")
	}
	result, err := db.Col("storage_clusters").UpdateOne(ctx, map[string]interface{}{"_id": id}, map[string]interface{}{"$set": map[string]interface{}{"status": status, "updated_at": time.Now().UTC()}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func validateMongoURI(uri string) error {
	if strings.HasPrefix(uri, "mongodb+srv://") || strings.HasPrefix(uri, "mongodb://") {
		if len(uri) <= len("mongodb://") {
			return errors.New("mongodb URI is incomplete")
		}
		return nil
	}
	return fmt.Errorf("mongodb URI must start with mongodb:// or mongodb+srv://")
}
