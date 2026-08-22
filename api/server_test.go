package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"telegram-archive-bot/config"
)

func testServer() *Server {
	return NewServer(&config.Config{APIKey: "secret", APIRateLimit: 10, AIRequestTimeoutSeconds: 1}, nil)
}

func TestHealthIsPublic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestHealthzAliasIsPublic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache header, got %q", got)
	}
}

func TestFactoryV2Health(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v2/health", nil)
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestFactoryV2RequiresConfiguration(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v2/bots", nil)
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.Code)
	}
}

func TestFactoryV2SendRequiresConfiguration(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v2/router/send", nil)
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.Code)
	}
}

func TestProtectedRouteRequiresAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestAIRequiresProviderConfiguration(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/summarize", nil)
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestArchiveNamespaceRejectsInvalidBotID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	req.Header.Set("X-API-Key", "secret")
	req.Header.Set("X-Telegram-Bot-ID", "not-a-number")
	res := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid bot namespace, got %d", res.Code)
	}
}

func TestAPIKeyPathPolicy(t *testing.T) {
	if !apiKeyPathAllowed("/api/v1/files") || !apiKeyPathAllowed("/api/v1/ai/chat") || !apiKeyPathAllowed("/api/v2/groups/-1001") {
		t.Fatal("expected archive and AI paths to be allowed")
	}
	if apiKeyPathAllowed("/api/v2/bots") || apiKeyPathAllowed("/api/v1/health") {
		t.Fatal("expected management and public health paths to be denied for scoped keys")
	}
}

func TestAPIKeyPermissionMapping(t *testing.T) {
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	if got := apiKeyPermissionForRequest(getReq); got != "archive:read" {
		t.Fatalf("expected archive:read, got %q", got)
	}
	groupGetReq := httptest.NewRequest(http.MethodGet, "/api/v2/groups/-1001", nil)
	if got := apiKeyPermissionForRequest(groupGetReq); got != "archive:read" {
		t.Fatalf("expected group archive:read, got %q", got)
	}
	groupPatchReq := httptest.NewRequest(http.MethodPatch, "/api/v2/groups/-1001", nil)
	if got := apiKeyPermissionForRequest(groupPatchReq); got != "archive:write" {
		t.Fatalf("expected group archive:write, got %q", got)
	}
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/files", nil)
	if got := apiKeyPermissionForRequest(postReq); got != "archive:write" {
		t.Fatalf("expected archive:write, got %q", got)
	}
}
