package factory

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/db"
)

const (
	workerLeaseDuration = 90 * time.Second
	workerLeaseRenewal  = 30 * time.Second
)

func primaryLeaseID(telegramBotID int64) string {
	return fmt.Sprintf("primary:%d", telegramBotID)
}

func managedLeaseID(telegramBotID int64) string {
	return fmt.Sprintf("managed:%d", telegramBotID)
}

// AcquirePrimaryLease reserves the parent bot's update stream for one service instance.
func (m *Manager) AcquirePrimaryLease(ctx context.Context, telegramBotID int64) bool {
	return m.acquireWorkerLease(ctx, primaryLeaseID(telegramBotID), telegramBotID)
}

// RenewPrimaryLease extends the parent bot update-stream reservation.
func (m *Manager) RenewPrimaryLease(ctx context.Context, telegramBotID int64) bool {
	return m.renewWorkerLease(ctx, primaryLeaseID(telegramBotID))
}

// ReleasePrimaryLease releases the parent bot update-stream reservation.
func (m *Manager) ReleasePrimaryLease(ctx context.Context, telegramBotID int64) {
	m.releaseWorkerLease(ctx, primaryLeaseID(telegramBotID))
}

func (m *Manager) acquireWorkerLease(ctx context.Context, recordID string, telegramBotID int64) bool {
	if recordID == "" || m.instanceID == "" {
		return false
	}
	now := time.Now().UTC()
	leaseUntil := now.Add(workerLeaseDuration)
	filter := bson.M{
		"_id": recordID,
		"$or": []bson.M{
			{"lease_until": bson.M{"$lte": now}},
			{"lease_until": bson.M{"$exists": false}},
			{"lease_owner": m.instanceID},
		},
	}
	update := bson.M{"$set": bson.M{
		"lease_owner":     m.instanceID,
		"telegram_bot_id": telegramBotID,
		"lease_until":     leaseUntil,
		"updated_at":      now,
	}}
	var claimed bson.M
	err := db.Col("worker_leases").FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&claimed)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("managed bot %s lease acquisition failed: %v", recordID, err)
		}
		return false
	}
	owner, _ := claimed["lease_owner"].(string)
	return owner == m.instanceID
}

func (m *Manager) renewWorkerLease(ctx context.Context, recordID string) bool {
	if recordID == "" || m.instanceID == "" {
		return false
	}
	result, err := db.Col("worker_leases").UpdateOne(ctx,
		bson.M{"_id": recordID, "lease_owner": m.instanceID},
		bson.M{"$set": bson.M{"lease_until": time.Now().UTC().Add(workerLeaseDuration), "updated_at": time.Now().UTC()}},
	)
	return err == nil && result.MatchedCount == 1
}

func (m *Manager) releaseWorkerLease(ctx context.Context, recordID string) {
	if recordID == "" || m.instanceID == "" {
		return
	}
	_, _ = db.Col("worker_leases").DeleteOne(ctx, bson.M{"_id": recordID, "lease_owner": m.instanceID})
}
