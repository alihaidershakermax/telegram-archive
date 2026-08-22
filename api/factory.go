package api

import (
	"errors"
	"net/http"
	"strings"

	"telegram-archive-bot/factory"
)

type factoryBotBody struct {
	Token string `json:"token"`
}

type factorySendBody struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
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
	case r.Method == http.MethodPost && action == "pause":
		if err := s.factory.Pause(r.Context(), id, 0, true); err != nil {
			writeFactoryError(w, err)
			return
		}
		row.Status = "paused"
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": row})
	case r.Method == http.MethodPost && action == "resume":
		if err := s.factory.Resume(r.Context(), id, 0, true); err != nil {
			writeFactoryError(w, err)
			return
		}
		row.Status = "active"
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": row})
	case r.Method == http.MethodDelete:
		if err := s.factory.Delete(r.Context(), id, 0, true); err != nil {
			writeFactoryError(w, err)
			return
		}
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
