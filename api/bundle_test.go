package api

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/config"
)

func TestBundleRequiresBotDelivery(t *testing.T) {
	s := NewServer(&config.Config{APIKey: "secret", APIRateLimit: 10}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundle", nil)
	req.Header.Set("X-API-Key", "secret")
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.Code)
	}
}

func TestBundleRejectsInvalidFileList(t *testing.T) {
	s := NewServer(&config.Config{APIKey: "secret", APIRateLimit: 10}, &tgbotapi.BotAPI{})
	for _, body := range []string{`{"file_ids":[]}`, `{"file_ids":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/bundle", strings.NewReader(body))
		req.Header.Set("X-API-Key", "secret")
		req.Header.Set("X-Telegram-User-ID", "123")
		res := httptest.NewRecorder()
		s.Handler().ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid file list, got %d", res.Code)
		}
	}
}

func TestSafeZipName(t *testing.T) {
	if got := safeZipName("../../notes.pdf", 7); got != "notes.pdf" {
		t.Fatalf("unexpected sanitized name: %q", got)
	}
	if got := safeZipName("", 7); got != "file-7" {
		t.Fatalf("unexpected fallback name: %q", got)
	}
}

func TestBuildZipIntegration(t *testing.T) {
	sources := []bundleSource{{ID: 7, Name: "../../lesson.pdf", URL: "mock://lesson", UploadDate: time.Now()}}
	archive, err := buildZip(context.Background(), sources, func(_ context.Context, url string, _ int64) ([]byte, error) {
		if url != "mock://lesson" {
			t.Fatalf("unexpected URL: %s", url)
		}
		return []byte("lesson content"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "lesson.pdf" {
		t.Fatalf("unexpected ZIP entries: %+v", reader.File)
	}
	body, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil || string(data) != "lesson content" {
		t.Fatalf("unexpected ZIP content: %q, %v", data, err)
	}
	if _, err := buildZip(context.Background(), sources, func(context.Context, string, int64) ([]byte, error) { return nil, errors.New("download failed") }); err == nil {
		t.Fatal("expected downloader error to propagate")
	}
}

func TestDownloadTelegramFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello archive"))
	}))
	defer server.Close()
	data, err := downloadTelegramFile(context.Background(), server.URL, 1024)
	if err != nil || string(data) != "hello archive" {
		t.Fatalf("expected successful download, got %q and %v", data, err)
	}

	failure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "upstream failed", http.StatusBadGateway) }))
	defer failure.Close()
	if _, err := downloadTelegramFile(context.Background(), failure.URL, 1024); err == nil {
		t.Fatal("expected HTTP failure")
	}
}

func TestBundleEndpointPropagatesBuildFailureAndSuccess(t *testing.T) {
	s := NewServer(&config.Config{APIKey: "secret", APIRateLimit: 10}, &tgbotapi.BotAPI{})
	s.bundleBuilder = func(context.Context, *tgbotapi.BotAPI, []int) ([]byte, error) {
		return nil, errors.New("download failed")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundle", strings.NewReader(`{"file_ids":[7]}`))
	req.Header.Set("X-API-Key", "secret")
	req.Header.Set("X-Telegram-User-ID", "123")
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", res.Code)
	}

	s.bundleBuilder = func(context.Context, *tgbotapi.BotAPI, []int) ([]byte, error) { return []byte("zip-bytes"), nil }
	s.bundleSender = func(userID int64, data []byte) error {
		if userID != 123 || string(data) != "zip-bytes" {
			t.Fatalf("unexpected delivery %d %q", userID, data)
		}
		return nil
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/bundle", strings.NewReader(`{"file_ids":[7]}`))
	req.Header.Set("X-API-Key", "secret")
	req.Header.Set("X-Telegram-User-ID", "123")
	res = httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}
