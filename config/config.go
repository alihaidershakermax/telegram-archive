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
	BotToken                string
	OwnerID                 int64
	ArchiveChannelID        int64
	MongoURI                string
	DBName                  string
	BroadcastChannelID      int64
	BroadcastDelay          float64
	CacheTTLSeconds         int
	WelcomePhoto            string
	APIKey                  string
	APIRateLimit            int
	AIBaseURL               string
	AIAPIKey                string
	AIModel                 string
	AIRequestTimeoutSeconds int
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
		BotToken:                getEnvRequired("BOT_TOKEN"),
		OwnerID:                 getEnvInt64Required("OWNER_ID"),
		ArchiveChannelID:        getEnvInt64Required("ARCHIVE_CHANNEL_ID"),
		MongoURI:                getEnvRequired("MONGO_URI"),
		DBName:                  getEnvDefault("DB_NAME", "telegram_archive_db"),
		BroadcastDelay:          getEnvFloat("BROADCAST_DELAY", 0.05),
		CacheTTLSeconds:         getEnvInt("CACHE_TTL_SECONDS", 60),
		WelcomePhoto:            os.Getenv("WELCOME_PHOTO"),
		APIKey:                  os.Getenv("API_KEY"),
		APIRateLimit:            getEnvInt("API_RATE_LIMIT", 60),
		AIBaseURL:               getEnvDefault("AI_BASE_URL", os.Getenv("OPENAI_API_BASE")),
		AIAPIKey:                getEnvDefault("AI_API_KEY", os.Getenv("OPENAI_API_KEY")),
		AIModel:                 getEnvDefault("AI_MODEL", "gpt-5-mini"),
		AIRequestTimeoutSeconds: getEnvInt("AI_REQUEST_TIMEOUT_SECONDS", 45),
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
