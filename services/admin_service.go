package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

// ── Rank system ─────────────────────────────────────────────

// RankLevels maps rank names to their numeric level.
var RankLevels = map[string]int{
	"owner":       4,
	"super_admin": 3,
	"admin":       2,
	"editor":      1,
	"moderator":   1,
	"viewer":      0,
}

// RankLabels maps rank names to their Arabic display labels.
var RankLabels = map[string]string{
	"owner":       "👑 المالك",
	"super_admin": "🥇 أدمن رئيسي",
	"admin":       "🥈 أدمن",
	"moderator":   "🥉 مشرف",
	"editor":      "📝 محرر",
	"viewer":      "👁️ مشاهد",
}

// RankPermissions maps each rank to its allowed permissions.
var RankPermissions = map[string]map[string]bool{
	"owner": {
		"view_stats": true, "manage_content": true, "manage_users": true,
		"broadcast": true, "manage_admins": true, "manage_settings": true,
		"view_logs": true, "manage_maintenance": true,
		"manage_bots": true, "view_bot_metrics": true, "manage_limits": true, "manage_api_keys": true, "manage_backups": true,
	},
	"super_admin": {
		"view_stats": true, "manage_content": true, "manage_users": true,
		"broadcast": true, "manage_admins": true, "manage_settings": true,
		"view_logs": true, "manage_maintenance": true,
		"manage_bots": true, "view_bot_metrics": true, "manage_limits": true, "manage_api_keys": true, "manage_backups": true,
	},
	"admin": {
		"view_stats": true, "manage_content": true, "manage_users": true,
		"broadcast": true, "view_logs": true,
	},
	"moderator": {
		"view_stats": true, "manage_content": true,
	},
	"editor": {
		"view_stats": true, "manage_content": true,
	},
	"viewer": {
		"view_stats": true,
	},
}

// ActionLabels maps action keys to Arabic display labels for the admin log.
var ActionLabels = map[string]string{
	"ban_user":             "⛔ حظر مستخدم",
	"unban_user":           "✅ إلغاء حظر",
	"mute_user":            "🔇 كتم مستخدم",
	"unmute_user":          "🔊 إلغاء كتم",
	"add_admin":            "👑 إضافة أدمن",
	"remove_admin":         "🤝 إزالة أدمن",
	"set_rank":             "🎖️ تغيير رتبة",
	"toggle_maintenance":   "🔧 تبديل الصيانة",
	"broadcast":            "📢 إذاعة",
	"create_category":      "📁 إنشاء تصنيف",
	"create_subject":       "📂 إنشاء مادة",
	"upload_files":         "📤 رفع ملفات",
	"delete_category":      "🗑️ حذف تصنيف",
	"delete_subject":       "🗑️ حذف مادة",
	"delete_file":          "🗑️ حذف ملف",
	"move_category":        "↕️ نقل تصنيف",
	"move_subject":         "↕️ نقل مادة",
	"move_file":            "↕️ نقل ملف",
	"set_welcome":          "🎨 تعديل الترحيب",
	"update_bot_limits":    "📏 تعديل حدود البوت",
	"rotate_bot_token":     "🔐 تدوير توكن البوت",
	"create_api_key":       "🔑 إنشاء مفتاح API",
	"create_backup":        "💾 إنشاء نسخة احتياطية",
	"register_managed_bot": "🤖 تسجيل بوت مُدار",
	"pause_managed_bot":    "⏸️ إيقاف بوت مُدار",
	"resume_managed_bot":   "▶️ تشغيل بوت مُدار",
	"delete_managed_bot":   "🗑️ حذف بوت مُدار",
}

// ── Admins cache ────────────────────────────────────────────

var (
	adminsCache   []models.Admin
	adminsCacheTS time.Time
	adminsMu      sync.RWMutex

	maintCache   *bool
	maintCacheTS time.Time
	maintMu      sync.RWMutex
)

func adminsCacheValid() bool {
	return time.Since(adminsCacheTS) < time.Duration(config.Cfg.CacheTTLSeconds)*time.Second
}

// InvalidateAdmins clears the admin cache.
func InvalidateAdmins() {
	adminsMu.Lock()
	adminsCache = nil
	adminsCacheTS = time.Time{}
	adminsMu.Unlock()
}

// GetAdmins returns all admin records, with caching.
func GetAdmins(ctx context.Context) ([]models.Admin, error) {
	useCache := !db.IsScoped(ctx)
	if useCache {
		adminsMu.RLock()
		if adminsCache != nil && adminsCacheValid() {
			defer adminsMu.RUnlock()
			return adminsCache, nil
		}
		adminsMu.RUnlock()

		adminsMu.Lock()
		defer adminsMu.Unlock()

		// Double-check after acquiring write lock
		if adminsCache != nil && adminsCacheValid() {
			return adminsCache, nil
		}
	}

	opts := options.Find().SetSort(bson.D{{Key: "added_at", Value: 1}})
	cursor, err := db.ColScoped(ctx, "admins").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var rows []models.Admin
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []models.Admin{}
	}
	if useCache {
		adminsCache = rows
		adminsCacheTS = time.Now()
	}
	return rows, nil
}

// ── Rank helpers ────────────────────────────────────────────

// RankLabel returns the display label for a rank.
func RankLabel(rank string) string {
	if l, ok := RankLabels[rank]; ok {
		return l
	}
	return "👤 مستخدم عادي"
}

// RankLevelValue returns the numeric level for a rank.
func RankLevelValue(rank string) int {
	return RankLevels[rank] // returns 0 if not found
}

// GetUserRank returns the rank of a user (or "" if not admin).
func GetUserRank(ctx context.Context, uid int64) (string, error) {
	if uid == config.Cfg.OwnerID {
		return "owner", nil
	}
	admins, err := GetAdmins(ctx)
	if err != nil {
		return "", err
	}
	for _, a := range admins {
		if a.ID == uid {
			if a.Rank == "" {
				return "admin", nil
			}
			return a.Rank, nil
		}
	}
	return "", nil
}

// GetUserPermissions returns the set of permissions for a user.
func GetUserPermissions(ctx context.Context, uid int64) (map[string]bool, error) {
	rank, err := GetUserRank(ctx, uid)
	if err != nil {
		return nil, err
	}
	perms := RankPermissions[rank]
	if perms == nil {
		return map[string]bool{}, nil
	}
	return perms, nil
}

// Can checks if a user has a specific permission.
func Can(ctx context.Context, uid int64, permission string) (bool, error) {
	perms, err := GetUserPermissions(ctx, uid)
	if err != nil {
		return false, err
	}
	return perms[permission], nil
}

// CanManage checks if actor can manage target (higher rank).
func CanManage(ctx context.Context, actorUID, targetUID int64) (bool, error) {
	actorRank, err := GetUserRank(ctx, actorUID)
	if err != nil {
		return false, err
	}
	targetRank, err := GetUserRank(ctx, targetUID)
	if err != nil {
		return false, err
	}
	return RankLevelValue(actorRank) > RankLevelValue(targetRank), nil
}

// IsAdmin checks if a user is an admin.
func IsAdmin(ctx context.Context, uid int64) (bool, error) {
	if uid == config.Cfg.OwnerID {
		return true, nil
	}
	admins, err := GetAdmins(ctx)
	if err != nil {
		return false, err
	}
	for _, a := range admins {
		if a.ID == uid {
			return true, nil
		}
	}
	return false, nil
}

// IsOwner checks if a user is the bot owner.
func IsOwner(uid int64) bool {
	return uid == config.Cfg.OwnerID
}

// AddAdmin adds or updates an admin.
func AddAdmin(ctx context.Context, userID int64, username, firstName, rank string) error {
	if _, ok := RankLevels[rank]; !ok {
		return fmt.Errorf("invalid admin rank: %s", rank)
	}
	if firstName == "" {
		firstName = fmt.Sprintf("ID:%d", userID)
	}
	opts := options.Update().SetUpsert(true)
	_, err := db.ColScoped(ctx, "admins").UpdateOne(ctx,
		bson.M{"id": userID},
		bson.M{"$set": bson.M{
			"id":         userID,
			"username":   username,
			"first_name": firstName,
			"rank":       rank,
			"added_at":   time.Now().UTC(),
		}},
		opts,
	)
	InvalidateAdmins()
	return err
}

// SetAdminRank changes an admin's rank or removes them.
func SetAdminRank(ctx context.Context, userID int64, rank string, username, firstName string) error {
	if rank == "" {
		_, err := db.ColScoped(ctx, "admins").DeleteOne(ctx, bson.M{"id": userID})
		InvalidateAdmins()
		return err
	}
	if _, ok := RankLevels[rank]; !ok {
		return fmt.Errorf("invalid admin rank: %s", rank)
	}
	doc := bson.M{"rank": rank}
	if username != "" {
		doc["username"] = username
	}
	if firstName != "" {
		doc["first_name"] = firstName
	}
	opts := options.Update().SetUpsert(true)
	_, err := db.ColScoped(ctx, "admins").UpdateOne(ctx,
		bson.M{"id": userID},
		bson.M{"$set": doc, "$setOnInsert": bson.M{"id": userID}},
		opts,
	)
	InvalidateAdmins()
	return err
}

// RemoveAdmin removes an admin.
func RemoveAdmin(ctx context.Context, userID int64) error {
	_, err := db.ColScoped(ctx, "admins").DeleteOne(ctx, bson.M{"id": userID})
	InvalidateAdmins()
	return err
}

// ── Admin logs ──────────────────────────────────────────────

// GetAdminLogs returns paginated admin activity logs.
func GetAdminLogs(ctx context.Context, page, perPage int) ([]models.AdminLog, error) {
	skip := int64((page - 1) * perPage)
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(perPage))
	cursor, err := db.ColScoped(ctx, "admin_logs").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var rows []models.AdminLog
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// GetAdminLogsCount returns the total number of admin log entries.
func GetAdminLogsCount(ctx context.Context) (int64, error) {
	return db.ColScoped(ctx, "admin_logs").CountDocuments(ctx, bson.M{})
}

// ── Maintenance mode ────────────────────────────────────────

// IsMaintenanceEnabled checks if maintenance mode is on.
func IsMaintenanceEnabled(ctx context.Context) bool {
	useCache := !db.IsScoped(ctx)
	if useCache {
		maintMu.RLock()
		if maintCache != nil && time.Since(maintCacheTS) < time.Duration(config.Cfg.CacheTTLSeconds)*time.Second {
			val := *maintCache
			maintMu.RUnlock()
			return val
		}
		maintMu.RUnlock()

		maintMu.Lock()
		defer maintMu.Unlock()
	}

	var row models.BotSetting
	err := db.ColScoped(ctx, "bot_settings").FindOne(ctx, bson.M{"key": "maintenance_mode"}).Decode(&row)
	if err != nil {
		val := false
		if useCache {
			maintCache = &val
			maintCacheTS = time.Now()
		}
		return false

	}
	val := row.Value == "true"
	if useCache {
		maintCache = &val
		maintCacheTS = time.Now()
	}
	return val
}

// SetMaintenanceMode enables or disables maintenance mode.
func SetMaintenanceMode(ctx context.Context, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	opts := options.Update().SetUpsert(true)
	_, err := db.ColScoped(ctx, "bot_settings").UpdateOne(ctx,
		bson.M{"key": "maintenance_mode"},
		bson.M{"$set": bson.M{"value": val}},
		opts,
	)
	// Scoped child settings must never mutate the primary process-wide cache.
	// Also keep the cache unchanged when the database write fails.
	if err == nil && !db.IsScoped(ctx) {
		maintMu.Lock()
		value := enabled
		maintCache = &value
		maintCacheTS = time.Now()
		maintMu.Unlock()
	}
	return err
}

// LogAdminAction writes an entry to the admin activity log.
func LogAdminAction(ctx context.Context, adminID int64, action string, details map[string]interface{}) {
	if details == nil {
		details = map[string]interface{}{}
	}
	_, err := db.ColScoped(ctx, "admin_logs").InsertOne(ctx, bson.M{
		"admin_id":  adminID,
		"action":    action,
		"details":   details,
		"timestamp": time.Now().UTC(),
	})
	if err != nil {
		log.Printf("Failed to log admin action: %v", err)
	}
}
