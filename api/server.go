package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/ai"
	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/factory"
	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
)

type apiKeyContextKey struct{}

type Server struct {
	bot           *tgbotapi.BotAPI
	ai            *ai.Client
	apiKey        string
	limit         int
	window        time.Duration
	mu            sync.Mutex
	buckets       map[string]bucket
	bundleBuilder func(context.Context, *tgbotapi.BotAPI, []int) ([]byte, error)
	bundleSender  func(int64, []byte) error
	factory       *factory.Manager
}

type bucket struct {
	started time.Time
	count   int
}

func NewServer(cfg *config.Config, bot *tgbotapi.BotAPI) *Server {
	limit := cfg.APIRateLimit
	if limit <= 0 {
		limit = 60
	}
	return &Server{
		bot:     bot,
		ai:      ai.NewClient(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel, time.Duration(cfg.AIRequestTimeoutSeconds)*time.Second),
		apiKey:  cfg.APIKey,
		limit:   limit,
		window:  time.Minute,
		buckets: make(map[string]bucket),
	}
}

// SetFactory attaches the managed-bot lifecycle service to API v2.
func (s *Server) SetFactory(manager *factory.Manager) { s.factory = manager }

// archiveContextFromRequest selects the primary database by default. A trusted
// API caller may opt into a managed bot namespace with X-Telegram-Bot-ID, but
// only after the ID is verified against the factory registry.
func (s *Server) archiveContextFromRequest(r *http.Request) (context.Context, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Telegram-Bot-ID"))
	if raw == "" {
		return r.Context(), nil
	}
	botID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || botID <= 0 {
		return nil, errors.New("invalid X-Telegram-Bot-ID")
	}
	if s.bot != nil && s.bot.Self.ID == botID {
		return r.Context(), nil
	}
	if s.factory == nil {
		return nil, errors.New("managed bot is not registered")
	}
	rows, err := s.factory.List(r.Context(), 0, true)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.TelegramBotID == botID {
			return db.WithBotDatabase(r.Context(), botID), nil
		}
	}
	return nil, errors.New("managed bot is not registered")
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.health)
	mux.HandleFunc("/api/v1/health", s.health)
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/v2/health", s.factoryV2Health)
	mux.HandleFunc("/api/v2/bots", s.withAuth(s.factoryV2Bots))
	mux.HandleFunc("/api/v2/bots/", s.withAuth(s.factoryV2Bot))
	mux.HandleFunc("/api/v2/router/best", s.withAuth(s.factoryV2Router))
	mux.HandleFunc("/api/v2/router/send", s.withAuth(s.factoryV2Send))
	mux.HandleFunc("/api/v2/storage/queue", s.withAuth(s.storageQueue))
	mux.HandleFunc("/api/v2/monitor", s.withAuth(s.factoryMonitor))
	mux.HandleFunc("/api/v1/categories", s.withAuth(s.categories))
	mux.HandleFunc("/api/v1/subjects", s.withAuth(s.subjects))
	mux.HandleFunc("/api/v1/files", s.withAuth(s.files))
	mux.HandleFunc("/api/v1/bundle", s.withAuth(s.bundle))
	mux.HandleFunc("/api/v1/ai/chat", s.withAuth(s.aiChat))
	mux.HandleFunc("/api/v1/ai/summarize", s.withAuth(s.aiSummarize))
	return requestID(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "service": "telegram-archive-bot"})
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if provided == "" {
			provided = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		}
		if provided == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		isMaster := s.apiKey != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(s.apiKey)) == 1
		var scopedKey *models.APIKey
		if !isMaster {
			var err error
			scopedKey, err = services.VerifyAPIKey(r.Context(), provided)
			if err != nil || scopedKey == nil || !apiKeyPathAllowed(r.URL.Path) {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			rawBotID := strings.TrimSpace(r.Header.Get("X-Telegram-Bot-ID"))
			botID, parseErr := strconv.ParseInt(rawBotID, 10, 64)
			if parseErr != nil || botID <= 0 || botID != scopedKey.BotID {
				writeError(w, http.StatusForbidden, "API key bot scope mismatch")
				return
			}
			required := apiKeyPermissionForRequest(r)
			if required != "" && !services.HasAPIKeyPermission(scopedKey, required) {
				writeError(w, http.StatusForbidden, "API key permission denied")
				return
			}
		}
		key := provided
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			key = key + ":" + host
		}
		if !s.allow(key) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		if scopedKey != nil {
			r = r.WithContext(context.WithValue(r.Context(), apiKeyContextKey{}, scopedKey))
		}
		next(w, r)
	}
}

func apiKeyPathAllowed(path string) bool {
	return strings.HasPrefix(path, "/api/v1/categories") ||
		strings.HasPrefix(path, "/api/v1/subjects") ||
		strings.HasPrefix(path, "/api/v1/files") ||
		strings.HasPrefix(path, "/api/v1/bundle") ||
		strings.HasPrefix(path, "/api/v1/ai/")
}

func apiKeyPermissionForRequest(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/api/v1/ai/") {
		return "archive:read"
	}
	if r.Method == http.MethodGet {
		return "archive:read"
	}
	return "archive:write"
}

func (s *Server) allow(key string) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.buckets[key]
	if b.started.IsZero() || now.Sub(b.started) >= s.window {
		s.buckets[key] = bucket{started: now, count: 1}
		return true
	}
	if b.count >= s.limit {
		return false
	}
	b.count++
	s.buckets[key] = b
	return true
}

func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, err := s.archiveContextFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := services.GetCategories(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load categories")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rows})
}

func (s *Server) subjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("category_id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "category_id must be a positive integer")
		return
	}
	ctx, err := s.archiveContextFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := services.GetSubjects(ctx, &id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load subjects")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rows})
}

func (s *Server) files(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("subject_id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "subject_id must be a positive integer")
		return
	}
	ctx, err := s.archiveContextFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := services.GetFiles(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load files")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": rows})
}

type chatBody struct{ ai.ChatRequest }

type summarizeBody struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
	Style    string `json:"style,omitempty"`
}

func (s *Server) aiChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body chatBody
	if !decodeJSON(w, r, &body) || len(body.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages are required")
		return
	}
	archiveCtx, err := s.archiveContextFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	inputChars := 0
	for _, message := range body.Messages {
		inputChars += len(message.Content)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	result, err := s.ai.Chat(ctx, body.ChatRequest)
	_ = services.RecordAIUsage(archiveCtx, "chat", inputChars, err == nil)
	if err != nil {
		handleAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) aiSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body summarizeBody
	if !decodeJSON(w, r, &body) || strings.TrimSpace(body.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	if len(body.Text) > 100000 {
		writeError(w, http.StatusRequestEntityTooLarge, "text is too large")
		return
	}
	language := body.Language
	if language == "" {
		language = "Arabic"
	}
	style := body.Style
	if style == "" {
		style = "concise educational"
	}
	prompt := "Summarize the following educational text in " + language + ". Style: " + style + ". Preserve key facts and return a clear summary.\n\n" + body.Text
	archiveCtx, err := s.archiveContextFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	result, err := s.ai.Chat(ctx, ai.ChatRequest{Messages: []ai.Message{{Role: "system", Content: "You are an accurate educational summarization assistant."}, {Role: "user", Content: prompt}}})
	_ = services.RecordAIUsage(archiveCtx, "summarize", len(body.Text), err == nil)
	if err != nil {
		handleAIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleAIError(w http.ResponseWriter, err error) {
	if errors.Is(err, ai.ErrNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "ai gateway is not configured")
		return
	}
	log.Printf("AI gateway error: %v", err)
	writeError(w, http.StatusBadGateway, "ai provider unavailable")
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idBytes := make([]byte, 8)
		if _, err := rand.Read(idBytes); err != nil {
			idBytes = []byte(strconv.FormatInt(time.Now().UnixNano(), 10))
		}
		id := hex.EncodeToString(idBytes)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	if err := decoder.Decode(dst); err != nil {
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
