package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"telegram-archive-bot/ai"
	"telegram-archive-bot/db"
	"telegram-archive-bot/factory"
	"telegram-archive-bot/services"
)

type factoryBotBody struct {
	Token string `json:"token"`
}

type factorySendBody struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

type factoryAPIKeyBody struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	ID          string   `json:"id"`
}

type factoryRestoreBody struct {
	BackupID string `json:"backup_id"`
}

type factoryAIIndexBody struct {
	FileID int    `json:"file_id"`
	Text   string `json:"text"`
}

type factoryLimitsBody struct {
	MaxUsers             int64 `json:"max_users"`
	MaxFiles             int64 `json:"max_files"`
	MaxStorageBytes      int64 `json:"max_storage_bytes"`
	MaxRequestsPerMinute int   `json:"max_requests_per_minute"`
}

func (s *Server) factoryV2Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	status := "disabled"
	if s.factory != nil {
		status = "ready"
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": status, "api_version": "v2"})
}

func (s *Server) factoryV2Bots(w http.ResponseWriter, r *http.Request) {
	if s.factory == nil {
		writeError(w, http.StatusServiceUnavailable, "bot factory is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.factory.List(r.Context(), 0, true)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load managed bots")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": rows, "count": len(rows)})
	case http.MethodPost:
		var body factoryBotBody
		if !decodeJSON(w, r, &body) || strings.TrimSpace(body.Token) == "" {
			writeError(w, http.StatusBadRequest, "token is required")
			return
		}
		row, err := s.factory.Register(r.Context(), s.factory.DefaultOwnerID(), body.Token)
		if err != nil {
			writeFactoryError(w, err)
			return
		}
		// API-key users are platform operators, so registrations are owned by the
		// configured owner rather than by an untrusted request field.
		if row.OwnerID == 0 {
			writeError(w, http.StatusInternalServerError, "factory owner is not configured")
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "register_managed_bot", map[string]interface{}{"bot_id": row.TelegramBotID, "factory_id": row.ID})
		writeJSON(w, http.StatusCreated, map[string]interface{}{"data": row})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) factoryV2Bot(w http.ResponseWriter, r *http.Request) {
	if s.factory == nil {
		writeError(w, http.StatusServiceUnavailable, "bot factory is not configured")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/bots/"), "/")
	parts := strings.Split(path, "/")
	id := parts[0]
	if id == "" || len(parts) > 2 {
		writeError(w, http.StatusBadRequest, "bot id is required")
		return
	}
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	row, err := s.factory.Get(r.Context(), id, 0, true)
	if err != nil {
		writeFactoryError(w, err)
		return
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": row})
	case r.Method == http.MethodGet && action == "expansion":
		if s.expansion == nil {
			writeError(w, http.StatusServiceUnavailable, "auto-expansion is not configured")
			return
		}
		state, err := factory.ExpansionState(r.Context(), row.TelegramBotID)
		if err != nil {
			writeFactoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": state})
	case r.Method == http.MethodPost && action == "ai-index":
		var body factoryAIIndexBody
		if !decodeJSON(w, r, &body) || body.FileID <= 0 || strings.TrimSpace(body.Text) == "" {
			writeError(w, http.StatusBadRequest, "file_id and text are required")
			return
		}
		if len(body.Text) > 100000 {
			writeError(w, http.StatusRequestEntityTooLarge, "text is too large")
			return
		}
		archiveCtx := db.WithBotDatabase(r.Context(), row.TelegramBotID)
		if _, err := services.GetFileRow(archiveCtx, body.FileID); err != nil {
			writeError(w, http.StatusNotFound, "file does not belong to this bot")
			return
		}
		prompt := "Analyze this educational archive text. Return only valid JSON with exactly two fields: summary (string) and tags (array of short strings). Do not invent facts.\\n\\n" + body.Text
		result, err := s.ai.Chat(r.Context(), ai.ChatRequest{Messages: []ai.Message{{Role: "system", Content: "You classify and summarize educational content accurately."}, {Role: "user", Content: prompt}}})
		_ = services.RecordAIUsage(archiveCtx, "index", len(body.Text), err == nil)
		if err != nil {
			handleAIError(w, err)
			return
		}
		content := ""
		if len(result.Choices) > 0 {
			content = strings.TrimSpace(result.Choices[0].Message.Content)
		}
		var metadata struct {
			Summary string   `json:"summary"`
			Tags    []string `json:"tags"`
		}
		if json.Unmarshal([]byte(content), &metadata) != nil {
			metadata.Summary = content
		}
		index, err := services.UpsertAIIndex(archiveCtx, body.FileID, body.Text, metadata.Summary, metadata.Tags)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save AI index")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": index})
	case r.Method == http.MethodGet && action == "backups":
		backups, err := services.ListBotBackups(r.Context(), row.TelegramBotID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load bot backups")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": backups})
	case r.Method == http.MethodPost && action == "backup":
		backup, err := services.CreateBotBackup(r.Context(), row.TelegramBotID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create bot backup")
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "create_backup", map[string]interface{}{"bot_id": row.TelegramBotID, "backup_id": backup.ID})
		writeJSON(w, http.StatusCreated, map[string]interface{}{"data": backup})
	case r.Method == http.MethodPost && action == "restore":
		var body factoryRestoreBody
		if !decodeJSON(w, r, &body) || strings.TrimSpace(body.BackupID) == "" {
			writeError(w, http.StatusBadRequest, "backup_id is required")
			return
		}
		if err := services.RestoreBotBackup(r.Context(), row.TelegramBotID, body.BackupID); err != nil {
			writeFactoryError(w, err)
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "restore_backup", map[string]interface{}{"bot_id": row.TelegramBotID, "backup_id": body.BackupID})
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "restored", "bot_id": row.TelegramBotID, "backup_id": body.BackupID})
	case r.Method == http.MethodGet && action == "api-keys":
		keys, err := services.ListAPIKeys(r.Context(), row.TelegramBotID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load bot API keys")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": keys})
	case r.Method == http.MethodPost && action == "api-keys":
		var body factoryAPIKeyBody
		if !decodeJSON(w, r, &body) {
			return
		}
		key, raw, err := services.CreateAPIKey(r.Context(), row.TelegramBotID, body.Name, body.Permissions)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "create_api_key", map[string]interface{}{"bot_id": row.TelegramBotID, "key_id": key.ID, "permissions": key.Permissions})
		writeJSON(w, http.StatusCreated, map[string]interface{}{"data": map[string]interface{}{"key": key, "secret": raw}})
	case r.Method == http.MethodDelete && action == "api-keys":
		var body factoryAPIKeyBody
		if !decodeJSON(w, r, &body) || strings.TrimSpace(body.ID) == "" {
			writeError(w, http.StatusBadRequest, "api key id is required")
			return
		}
		if err := services.RevokeAPIKey(r.Context(), row.TelegramBotID, body.ID); err != nil {
			writeFactoryError(w, err)
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "revoke_api_key", map[string]interface{}{"bot_id": row.TelegramBotID, "key_id": body.ID})
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && action == "usage":
		usage, err := services.GetUsage(db.WithBotDatabase(r.Context(), row.TelegramBotID))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load bot usage")
			return
		}
		queue, err := services.GetStorageQueueStats(r.Context(), row.TelegramBotID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load storage queue")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"usage": usage, "queue": queue}})
	case r.Method == http.MethodPost && action == "rotate-token":
		var body factoryBotBody
		if !decodeJSON(w, r, &body) {
			return
		}
		updated, err := s.factory.RotateToken(r.Context(), id, 0, true, body.Token)
		if err != nil {
			writeFactoryError(w, err)
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "rotate_bot_token", map[string]interface{}{"bot_id": row.TelegramBotID})
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": updated})
	case r.Method == http.MethodPost && action == "limits":
		var body factoryLimitsBody
		if !decodeJSON(w, r, &body) {
			return
		}
		updated, err := s.factory.UpdateLimits(r.Context(), id, 0, true, body.MaxUsers, body.MaxFiles, body.MaxStorageBytes, body.MaxRequestsPerMinute)
		if err != nil {
			writeFactoryError(w, err)
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "update_bot_limits", map[string]interface{}{"bot_id": row.TelegramBotID, "max_users": body.MaxUsers, "max_files": body.MaxFiles, "max_storage_bytes": body.MaxStorageBytes, "max_requests_per_minute": body.MaxRequestsPerMinute})
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": updated})
	case r.Method == http.MethodPost && action == "pause":
		if err := s.factory.Pause(r.Context(), id, 0, true); err != nil {
			writeFactoryError(w, err)
			return
		}
		row.Status = "paused"
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "pause_managed_bot", map[string]interface{}{"bot_id": row.TelegramBotID})
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": row})
	case r.Method == http.MethodPost && action == "resume":
		if err := s.factory.Resume(r.Context(), id, 0, true); err != nil {
			writeFactoryError(w, err)
			return
		}
		row.Status = "active"
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "resume_managed_bot", map[string]interface{}{"bot_id": row.TelegramBotID})
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": row})
	case r.Method == http.MethodDelete:
		if err := s.factory.Delete(r.Context(), id, 0, true); err != nil {
			writeFactoryError(w, err)
			return
		}
		services.LogAdminAction(r.Context(), s.factory.DefaultOwnerID(), "delete_managed_bot", map[string]interface{}{"bot_id": row.TelegramBotID, "factory_id": row.ID})
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) factoryV2Send(w http.ResponseWriter, r *http.Request) {
	if s.factory == nil {
		writeError(w, http.StatusServiceUnavailable, "bot factory is not configured")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body factorySendBody
	if !decodeJSON(w, r, &body) || body.ChatID == 0 || strings.TrimSpace(body.Text) == "" {
		writeError(w, http.StatusBadRequest, "chat_id and text are required")
		return
	}
	row, err := s.factory.SendText(r.Context(), s.factory.DefaultOwnerID(), body.ChatID, body.Text, true)
	if err != nil {
		writeFactoryError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"data": row, "routed": true})
}

func (s *Server) factoryV2Router(w http.ResponseWriter, r *http.Request) {
	if s.factory == nil {
		writeError(w, http.StatusServiceUnavailable, "bot factory is not configured")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	row, err := s.factory.Best(r.Context(), 0, true)
	if err != nil {
		writeFactoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": row, "policy": "health-latency-errors-recency"})
}

func writeFactoryError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, factory.ErrNotFound):
		status = http.StatusNotFound
	case strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "invalid"), strings.Contains(err.Error(), "format"), strings.Contains(err.Error(), "limit"), strings.Contains(err.Error(), "already registered"):
		status = http.StatusBadRequest
	case strings.Contains(err.Error(), "not healthy"):
		status = http.StatusServiceUnavailable
	}
	writeError(w, status, err.Error())
}

func (s *Server) storageQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	stats, err := services.GetStorageQueueStats(r.Context(), 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load storage queue")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": stats})
}

func (s *Server) factoryMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.factory == nil {
		writeError(w, http.StatusServiceUnavailable, "bot factory is not configured")
		return
	}
	rows, err := s.factory.List(r.Context(), 0, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load bot metrics")
		return
	}
	type monitorRow struct {
		Bot       interface{}                `json:"bot"`
		Usage     services.Usage             `json:"usage"`
		Queue     services.StorageQueueStats `json:"queue"`
		Expansion interface{}                `json:"expansion,omitempty"`
	}

	result := make([]monitorRow, 0, len(rows))
	for _, row := range rows {
		usage, usageErr := services.GetUsage(db.WithBotDatabase(r.Context(), row.TelegramBotID))
		queue, queueErr := services.GetStorageQueueStats(r.Context(), row.TelegramBotID)
		if usageErr != nil || queueErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to load bot monitor data")
			return
		}
		var expansion interface{}
		if state, stateErr := factory.ExpansionState(r.Context(), row.TelegramBotID); stateErr == nil {
			expansion = state
		}
		result = append(result, monitorRow{Bot: row, Usage: usage, Queue: queue, Expansion: expansion})

	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": result, "count": len(result)})
}
