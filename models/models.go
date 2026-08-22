package models

import (
	"sync"
	"time"
)

// User represents a Telegram user in the database.
type User struct {
	UserID     int64     `bson:"user_id"`
	Username   string    `bson:"username,omitempty"`
	FirstName  string    `bson:"first_name,omitempty"`
	IsBanned   bool      `bson:"is_banned"`
	IsMuted    bool      `bson:"is_muted"`
	CreatedAt  time.Time `bson:"created_at"`
	LastSeenAt time.Time `bson:"last_seen_at"`
}

// Category represents a top-level category (e.g., "محاضرات PDF").
type Category struct {
	CatID int    `bson:"cat_id"`
	Name  string `bson:"name"`
	Order int    `bson:"order"`
}

// Subject represents a subject within a category.
type Subject struct {
	SubID      int    `bson:"sub_id"`
	Name       string `bson:"name"`
	CategoryID int    `bson:"category_id"`
	Order      int    `bson:"order"`
}

// File represents an archived file.
type File struct {
	FileID         int       `bson:"file_id"`
	Name           string    `bson:"name"`
	TelegramFileID string    `bson:"telegram_file_id"`
	FileType       string    `bson:"file_type"`
	SubjectID      int       `bson:"subject_id"`
	MessageID      int       `bson:"message_id,omitempty"`
	FileSize       int64     `bson:"file_size,omitempty"`
	Order          int       `bson:"order"`
	Downloads      int       `bson:"downloads"`
	UploadDate     time.Time `bson:"upload_date"`
}

// FileRow is an enriched File with subject name for display.
type FileRow struct {
	File
	SubjectName string
}

// Admin represents a bot administrator.
type Admin struct {
	ID        int64     `bson:"id"`
	Username  string    `bson:"username,omitempty"`
	FirstName string    `bson:"first_name,omitempty"`
	Rank      string    `bson:"rank"`
	AddedAt   time.Time `bson:"added_at"`
}

// AdminLog represents an entry in the admin activity log.
type AdminLog struct {
	AdminID   int64                  `bson:"admin_id"`
	Action    string                 `bson:"action"`
	Details   map[string]interface{} `bson:"details"`
	Timestamp time.Time              `bson:"timestamp"`
}

// SharedFile represents a file shared via deep link.
type SharedFile struct {
	ShareHash      string    `bson:"share_hash"`
	TelegramFileID string    `bson:"telegram_file_id"`
	ExpiresAt      time.Time `bson:"expires_at"`
	CreatedBy      int64     `bson:"created_by"`
}

// ManagedBot represents a user-owned bot registered in the Bot Factory.
type ManagedBot struct {
	ID                   string    `bson:"_id" json:"id"`
	OwnerID              int64     `bson:"owner_id" json:"owner_id"`
	DelegatedAdminID     int64     `bson:"delegated_admin_id,omitempty" json:"delegated_admin_id,omitempty"`
	HandedOffAt          time.Time `bson:"handed_off_at,omitempty" json:"handed_off_at,omitempty"`
	TokenCiphertext      string    `bson:"token_ciphertext" json:"-"`
	TokenNonce           string    `bson:"token_nonce" json:"-"`
	TokenHash            string    `bson:"token_hash" json:"-"`
	TelegramBotID        int64     `bson:"telegram_bot_id" json:"telegram_bot_id"`
	Username             string    `bson:"username" json:"username"`
	FirstName            string    `bson:"first_name,omitempty" json:"first_name,omitempty"`
	DatabaseName         string    `bson:"database_name" json:"database_name"`
	ClusterID            string    `bson:"cluster_id,omitempty" json:"cluster_id,omitempty"`
	StorageFolder        string    `bson:"storage_folder" json:"storage_folder"`
	StorageChannelID     int64     `bson:"storage_channel_id" json:"storage_channel_id"`
	MaxUsers             int64     `bson:"max_users" json:"max_users"`
	MaxFiles             int64     `bson:"max_files" json:"max_files"`
	MaxStorageBytes      int64     `bson:"max_storage_bytes" json:"max_storage_bytes"`
	MaxRequestsPerMinute int       `bson:"max_requests_per_minute" json:"max_requests_per_minute"`
	Status               string    `bson:"status" json:"status"`
	CreatedAt            time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt            time.Time `bson:"updated_at" json:"updated_at"`
	LastSeenAt           time.Time `bson:"last_seen_at,omitempty" json:"last_seen_at,omitempty"`
	LastError            string    `bson:"last_error,omitempty" json:"last_error,omitempty"`
	ConsecutiveErrors    int       `bson:"consecutive_errors" json:"consecutive_errors"`
	TotalUpdates         int64     `bson:"total_updates" json:"total_updates"`
	TotalErrors          int64     `bson:"total_errors" json:"total_errors"`
	ActiveRequests       int64     `bson:"-" json:"active_requests"`
	LastLatencyMS        int64     `bson:"-" json:"last_latency_ms"`
}

const (
	ManagedBotActive    = "active"
	ManagedBotPaused    = "paused"
	ManagedBotUnhealthy = "unhealthy"
)

// GroupConfig stores lightweight settings for one Telegram group inside one bot namespace.
type GroupConfig struct {
	ID         string    `bson:"_id" json:"id"`
	BotID      int64     `bson:"bot_id" json:"bot_id"`
	ChatID     int64     `bson:"chat_id" json:"chat_id"`
	Title      string    `bson:"title,omitempty" json:"title,omitempty"`
	Enabled    bool      `bson:"enabled" json:"enabled"`
	AdminsOnly bool      `bson:"admins_only" json:"admins_only"`
	CreatedAt  time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at" json:"updated_at"`
}

// SubjectSubscription tracks a user's subscription to a subject inside one bot namespace.
type SubjectSubscription struct {
	ID        string    `bson:"_id" json:"id"`
	BotID     int64     `bson:"bot_id" json:"bot_id"`
	UserID    int64     `bson:"user_id" json:"user_id"`
	SubjectID int       `bson:"subject_id" json:"subject_id"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

// VaultItem stores Telegram metadata only; file bytes remain in Telegram storage.
type VaultItem struct {
	ID      string    `bson:"_id" json:"id"`
	BotID   int64     `bson:"bot_id" json:"bot_id"`
	UserID  int64     `bson:"user_id" json:"user_id"`
	FileID  int       `bson:"file_id" json:"file_id"`
	AddedAt time.Time `bson:"added_at" json:"added_at"`
}

// SubjectNotification prevents duplicate notifications for one user and file.
type SubjectNotification struct {
	ID        string    `bson:"_id" json:"id"`
	BotID     int64     `bson:"bot_id" json:"bot_id"`
	UserID    int64     `bson:"user_id" json:"user_id"`
	SubjectID int       `bson:"subject_id" json:"subject_id"`
	FileID    int       `bson:"file_id" json:"file_id"`
	SentAt    time.Time `bson:"sent_at" json:"sent_at"`
}

// StorageCluster represents a separately reachable MongoDB cluster registered by the parent bot.
type StorageCluster struct {
	ID             string    `bson:"_id" json:"id"`
	Name           string    `bson:"name" json:"name"`
	URICiphertext  string    `bson:"uri_ciphertext" json:"-"`
	URINonce       string    `bson:"uri_nonce" json:"-"`
	DatabasePrefix string    `bson:"database_prefix" json:"database_prefix"`
	Status         string    `bson:"status" json:"status"`
	LastHealthAt   time.Time `bson:"last_health_at,omitempty" json:"last_health_at,omitempty"`
	LastError      string    `bson:"last_error,omitempty" json:"last_error,omitempty"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at" json:"updated_at"`
}

const (
	StorageClusterActive   = "active"
	StorageClusterDraining = "draining"
	StorageClusterOffline  = "offline"
)

// BotShardRoute maps a virtual shard in one bot namespace to a separate cluster.
type BotShardRoute struct {
	ID           string    `bson:"_id" json:"id"`
	BotID        int64     `bson:"bot_id" json:"bot_id"`
	VirtualShard int       `bson:"virtual_shard" json:"virtual_shard"`
	ClusterID    string    `bson:"cluster_id" json:"cluster_id"`
	DatabaseName string    `bson:"database_name" json:"database_name"`
	State        string    `bson:"state" json:"state"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}

const (
	ShardRouteActive    = "active"
	ShardRouteMigrating = "migrating"
	ShardRouteDraining  = "draining"
)

// ExpansionState tracks the parent controller's automatic database expansion for one bot.
type ExpansionState struct {
	BotID           int64     `bson:"_id" json:"bot_id"`
	DatabaseName    string    `bson:"database_name" json:"database_name"`
	SchemaVersion   int       `bson:"schema_version" json:"schema_version"`
	Status          string    `bson:"status" json:"status"`
	UsersCount      int64     `bson:"users_count" json:"users_count"`
	FilesCount      int64     `bson:"files_count" json:"files_count"`
	StorageBytes    int64     `bson:"storage_bytes" json:"storage_bytes"`
	CapacityTier    string    `bson:"capacity_tier" json:"capacity_tier"`
	ExpansionCount  int64     `bson:"expansion_count" json:"expansion_count"`
	LastRunAt       time.Time `bson:"last_run_at,omitempty" json:"last_run_at,omitempty"`
	LastCompletedAt time.Time `bson:"last_completed_at,omitempty" json:"last_completed_at,omitempty"`
	NextEligibleAt  time.Time `bson:"next_eligible_at,omitempty" json:"next_eligible_at,omitempty"`
	LockOwner       string    `bson:"lock_owner,omitempty" json:"-"`
	LockUntil       time.Time `bson:"lock_until,omitempty" json:"-"`
	LastError       string    `bson:"last_error,omitempty" json:"last_error,omitempty"`
}

const (
	ExpansionReady    = "ready"
	ExpansionRunning  = "running"
	ExpansionComplete = "completed"
	ExpansionFailed   = "failed"
)

// AIIndex stores AI-generated metadata for a file inside one bot namespace.
type AIIndex struct {
	FileID      int       `bson:"file_id" json:"file_id"`
	Summary     string    `bson:"summary" json:"summary"`
	Tags        []string  `bson:"tags" json:"tags"`
	ContentHash string    `bson:"content_hash" json:"content_hash"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}

// BotBackup records a snapshot of one isolated bot database.
type BotBackup struct {
	ID          string         `bson:"_id" json:"id"`
	BotID       int64          `bson:"bot_id" json:"bot_id"`
	Status      string         `bson:"status" json:"status"`
	Collections map[string]int `bson:"collections" json:"collections"`
	CreatedAt   time.Time      `bson:"created_at" json:"created_at"`
	CompletedAt *time.Time     `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	Error       string         `bson:"error,omitempty" json:"error,omitempty"`
}

const (
	BackupPending   = "pending"
	BackupCompleted = "completed"
	BackupFailed    = "failed"
)

// APIKey stores only a hash of a bot-scoped API credential.
type APIKey struct {
	ID          string     `bson:"_id" json:"id"`
	BotID       int64      `bson:"bot_id" json:"bot_id"`
	Name        string     `bson:"name" json:"name"`
	KeyHash     string     `bson:"key_hash" json:"-"`
	Prefix      string     `bson:"prefix" json:"prefix"`
	Permissions []string   `bson:"permissions" json:"permissions"`
	CreatedAt   time.Time  `bson:"created_at" json:"created_at"`
	RevokedAt   *time.Time `bson:"revoked_at,omitempty" json:"revoked_at,omitempty"`
}

// StorageJob represents a durable delivery request handled by the primary Storage Gateway.
type StorageJob struct {
	ID             string    `bson:"_id" json:"id"`
	BotID          int64     `bson:"bot_id" json:"bot_id"`
	ChatID         int64     `bson:"chat_id" json:"chat_id"`
	TelegramFileID string    `bson:"telegram_file_id" json:"-"`
	FileType       string    `bson:"file_type" json:"file_type"`
	Caption        string    `bson:"caption,omitempty" json:"caption,omitempty"`
	Status         string    `bson:"status" json:"status"`
	Attempts       int       `bson:"attempts" json:"attempts"`
	NextAttemptAt  time.Time `bson:"next_attempt_at" json:"next_attempt_at"`
	LastError      string    `bson:"last_error,omitempty" json:"last_error,omitempty"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at" json:"updated_at"`
	CompletedAt    time.Time `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
}

const (
	StorageJobPending    = "pending"
	StorageJobProcessing = "processing"
	StorageJobSent       = "sent"
	StorageJobRetrying   = "retrying"
	StorageJobDead       = "dead"
)

// BotSetting represents a key-value setting stored in the database.
type BotSetting struct {
	Key   string `bson:"key"`
	Value string `bson:"value"`
}

// FileRating represents a user's rating for a file.
type FileRating struct {
	UserID int64 `bson:"user_id"`
	FileID int   `bson:"file_id"`
	Stars  int   `bson:"stars"`
}

// Counter represents a sequence counter for auto-incrementing IDs.
type Counter struct {
	ID  string `bson:"_id"`
	Seq int    `bson:"seq"`
}

// WelcomeSettings holds the welcome message and photo configuration.
type WelcomeSettings struct {
	Message string
	Photo   string
}

// PendingUpload holds info about a file being uploaded by an admin.
type PendingUpload struct {
	TelegramFileID string
	Name           string
	FileType       string
	ChatID         int64
	MessageID      int
	FileSize       int64
}

// UploadLocation holds the selected category/subject for an upload.
type UploadLocation struct {
	CatID int
	SubID int
}

// UserState holds the per-user conversation state (replaces context.user_data).
type UserState struct {
	Mu                sync.Mutex `bson:"-" json:"-"`
	Awaiting          *AwaitingState
	PendingUploads    []PendingUpload
	UploadLoc         *UploadLocation
	UsersPage         int
	UploadTimerActive bool
}

// AwaitingState tracks what the bot is waiting for from a user.
type AwaitingState struct {
	Type  string // workflow identifier
	CatID int    // used for "new_sub"
	Value string // short, non-secret workflow value such as a cluster name
}
