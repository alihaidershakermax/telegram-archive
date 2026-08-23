package factory

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"telegram-archive-bot/db"
)

const (
	workerLeaseDuration      = 90 * time.Second
	workerLeaseRenewal       = 30 * time.Second
	workerLeaseRetryInterval = 5 * time.Second
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

// WaitForPrimaryLease retries until the parent update stream is available or stop is closed.
func (m *Manager) WaitForPrimaryLease(ctx context.Context, telegramBotID int64, stop <-chan struct{}) bool {
	return m.waitForWorkerLease(ctx, primaryLeaseID(telegramBotID), telegramBotID, stop)
}

// RenewPrimaryLease extends the parent bot update-stream reservation.
func (m *Manager) RenewPrimaryLease(ctx context.Context, telegramBotID int64) bool {
	return m.renewWorkerLease(ctx, primaryLeaseID(telegramBotID))
}

// ReleasePrimaryLease releases the parent bot update-stream reservation.
func (m *Manager) ReleasePrimaryLease(ctx context.Context, telegramBotID int64) {
	m.releaseWorkerLease(ctx, primaryLeaseID(telegramBotID))
}

func (m *Manager) waitForWorkerLease(ctx context.Context, recordID string, telegramBotID int64, stop <-chan struct{}) bool {
	for {
		if stop != nil {
			select {
			case <-stop:
				return false
			default:
			}
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		claimed := m.acquireWorkerLease(attemptCtx, recordID, telegramBotID)
		cancel()
		if claimed {
			return true
		}
		timer := time.NewTimer(workerLeaseRetryInterval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-stop:
			timer.Stop()
			return false
		}
	}
}

func (m *Manager) acquireWorkerLease(ctx context.Context, recordID string, telegramBotID int64) bool {
	if recordID == "" || m.instanceID == "" {
		return false
	}
	now := time.Now().UTC()
	leaseUntil := now.Add(workerLeaseDuration)
	leaseFilter := bson.M{
		"_id": recordID,
		"$or": []bson.M{
			{"lease_until": bson.M{"$lte": now}},
			{"lease_until": bson.M{"$exists": false}},
			{"lease_owner": m.instanceID},
		},
	}
	leaseUpdate := bson.M{"$set": bson.M{
		"lease_owner":     m.instanceID,
		"telegram_bot_id": telegramBotID,
		"lease_until":     leaseUntil,
		"updated_at":      now,
	}}

	// Update an existing, available lease first. Avoiding upsert here prevents
	// MongoDB from attempting an insert when an active lease document exists.
	result, err := db.Col("worker_leases").UpdateOne(ctx, leaseFilter, leaseUpdate)
	if err != nil {
		log.Printf("managed bot %s lease update failed: %v", recordID, err)
		return false
	}
	if result.MatchedCount == 1 {
		return true
	}

	// There is no matching document. Insert is safe under a race: exactly one
	// instance wins the unique _id and the others retry after the interval.
	_, err = db.Col("worker_leases").InsertOne(ctx, bson.M{
		"_id":             recordID,
		"lease_owner":     m.instanceID,
		"telegram_bot_id": telegramBotID,
		"lease_until":     leaseUntil,
		"updated_at":      now,
	})
	if err == nil {
		return true
	}
	if mongo.IsDuplicateKeyError(err) {
		return false
	}
	log.Printf("managed bot %s lease insert failed: %v", recordID, err)
	return false
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
