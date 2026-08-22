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
