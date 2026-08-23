package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type updateTraceHTTPClient struct {
	mu    sync.Mutex
	paths []string
	forms []url.Values
}

func (c *updateTraceHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.paths = append(c.paths, req.URL.Path)
	c.forms = append(c.forms, form)
	c.mu.Unlock()

	result := `true`
	if strings.HasSuffix(req.URL.Path, "/getMe") {
		result = `{"id":901,"is_bot":true,"first_name":"Parent","username":"parent_test_bot"}`
	}
	response := `{"ok":true,"result":` + result + `}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(response)),
	}, nil
}

func TestHandleUpdateAcknowledgesCallbackBeforeRoute(t *testing.T) {
	client := &updateTraceHTTPClient{}
	bot, err := tgbotapi.NewBotAPIWithClient("test-token", "https://telegram.test/bot%s/%s", client)
	if err != nil {
		t.Fatalf("create test bot: %v", err)
	}

	query := &tgbotapi.CallbackQuery{
		ID:   "parent-callback-123",
		Data: "view_archive",
		From: &tgbotapi.User{ID: 777},
		Message: &tgbotapi.Message{
			MessageID: 12,
			From:      &tgbotapi.User{ID: 901, IsBot: true},
			Chat:      &tgbotapi.Chat{ID: 777},
		},
	}

	// The route may stop after the acknowledgement because this unit test does
	// not initialize MongoDB. The ordering of the Telegram calls is the contract.
	handleUpdate(bot, tgbotapi.Update{UpdateID: 99, CallbackQuery: query})

	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.paths) < 2 {
		t.Fatalf("expected getMe plus callback acknowledgement, got %d calls", len(client.paths))
	}
	if !strings.HasSuffix(client.paths[1], "/answerCallbackQuery") {
		t.Fatalf("callback was not acknowledged first: %q", client.paths[1])
	}
	if got := client.forms[1].Get("callback_query_id"); got != query.ID {
		t.Fatalf("unexpected callback id %q", got)
	}
}
