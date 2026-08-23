package factory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

const (
	healthInterval       = 60 * time.Second
	maxConsecutiveErrors = 3
)

var ErrNotFound = mongo.ErrNoDocuments

type workerStats struct {
	activeRequests atomic.Int64
	totalUpdates   atomic.Int64
	totalErrors    atomic.Int64
	lastLatencyNS  atomic.Int64
	consecutive    atomic.Int64
	lastSeenUnix   atomic.Int64
}

type managedWorker struct {
	bot         *tgbotapi.BotAPI
	stop        chan struct{}
	stats       *workerStats
	once        sync.Once
	updatesOnce sync.Once
	closed      chan struct{}
}

// Manager owns the lifecycle and routing policy for all managed bots.
type Manager struct {
	cfg        *config.Config
	cipher     *TokenCipher
	handler    func(*tgbotapi.BotAPI, tgbotapi.Update)
	instanceID string

	mu      sync.RWMutex
	workers map[string]*managedWorker
	slots   chan struct{}
}

// NewManager creates a Bot Factory manager. The encryption key is mandatory
// because plaintext bot tokens must never be persisted.
func NewManager(cfg *config.Config, handler func(*tgbotapi.BotAPI, tgbotapi.Update)) (*Manager, error) {
	if cfg == nil {
		return nil, errors.New("factory configuration is missing")
	}
	cipher, err := NewTokenCipher(cfg.FactoryEncryptionKey)
	if err != nil {
		return nil, err
	}
	maxBots := cfg.FactoryMaxBotsPerOwner
	if maxBots <= 0 {
		cfg.FactoryMaxBotsPerOwner = 5
	}
	workers := cfg.FactoryWorkers
	if workers <= 0 {
		cfg.FactoryWorkers = 8
	}
	return &Manager{cfg: cfg, cipher: cipher, handler: handler, instanceID: newID(), workers: make(map[string]*managedWorker), slots: make(chan struct{}, cfg.FactoryWorkers)}, nil
}

// LoadAndStart restores active managed bots after a process restart.
// DefaultOwnerID returns the configured platform owner for API registrations.
func (m *Manager) DefaultOwnerID() int64 { return m.cfg.OwnerID }

func (m *Manager) LoadAndStart(ctx context.Context) error {
	if err := m.loadStorageClusters(ctx); err != nil {
		log.Printf("Storage cluster restore warning: %v", err)
	}
	cursor, err := db.Col("managed_bots").Find(ctx, bson.M{"status": models.ManagedBotActive}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var records []models.ManagedBot
	if err := cursor.All(ctx, &records); err != nil {
		return err
	}
	for _, record := range records {
		if err := m.startRecord(ctx, record); err != nil {
			log.Printf("managed bot %s failed to start: %v", record.ID, err)
			m.markUnhealthy(ctx, record.ID, err)
		}
	}
	return nil
}

// Register validates a user-supplied token with Telegram, encrypts it, stores
// metadata only, and starts an isolated update worker.
func (m *Manager) Register(ctx context.Context, ownerID int64, token string) (*models.ManagedBot, error) {
	token = strings.TrimSpace(token)
	if err := validateTokenShape(token); err != nil {
		return nil, err
	}
	if ownerID == 0 {
		return nil, errors.New("owner id is required")
	}

	count, err := db.Col("managed_bots").CountDocuments(ctx, bson.M{"owner_id": ownerID, "status": bson.M{"$ne": models.ManagedBotPaused}})
	if err != nil {
		return nil, err
	}
	if int(count) >= m.cfg.FactoryMaxBotsPerOwner {
		return nil, fmt.Errorf("bot limit reached (%d)", m.cfg.FactoryMaxBotsPerOwner)
	}

	hash := TokenHash(token)
	if err := db.Col("managed_bots").FindOne(ctx, bson.M{"token_hash": hash}).Err(); err == nil {
		return nil, errors.New("this bot token is already registered")
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram token validation failed: %w", err)
	}
	if err := db.Col("managed_bots").FindOne(ctx, bson.M{"telegram_bot_id": bot.Self.ID}).Err(); err == nil {
		return nil, errors.New("this Telegram bot is already registered")
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	ciphertext, nonce, err := m.cipher.Encrypt(token)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	namespace, folder := botNamespace(bot.Self.UserName, bot.Self.ID)
	clusterID := m.pickStorageCluster(ctx)
	record := models.ManagedBot{

		ID:              newID(),
		OwnerID:         ownerID,
		TokenCiphertext: ciphertext,
		TokenNonce:      nonce,
		TokenHash:       hash,
		TelegramBotID:   bot.Self.ID,
		Username:        bot.Self.UserName,
		FirstName:       bot.Self.FirstName,
		DatabaseName:    namespace,
		ClusterID:       clusterID,
		StorageFolder:   folder,

		StorageChannelID:     m.cfg.ArchiveChannelID,
		MaxUsers:             m.cfg.FactoryDefaultMaxUsers,
		MaxFiles:             m.cfg.FactoryDefaultMaxFiles,
		MaxStorageBytes:      m.cfg.FactoryDefaultMaxBytes,
		MaxRequestsPerMinute: m.cfg.FactoryDefaultMaxRequests,
		Status:               models.ManagedBotActive,

		CreatedAt:  now,
		UpdatedAt:  now,
		LastSeenAt: now,
	}
	if _, err := db.Col("managed_bots").InsertOne(ctx, record); err != nil {
		return nil, err
	}
	if clusterID != "" {
		db.SetBotClusterRoute(record.TelegramBotID, clusterID)
	}
	if err := m.startWorker(record, bot); err != nil {

		_, _ = db.Col("managed_bots").DeleteOne(ctx, bson.M{"_id": record.ID})
		db.SetBotClusterRoute(record.TelegramBotID, "")
		return nil, err

	}
	return &record, nil
}

func botNamespace(username string, telegramID int64) (string, string) {
	clean := strings.ToLower(strings.TrimSpace(username))
	if clean == "" {
		clean = "bot"
	}
	var safe strings.Builder
	for _, r := range clean {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			safe.WriteRune(r)
		}
	}
	if safe.Len() == 0 {
		safe.WriteString("bot")
	}
	name := fmt.Sprintf("%s_%d", safe.String(), telegramID)
	return fmt.Sprintf("archive_bot_%d", telegramID), "bots/" + name + "/"
}

// List returns metadata and live metrics, never token material.
func (m *Manager) List(ctx context.Context, ownerID int64, includeAll bool) ([]models.ManagedBot, error) {
	filter := bson.M{}
	if !includeAll {
		filter["owner_id"] = ownerID
	}
	cursor, err := db.Col("managed_bots").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var rows []models.ManagedBot
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	for i := range rows {
		m.applyLiveMetrics(&rows[i])
	}
	return rows, nil
}

// Get returns one record if the caller owns it or is an administrator.
func (m *Manager) Get(ctx context.Context, id string, ownerID int64, includeAll bool) (*models.ManagedBot, error) {
	if !primitive.IsValidObjectID(id) && len(id) < 8 {
		return nil, errors.New("invalid bot id")
	}
	filter := bson.M{"_id": id}
	if !includeAll {
		filter["owner_id"] = ownerID
	}
	var row models.ManagedBot
	if err := db.Col("managed_bots").FindOne(ctx, filter).Decode(&row); err != nil {
		return nil, err
	}
	m.applyLiveMetrics(&row)
	return &row, nil
}

// AssignDelegatedAdmin records the person who receives administration of a child bot.
// It never transfers the encrypted token or Bot Factory ownership.
func (m *Manager) AssignDelegatedAdmin(ctx context.Context, botRecordID string, ownerID, delegatedAdminID int64) (*models.ManagedBot, error) {
	if delegatedAdminID <= 0 {
		return nil, errors.New("delegated admin id is required")
	}
	row, err := m.Get(ctx, botRecordID, ownerID, false)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if _, err := db.Col("managed_bots").UpdateOne(ctx, bson.M{"_id": row.ID, "owner_id": ownerID}, bson.M{"$set": bson.M{"delegated_admin_id": delegatedAdminID, "handed_off_at": now, "updated_at": now}}); err != nil {
		return nil, err
	}
	row.DelegatedAdminID = delegatedAdminID
	row.HandedOffAt = now
	return row, nil
}

// GetByTelegramBotID returns a managed bot by its Telegram identity.
func (m *Manager) GetByTelegramBotID(ctx context.Context, telegramBotID int64) (*models.ManagedBot, error) {
	if telegramBotID <= 0 {
		return nil, ErrNotFound
	}
	var row models.ManagedBot
	if err := db.Col("managed_bots").FindOne(ctx, bson.M{"telegram_bot_id": telegramBotID}).Decode(&row); err != nil {
		return nil, err
	}
	m.applyLiveMetrics(&row)
	return &row, nil
}

// UpdateLimits changes only quota metadata; it never changes the bot token or namespace.
func (m *Manager) UpdateLimits(ctx context.Context, id string, ownerID int64, includeAll bool, maxUsers, maxFiles, maxStorageBytes int64, maxRequestsPerMinute int) (*models.ManagedBot, error) {
	if maxUsers < 0 || maxFiles < 0 || maxStorageBytes < 0 || maxRequestsPerMinute < 0 {
		return nil, errors.New("limits cannot be negative")
	}
	if _, err := m.Get(ctx, id, ownerID, includeAll); err != nil {
		return nil, err
	}
	_, err := db.Col("managed_bots").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
		"max_users": maxUsers, "max_files": maxFiles, "max_storage_bytes": maxStorageBytes, "max_requests_per_minute": maxRequestsPerMinute, "updated_at": time.Now().UTC(),
	}})
	if err != nil {
		return nil, err
	}
	return m.Get(ctx, id, ownerID, includeAll)
}

// RotateToken replaces a bot token while preserving its namespace and metadata.
// Telegram must return the same bot ID; changing identity requires a new registration.
func (m *Manager) RotateToken(ctx context.Context, id string, ownerID int64, includeAll bool, newToken string) (*models.ManagedBot, error) {
	row, err := m.Get(ctx, id, ownerID, includeAll)
	if err != nil {
		return nil, err
	}
	newToken = strings.TrimSpace(newToken)
	if err := validateTokenShape(newToken); err != nil {
		return nil, err
	}
	if TokenHash(newToken) == row.TokenHash {
		return row, nil
	}
	if err := db.Col("managed_bots").FindOne(ctx, bson.M{"token_hash": TokenHash(newToken), "_id": bson.M{"$ne": id}}).Err(); err == nil {
		return nil, errors.New("this bot token is already registered")
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	bot, err := tgbotapi.NewBotAPI(newToken)
	if err != nil {
		return nil, fmt.Errorf("telegram token validation failed: %w", err)
	}
	if bot.Self.ID != row.TelegramBotID {
		return nil, errors.New("new token belongs to a different Telegram bot")
	}
	ciphertext, nonce, err := m.cipher.Encrypt(newToken)
	if err != nil {
		return nil, err
	}
	updated := *row
	updated.TokenCiphertext = ciphertext
	updated.TokenNonce = nonce
	updated.TokenHash = TokenHash(newToken)
	updated.Username = bot.Self.UserName
	updated.FirstName = bot.Self.FirstName
	updated.UpdatedAt = time.Now().UTC()
	m.stopWorker(id)
	if err := m.startWorker(updated, bot); err != nil {
		m.markUnhealthy(ctx, id, err)
		return nil, err
	}
	if _, err := db.Col("managed_bots").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
		"token_ciphertext": ciphertext, "token_nonce": nonce, "token_hash": updated.TokenHash,
		"username": updated.Username, "first_name": updated.FirstName, "updated_at": updated.UpdatedAt,
		"status": models.ManagedBotActive, "last_error": "",
	}}); err != nil {
		return nil, err
	}
	m.applyLiveMetrics(&updated)
	return &updated, nil
}

// Pause stops polling but preserves the encrypted registration.
func (m *Manager) Pause(ctx context.Context, id string, ownerID int64, includeAll bool) error {
	if _, err := m.Get(ctx, id, ownerID, includeAll); err != nil {
		return err
	}
	m.stopWorker(id)
	return m.setStatus(ctx, id, models.ManagedBotPaused, "")
}

// Resume validates the encrypted token and restarts polling.
func (m *Manager) Resume(ctx context.Context, id string, ownerID int64, includeAll bool) error {
	row, err := m.Get(ctx, id, ownerID, includeAll)
	if err != nil {
		return err
	}
	if row.Status == models.ManagedBotActive {
		return nil
	}
	token, err := m.cipher.Decrypt(row.TokenCiphertext, row.TokenNonce)
	if err != nil {
		return err
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		m.markUnhealthy(ctx, id, err)
		return fmt.Errorf("telegram token validation failed: %w", err)
	}
	if err := m.startWorker(*row, bot); err != nil {
		return err
	}
	return m.setStatus(ctx, id, models.ManagedBotActive, "")
}

// Delete permanently removes the registration and stops its worker.
func (m *Manager) Delete(ctx context.Context, id string, ownerID int64, includeAll bool) error {
	if _, err := m.Get(ctx, id, ownerID, includeAll); err != nil {
		return err
	}
	m.stopWorker(id)
	result, err := db.Col("managed_bots").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// Best chooses a healthy bot using a weighted score from latency, errors,
// active work, and queue pressure. Telegram update streams remain isolated by
// token; this router is for factory-managed jobs and outbound work selection.
func (m *Manager) Best(ctx context.Context, ownerID int64, includeAll bool) (*models.ManagedBot, error) {
	rows, err := m.List(ctx, ownerID, includeAll)
	if err != nil {
		return nil, err
	}
	candidates := rows[:0]
	for _, row := range rows {
		if row.Status == models.ManagedBotActive && row.ConsecutiveErrors < maxConsecutiveErrors {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("no healthy active managed bot is available")
	}
	sort.SliceStable(candidates, func(i, j int) bool { return botScore(candidates[i]) > botScore(candidates[j]) })
	return &candidates[0], nil
}

func botScore(row models.ManagedBot) float64 {
	// Lower latency, fewer active requests, and recent health produce a higher score.
	recency := float64(time.Since(row.LastSeenAt).Seconds())
	if row.LastSeenAt.IsZero() || recency < 0 {
		recency = 0
	}
	return 1000 - recency - float64(row.LastLatencyMS)*0.5 - float64(row.ActiveRequests)*10 - float64(row.ConsecutiveErrors*100) - float64(row.TotalErrors)*0.01
}

func (m *Manager) startRecord(ctx context.Context, record models.ManagedBot) error {
	token, err := m.cipher.Decrypt(record.TokenCiphertext, record.TokenNonce)
	if err != nil {
		return err
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}
	if record.DatabaseName == "" || record.StorageFolder == "" || record.StorageChannelID == 0 {
		databaseName, storageFolder := botNamespace(bot.Self.UserName, bot.Self.ID)
		record.DatabaseName = databaseName
		record.StorageFolder = storageFolder
		record.StorageChannelID = m.cfg.ArchiveChannelID
		_, _ = db.Col("managed_bots").UpdateOne(ctx, bson.M{"_id": record.ID}, bson.M{"$set": bson.M{
			"database_name":      databaseName,
			"storage_folder":     storageFolder,
			"storage_channel_id": m.cfg.ArchiveChannelID,
		}})
	}
	return m.startWorker(record, bot)
}

func configureManagedBotCommands(bot *tgbotapi.BotAPI) {
	if bot == nil {
		return
	}
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "🚀 القائمة الرئيسية"},
		{Command: "id", Description: "🆔 عرض Telegram ID"},
		{Command: "group", Description: "⚙️ إعدادات المجموعة"},
		{Command: "subscribe", Description: "🔔 الاشتراك في مادة"},
		{Command: "unsubscribe", Description: "🔕 إلغاء اشتراك مادة"},
		{Command: "subscriptions", Description: "📚 اشتراكاتي"},
		{Command: "vault", Description: "🔐 Personal Vault"},
		{Command: "vaultadd", Description: "➕ حفظ ملف في Vault"},
	}
	if _, err := bot.Request(tgbotapi.NewSetMyCommands(commands...)); err != nil {
		log.Printf("managed bot %d command menu setup failed: %v", bot.Self.ID, err)
	}
}

func (m *Manager) startWorker(record models.ManagedBot, bot *tgbotapi.BotAPI) error {
	if record.ClusterID != "" {
		db.SetBotClusterRoute(record.TelegramBotID, record.ClusterID)
	}
	worker := &managedWorker{bot: bot, stop: make(chan struct{}), stats: &workerStats{}, closed: make(chan struct{})}
	worker.stats.lastSeenUnix.Store(time.Now().Unix())
	m.mu.Lock()
	if old := m.workers[record.ID]; old != nil {
		m.mu.Unlock()
		return errors.New("managed bot worker already running")
	}
	m.workers[record.ID] = worker
	m.mu.Unlock()

	go db.EnsureIndexesForContext(db.WithBotDatabase(context.Background(), record.TelegramBotID))
	go m.poll(record.ID, worker)
	return nil
}

func (m *Manager) poll(id string, worker *managedWorker) {
	defer close(worker.closed)
	leaseID := managedLeaseID(worker.bot.Self.ID)
	log.Printf("managed bot %s waiting for polling lease", id)
	if !m.waitForWorkerLease(context.Background(), leaseID, worker.bot.Self.ID, worker.stop) {
		m.removeWorker(id, worker)
		return
	}
	log.Printf("managed bot %s acquired polling lease", id)
	defer m.releaseWorkerLease(context.Background(), leaseID)

	if _, err := worker.bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		log.Printf("managed bot %d webhook cleanup failed: %v", worker.bot.Self.ID, err)
	}
	configureManagedBotCommands(worker.bot)
	go m.health(id, worker)
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := worker.bot.GetUpdatesChan(updateConfig)
	leaseLost := make(chan struct{})
	go func() {
		ticker := time.NewTicker(workerLeaseRenewal)
		defer ticker.Stop()
		for {
			select {
			case <-worker.stop:
				return
			case <-ticker.C:
				renewCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				ok := m.renewWorkerLease(renewCtx, leaseID)

				cancel()
				if !ok {
					close(leaseLost)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-worker.stop:
			return
		case <-leaseLost:
			log.Printf("managed bot %s polling lease lost; stopping worker", id)
			worker.shutdown()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			started := time.Now()
			worker.stats.activeRequests.Add(1)
			worker.stats.totalUpdates.Add(1)
			worker.stats.lastSeenUnix.Store(time.Now().Unix())
			if m.handler != nil {
				func() {
					defer func() {
						if recovered := recover(); recovered != nil {
							worker.stats.totalErrors.Add(1)
							worker.stats.consecutive.Add(1)
							log.Printf("managed bot %s update panic: %v", id, recovered)
						}
					}()
					m.handler(worker.bot, update)
				}()
			}
			worker.stats.lastLatencyNS.Store(time.Since(started).Nanoseconds())
			worker.stats.activeRequests.Add(-1)
		}
	}
}

func (m *Manager) health(id string, worker *managedWorker) {
	ticker := time.NewTicker(healthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-worker.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := worker.bot.GetMe()
			if err != nil {
				consecutive := worker.stats.consecutive.Add(1)
				worker.stats.totalErrors.Add(1)
				if consecutive >= maxConsecutiveErrors {
					m.markUnhealthy(ctx, id, err)
				}
				log.Printf("managed bot %s health check failed (%d): %v", id, consecutive, err)
				cancel()
				continue
			}
			worker.stats.consecutive.Store(0)
			worker.stats.lastSeenUnix.Store(time.Now().Unix())
			m.persistHealth(ctx, id, worker)
			cancel()
		}
	}
}

func (worker *managedWorker) shutdown() {
	if worker == nil {
		return
	}
	worker.once.Do(func() {
		close(worker.stop)
		worker.stopUpdates()
	})
}

func (worker *managedWorker) stopUpdates() {
	if worker == nil || worker.bot == nil {
		return
	}
	worker.updatesOnce.Do(func() { worker.bot.StopReceivingUpdates() })
}

func (m *Manager) removeWorker(id string, target *managedWorker) {
	m.mu.Lock()
	if m.workers[id] == target {
		delete(m.workers, id)
	}
	m.mu.Unlock()
}

func (m *Manager) stopWorker(id string) {
	m.mu.Lock()
	worker := m.workers[id]
	delete(m.workers, id)
	m.mu.Unlock()
	if worker == nil {
		return
	}
	worker.shutdown()
	select {
	case <-worker.closed:
	case <-time.After(5 * time.Second):
		log.Printf("managed bot %s worker did not stop within timeout", id)
	}
}

func (m *Manager) persistHealth(ctx context.Context, id string, worker *managedWorker) {
	_, err := db.Col("managed_bots").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
		"status":             models.ManagedBotActive,
		"updated_at":         time.Now().UTC(),
		"last_seen_at":       time.Unix(worker.stats.lastSeenUnix.Load(), 0).UTC(),
		"consecutive_errors": worker.stats.consecutive.Load(),
		"total_updates":      worker.stats.totalUpdates.Load(),
		"total_errors":       worker.stats.totalErrors.Load(),
		"last_error":         "",
	}})
	if err != nil {
		log.Printf("managed bot %s health persist failed: %v", id, err)
	}
}

func (m *Manager) applyLiveMetrics(row *models.ManagedBot) {
	m.mu.RLock()
	worker := m.workers[row.ID]
	m.mu.RUnlock()
	if worker == nil {
		return
	}
	row.Status = models.ManagedBotActive
	row.ConsecutiveErrors = int(worker.stats.consecutive.Load())
	if row.ConsecutiveErrors >= maxConsecutiveErrors {
		row.Status = models.ManagedBotUnhealthy
	}
	row.LastError = ""
	row.ActiveRequests = worker.stats.activeRequests.Load()
	row.LastLatencyMS = worker.stats.lastLatencyNS.Load() / int64(time.Millisecond)
	row.TotalUpdates = worker.stats.totalUpdates.Load()
	row.TotalErrors = worker.stats.totalErrors.Load()
	row.LastSeenAt = time.Unix(worker.stats.lastSeenUnix.Load(), 0).UTC()
}

func (m *Manager) setStatus(ctx context.Context, id, status, lastError string) error {
	set := bson.M{"status": status, "updated_at": time.Now().UTC()}
	if lastError == "" {
		set["last_error"] = ""
	} else {
		set["last_error"] = safeError(lastError)
	}
	_, err := db.Col("managed_bots").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

func (m *Manager) markUnhealthy(ctx context.Context, id string, err error) {
	message := "managed bot health check failed"
	if err != nil {
		message = err.Error()
	}
	_ = m.setStatus(ctx, id, models.ManagedBotUnhealthy, message)
}

func safeError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func newID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return primitive.NewObjectID().Hex()
	}
	return hex.EncodeToString(buf[:])
}

// SendText routes an outbound message to the healthiest managed bot and
// updates the worker's live load signals. It is intentionally explicit: it
// does not move Telegram polling updates between bot tokens.
func (m *Manager) SendText(ctx context.Context, ownerID, chatID int64, text string, includeAll bool) (*models.ManagedBot, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("message text is empty")
	}
	row, err := m.Best(ctx, ownerID, includeAll)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	worker := m.workers[row.ID]
	m.mu.RUnlock()
	if worker == nil {
		return nil, errors.New("selected managed bot worker is not running")
	}
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	started := time.Now()
	worker.stats.activeRequests.Add(1)
	_, sendErr := worker.bot.Send(tgbotapi.NewMessage(chatID, text))
	worker.stats.lastLatencyNS.Store(time.Since(started).Nanoseconds())
	worker.stats.activeRequests.Add(-1)
	if sendErr != nil {
		worker.stats.totalErrors.Add(1)
		worker.stats.consecutive.Add(1)
		return nil, sendErr
	}
	worker.stats.consecutive.Store(0)
	worker.stats.lastSeenUnix.Store(time.Now().Unix())
	return row, nil
}

// Close stops all managed workers during graceful shutdown.
func (m *Manager) Close() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.workers))
	for id := range m.workers {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		m.stopWorker(id)
	}
}
