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
	Type  string // "new_cat", "new_sub", "broadcast", "add_admin", "welcome_text", "welcome_photo", "upload"
	CatID int    // used for "new_sub"
}
