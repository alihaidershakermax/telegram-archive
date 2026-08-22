package api

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/services"
)

type bundleRequest struct {
	FileIDs []int `json:"file_ids"`
}
type bundleSource struct {
	ID         int
	Name       string
	UploadDate time.Time
	FileSize   int64
	URL        string
}

func (s *Server) bundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.bot == nil {
		writeError(w, http.StatusServiceUnavailable, "bot delivery is not configured")
		return
	}
	userID, err := strconv.ParseInt(r.Header.Get("X-Telegram-User-ID"), 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "telegram user id is required")
		return
	}
	var body bundleRequest
	if !decodeJSON(w, r, &body) || len(body.FileIDs) == 0 || len(body.FileIDs) > 20 {
		writeError(w, http.StatusBadRequest, "file_ids must contain between 1 and 20 files")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	builder := s.bundleBuilder
	if builder == nil {
		builder = buildBundle
	}
	archive, err := builder(ctx, s.bot, body.FileIDs)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to build archive")
		return
	}
	sender := s.bundleSender
	if sender == nil {
		sender = func(target int64, data []byte) error {
			_, sendErr := s.bot.Send(tgbotapi.NewDocument(target, tgbotapi.FileBytes{Name: "telegram-archive.zip", Bytes: data}))
			return sendErr
		}
	}
	if err = sender(userID, archive); err != nil {
		writeError(w, http.StatusBadGateway, "failed to send archive")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sent": true, "file_count": len(body.FileIDs)})
}

func buildBundle(ctx context.Context, bot *tgbotapi.BotAPI, ids []int) ([]byte, error) {
	sources := make([]bundleSource, 0, len(ids))
	for _, id := range ids {
		row, err := services.GetFileRow(ctx, id)
		if err != nil {
			return nil, err
		}
		if row.FileSize > 100*1024*1024 {
			return nil, fmt.Errorf("file exceeds size limit")
		}
		file, err := bot.GetFile(tgbotapi.FileConfig{FileID: row.TelegramFileID})
		if err != nil {
			return nil, err
		}
		sources = append(sources, bundleSource{ID: id, Name: row.Name, UploadDate: row.UploadDate, FileSize: row.FileSize, URL: file.Link(bot.Token)})
	}
	return buildZip(ctx, sources, downloadTelegramFile)
}

func buildZip(ctx context.Context, sources []bundleSource, downloader func(context.Context, string, int64) ([]byte, error)) ([]byte, error) {
	var output bytes.Buffer
	zw := zip.NewWriter(&output)
	seen := make(map[string]int)
	var total int64
	for _, source := range sources {
		if source.FileSize > 100*1024*1024 || total+source.FileSize > 100*1024*1024 {
			zw.Close()
			return nil, fmt.Errorf("bundle exceeds size limit")
		}
		data, err := downloader(ctx, source.URL, 100*1024*1024-total)
		if err != nil {
			zw.Close()
			return nil, err
		}
		name := safeZipName(source.Name, source.ID)
		if seen[name] > 0 {
			seen[name]++
			name = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, filepath.Ext(name)), seen[name], filepath.Ext(name))
		} else {
			seen[name] = 1
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(source.UploadDate)
		entry, err := zw.CreateHeader(header)
		if err != nil {
			zw.Close()
			return nil, err
		}
		written, err := io.Copy(entry, bytes.NewReader(data))
		if err != nil {
			zw.Close()
			return nil, err
		}
		total += written
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func downloadTelegramFile(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("bundle exceeds size limit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("telegram download returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("download exceeds bundle size limit")
	}
	return data, nil
}

func safeZipName(name string, id int) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" {
		return fmt.Sprintf("file-%d", id)
	}
	return strings.ReplaceAll(name, "..", "_")
}
