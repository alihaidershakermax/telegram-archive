package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/keyboards"
	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
	"telegram-archive-bot/utils"
)

const searchPageSize = 8

func HandleSearchCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil {
		return
	}
	if services.IsBanned(context.Background(), message.From.ID) || services.IsMuted(context.Background(), message.From.ID) {
		return
	}
	WithState(message.From.ID, func(state *models.UserState) {
		state.Awaiting = &models.AwaitingState{Type: "search"}
	})
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "🔎 أرسل كلمة البحث. يمكنك استخدام: نص | النوع:pdf | الترتيب:الأحدث\nمثال: تشريح النوع:pdf الترتيب:الأكثر_تنزيلاً"))
}

func HandleSearchText(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string) {
	if message.From == nil {
		return
	}
	query, params := parseSearchQuery(text)
	if query == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ اكتب كلمة أو عبارة للبحث."))
		return
	}
	WithState(message.From.ID, func(state *models.UserState) {
		state.Awaiting = &models.AwaitingState{Type: "search_result"}
		state.SearchQuery = text
	})
	showSearchResults(bot, message.Chat.ID, query, params, 0, nil)
}

func showSearchPage(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, page int) {
	state := GetState(query.From.ID)
	state.Mu.Lock()
	searchText := state.SearchQuery
	state.Mu.Unlock()
	searchQuery, params := parseSearchQuery(searchText)
	showSearchResults(bot, query.Message.Chat.ID, searchQuery, params, page, query)
}

func showSearchResults(bot *tgbotapi.BotAPI, chatID int64, query string, params services.FileSearchParams, page int, callback *tgbotapi.CallbackQuery) {
	params.Query = query
	params.Page = page
	params.Limit = searchPageSize
	result, err := services.SearchFiles(context.Background(), params)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ تعذر تنفيذ البحث حالياً."))
		return
	}
	text := fmt.Sprintf("🔎 نتائج البحث عن: %s\nالصفحة %d — إجمالي النتائج: %d", query, result.Page+1, result.Total)
	if len(result.Files) == 0 {
		text += "\n\nلم يتم العثور على ملفات مطابقة."
	}
	keyboard := keyboards.SearchResultsKeyboard(result.Files, result.Page, result.Page > 0, (result.Page+1)*result.Limit < result.Total)
	if callback != nil {
		hasPhoto := callback.Message.Photo != nil && len(callback.Message.Photo) > 0
		utils.EditOrSend(bot, callback.ID, chatID, callback.Message.MessageID, text, &keyboard, hasPhoto)
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func parseSearchQuery(raw string) (string, services.FileSearchParams) {
	params := services.FileSearchParams{Sort: "newest"}
	parts := strings.Fields(strings.TrimSpace(raw))
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, "النوع:") || strings.HasPrefix(lower, "type:"):
			params.FileType = strings.TrimSpace(strings.SplitN(part, ":", 2)[1])
		case strings.HasPrefix(lower, "الترتيب:") || strings.HasPrefix(lower, "sort:"):
			value := strings.TrimSpace(strings.SplitN(part, ":", 2)[1])
			switch value {
			case "الأقدم", "oldest":
				params.Sort = "oldest"
			case "الأكثر_تنزيلاً", "downloads":
				params.Sort = "downloads"
			case "الاسم", "name":
				params.Sort = "name"
			}
		case strings.HasPrefix(lower, "المادة:") || strings.HasPrefix(lower, "subject:"):
			if id, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(part, ":", 2)[1])); err == nil {
				params.SubjectID = id
			}
		case strings.HasPrefix(lower, "التصنيف:") || strings.HasPrefix(lower, "category:"):
			if id, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(part, ":", 2)[1])); err == nil {
				params.CategoryID = id
			}
		default:
			words = append(words, part)
		}
	}
	return strings.Join(words, " "), params
}
