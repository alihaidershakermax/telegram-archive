package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all bot configuration loaded from environment variables.
type Config struct {
	BotToken                   string
	OwnerID                    int64
	ArchiveChannelID           int64
	MongoURI                   string
	DBName                     string
	BroadcastChannelID         int64
	BroadcastDelay             float64
	CacheTTLSeconds            int
	WelcomePhoto               string
	APIKey                     string
	APIRateLimit               int
	AIBaseURL                  string
	AIAPIKey                   string
	AIModel                    string
	AIRequestTimeoutSeconds    int
	FactoryEncryptionKey       string
	FactoryMaxBotsPerOwner     int
	FactoryWorkers             int
	StorageQueuePollSeconds    int
	StorageMaxAttempts         int
	StorageRetryBaseSeconds    int
	StorageQueueBatchSize      int
	FactoryDefaultMaxUsers     int64
	FactoryDefaultMaxFiles     int64
	FactoryDefaultMaxBytes     int64
	FactoryDefaultMaxRequests  int
	DBAutoExpansion            bool
	DBExpansionPollSeconds     int
	DBExpansionBatchSize       int
	DBExpansionMaxDocs         int64
	DBExpansionMaxBytes        int64
	DBExpansionLockSeconds     int
	DBExpansionCooldownSeconds int
}

// Text constants
const (
	Developer         = "علي الأكبر حيدر شاكر"
	DeveloperUsername = "@dextermorgenk"
	AboutPhoto        = "https://c.top4top.io/p_38705xdnk1.jpg"

	WelcomeMessage = "🎓 منصة الأرشيف التعليمي الذكي\n\n" +
		"مرحباً بك! 👋\n\n" +
		"الأرشيف التعليمي هو رفيقك الرقمي على تليجرام، المصمم خصيصاً " +
		"لتبسيط الوصول إلى المصادر العلمية والملفات الأكاديمية بكل كفاءة وسرعة.\n\n" +
		"💡 ما نقدمه:\n" +
		"📂 تنظيم ذكي للملفات\n" +
		"⭐ إضافة ملفاتك المفضلة\n\n"

	AboutDevText = "السلام عليكم ورحمة الله وبركاته 👋\n\n" +
		"🤖 فكرة هذا البوت:\n" +
		"منصة تعليمية ذكية تنظّم ملفاتك الأكاديمية (محاضرات، ملخصات، فيديوهات، كتب) " +
		"في أقسام ومواد مرتبة، لتصل إليها بضغطة زر وفي أي وقت.\n\n" +
		"✨ ماذا يقدم لك؟\n" +
		"📂 أقسام ومواد منظمة ومرتبة\n" +
		"⬇️ تحميل مباشر أو عرض PDF كصور\n" +
		"🔗 مشاركة الملفات بروابط مباشرة\n\n" +
		"👨‍💻 المطور: " + Developer + "\n" +
		"📱 التواصل: " + DeveloperUsername + "\n\n" +
		"شكراً لثقتكم بنا، ونتمنى أن يكون هذا العمل إضافة حقيقية لرحلتكم التعليمية."
)

// FixedCategoryNames are the default categories created on first run.
var FixedCategoryNames = []string{
	"محاضرات PDF",
	"ملخصات PDF",
	"محاضرات فيديوية",
	"محاضرات مترجمة",
}

// FileTypeIcons maps file types to emoji icons.
var FileTypeIcons = map[string]string{
	"document":   "📄",
	"video":      "🎥",
	"audio":      "🎵",
	"photo":      "📷",
	"voice":      "🎤",
	"animation":  "🖼",
	"video_note": "🎬",
	"sticker":    "🏷",
}

// Cfg is the global configuration instance.
var Cfg *Config

// Load reads environment variables and populates the global Cfg.
func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system env vars")
	}

	Cfg = &Config{
		BotToken:                   getEnvRequired("BOT_TOKEN"),
		OwnerID:                    getEnvInt64Required("OWNER_ID"),
		ArchiveChannelID:           getEnvInt64Required("ARCHIVE_CHANNEL_ID"),
		MongoURI:                   getEnvRequired("MONGO_URI"),
		DBName:                     getEnvDefault("DB_NAME", "telegram_archive_db"),
		BroadcastDelay:             getEnvFloat("BROADCAST_DELAY", 0.05),
		CacheTTLSeconds:            getEnvInt("CACHE_TTL_SECONDS", 60),
		WelcomePhoto:               os.Getenv("WELCOME_PHOTO"),
		APIKey:                     os.Getenv("API_KEY"),
		APIRateLimit:               getEnvInt("API_RATE_LIMIT", 60),
		AIBaseURL:                  getEnvDefault("AI_BASE_URL", os.Getenv("OPENAI_API_BASE")),
		AIAPIKey:                   getEnvDefault("AI_API_KEY", os.Getenv("OPENAI_API_KEY")),
		AIModel:                    getEnvDefault("AI_MODEL", "gpt-5-mini"),
		AIRequestTimeoutSeconds:    getEnvInt("AI_REQUEST_TIMEOUT_SECONDS", 45),
		FactoryEncryptionKey:       os.Getenv("FACTORY_ENCRYPTION_KEY"),
		FactoryMaxBotsPerOwner:     getEnvInt("FACTORY_MAX_BOTS_PER_OWNER", 5),
		FactoryWorkers:             getEnvInt("FACTORY_WORKERS", 8),
		StorageQueuePollSeconds:    getEnvInt("STORAGE_QUEUE_POLL_SECONDS", 5),
		StorageMaxAttempts:         getEnvInt("STORAGE_MAX_ATTEMPTS", 5),
		StorageRetryBaseSeconds:    getEnvInt("STORAGE_RETRY_BASE_SECONDS", 5),
		StorageQueueBatchSize:      getEnvInt("STORAGE_QUEUE_BATCH_SIZE", 10),
		FactoryDefaultMaxUsers:     getEnvInt64("FACTORY_DEFAULT_MAX_USERS", 10000),
		FactoryDefaultMaxFiles:     getEnvInt64("FACTORY_DEFAULT_MAX_FILES", 10000),
		FactoryDefaultMaxBytes:     getEnvInt64("FACTORY_DEFAULT_MAX_STORAGE_BYTES", 5368709120),
		FactoryDefaultMaxRequests:  getEnvInt("FACTORY_DEFAULT_MAX_REQUESTS_PER_MINUTE", 120),
		DBAutoExpansion:            getEnvBool("DB_AUTO_EXPANSION", true),
		DBExpansionPollSeconds:     getEnvInt("DB_EXPANSION_POLL_SECONDS", 60),
		DBExpansionBatchSize:       getEnvInt("DB_EXPANSION_BATCH_SIZE", 500),
		DBExpansionMaxDocs:         getEnvInt64("DB_EXPANSION_MAX_DOCS", 100000),
		DBExpansionMaxBytes:        getEnvInt64("DB_EXPANSION_MAX_BYTES", 10737418240),
		DBExpansionLockSeconds:     getEnvInt("DB_EXPANSION_LOCK_SECONDS", 300),
		DBExpansionCooldownSeconds: getEnvInt("DB_EXPANSION_COOLDOWN_SECONDS", 300),
	}

	bcID := os.Getenv("BROADCAST_CHANNEL_ID")
	if bcID != "" {
		val, err := strconv.ParseInt(bcID, 10, 64)
		if err != nil {
			log.Fatalf("CRITICAL ERROR: BROADCAST_CHANNEL_ID must be an integer!")
		}
		Cfg.BroadcastChannelID = val
	}
}

func getEnvRequired(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("CRITICAL ERROR: Environment variable %s is missing!", key)
	}
	return val
}

func getEnvDefault(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func getEnvInt64Required(key string) int64 {
	s := getEnvRequired(key)
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Fatalf("CRITICAL ERROR: %s must be an integer! Got: %s", key, s)
	}
	return val
}

func getEnvInt64(key string, def int64) int64 {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil || val < 0 {
		return def
	}
	return val
}

func getEnvBool(key string, def bool) bool {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	val, err := strconv.ParseBool(s)
	if err != nil {
		return def
	}
	return val
}

func getEnvInt(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return val
}

func getEnvFloat(key string, def float64) float64 {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil || val < 0 {
		return def
	}
	return val
}

func init() {
	_ = fmt.Sprintf // avoid unused import
}
