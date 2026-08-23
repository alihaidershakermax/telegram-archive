package handlers

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type callbackHTTPClient struct {
	mu    sync.Mutex
	paths []string
	forms []url.Values
}

func (c *callbackHTTPClient) Do(req *http.Request) (*http.Response, error) {
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
		result = `{"id":200,"is_bot":true,"first_name":"Child","username":"child_test_bot"}`
	}
	response := `{"ok":true,"result":` + result + `}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(response)),
	}, nil
}

func newCallbackTestBot(t *testing.T, client *callbackHTTPClient) *tgbotapi.BotAPI {
	t.Helper()
	bot, err := tgbotapi.NewBotAPIWithClient("test-token", "https://telegram.test/bot%s/%s", client)
	if err != nil {
		t.Fatalf("create test bot: %v", err)
	}
	return bot
}

func TestAnswerCallbackUsesTelegramCallbackEndpoint(t *testing.T) {
	client := &callbackHTTPClient{}
	bot := newCallbackTestBot(t, client)
	query := &tgbotapi.CallbackQuery{ID: "callback-123", Data: "view_archive"}

	answerCallback(bot, query, "", false)

	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.paths) != 2 {
		t.Fatalf("expected getMe plus answerCallback, got %d requests", len(client.paths))
	}
	if !strings.HasSuffix(client.paths[1], "/answerCallbackQuery") {
		t.Fatalf("expected answerCallbackQuery endpoint, got %q", client.paths[1])
	}
	if got := client.forms[1].Get("callback_query_id"); got != "callback-123" {
		t.Fatalf("unexpected callback id %q", got)
	}
}

func TestInlineKeyboardCallbacksMatchHandlers(t *testing.T) {
	known := []string{
		"back_main", "view_archive", "back_cats", "cat_1", "back_subs_1", "sub_1",
		"back_files_1", "file_1", "download_1", "share_1", "about", "factory_info", "noop",
		"panel", "adm_back", "panel_stats", "panel_users_2", "panel_user_1", "panel_ban_1",
		"panel_unban_1", "panel_mute_1", "panel_unmute_1", "panel_admins", "panel_add_admin",
		"panel_rank_1", "panel_setrank_1_moderator", "panel_rm_admin_1", "panel_content",
		"panel_content_cat_1", "panel_content_sub_1", "panel_new_cat", "panel_new_sub_1",
		"panel_del_sub_1", "panel_del_file_1", "panel_del_cat_1", "panel_up_cat_1",
		"panel_down_cat_1", "panel_up_sub_1", "panel_down_sub_1", "panel_up_file_1",
		"panel_down_file_1", "panel_broadcast", "panel_maint", "panel_logs", "panel_logs_2",
		"panel_welcome", "panel_welcome_text", "panel_welcome_photo", "panel_welcome_preview",
		"start_upload", "cancel_upload", "uploc_cat_1", "uploc_sub_1", "uploc_back_cats",
	}
	for _, data := range known {
		if !callbackDataSupported(data) {
			t.Errorf("callback_data %q is not recognized by handler", data)
		}
	}
}

func callbackDataSupported(data string) bool {
	switch {
	case data == "back_main", data == "view_archive", data == "back_cats", data == "about", data == "factory_info", data == "noop":
		return true
	case strings.HasPrefix(data, "cat_"), strings.HasPrefix(data, "back_subs_"), strings.HasPrefix(data, "sub_"), strings.HasPrefix(data, "back_files_"):
		return true
	case strings.HasPrefix(data, "file_"), strings.HasPrefix(data, "download_"), strings.HasPrefix(data, "share_"):
		return true
	case data == "panel", data == "adm_back", data == "panel_stats", strings.HasPrefix(data, "panel_users"):
		return true
	case strings.HasPrefix(data, "panel_user_"), strings.HasPrefix(data, "panel_ban_"), strings.HasPrefix(data, "panel_unban_"):
		return true
	case strings.HasPrefix(data, "panel_mute_"), strings.HasPrefix(data, "panel_unmute_"), data == "panel_admins", data == "panel_add_admin":
		return true
	case strings.HasPrefix(data, "panel_rank_"), strings.HasPrefix(data, "panel_setrank_"), strings.HasPrefix(data, "panel_rm_admin_"):
		return true
	case data == "panel_content", strings.HasPrefix(data, "panel_content_cat_"), strings.HasPrefix(data, "panel_content_sub_"):
		return true
	case data == "panel_new_cat", strings.HasPrefix(data, "panel_new_sub_"), strings.HasPrefix(data, "panel_del_sub_"):
		return true
	case strings.HasPrefix(data, "panel_del_file_"), strings.HasPrefix(data, "panel_del_cat_"):
		return true
	case strings.HasPrefix(data, "panel_up_cat_"), strings.HasPrefix(data, "panel_down_cat_"):
		return true
	case strings.HasPrefix(data, "panel_up_sub_"), strings.HasPrefix(data, "panel_down_sub_"):
		return true
	case strings.HasPrefix(data, "panel_up_file_"), strings.HasPrefix(data, "panel_down_file_"):
		return true
	case data == "panel_broadcast", data == "panel_maint", data == "panel_logs", strings.HasPrefix(data, "panel_logs_"):
		return true
	case data == "panel_welcome", data == "panel_welcome_text", data == "panel_welcome_photo", data == "panel_welcome_preview":
		return true
	case data == "start_upload", data == "cancel_upload", strings.HasPrefix(data, "uploc_cat_"), strings.HasPrefix(data, "uploc_sub_"), data == "uploc_back_cats":
		return true
	default:
		return false
	}
}
