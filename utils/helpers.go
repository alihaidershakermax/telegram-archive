package utils

import (
	"log"
	"path/filepath"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ImageExtensions are file extensions considered as images.
var ImageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
	".bmp": true, ".gif": true, ".heic": true, ".tif": true,
	".tiff": true, ".avif": true,
}

// IsImageFile checks if a file is an image by type or extension.
func IsImageFile(fileType, name string) bool {
	if fileType == "photo" {
		return true
	}
	if name != "" {
		ext := strings.ToLower(filepath.Ext(name))
		return ImageExtensions[ext]
	}
	return false
}

// IsPDFFile checks if a file is a PDF.
func IsPDFFile(fileType, name string) bool {
	if fileType == "pdf" {
		return true
	}
	if name != "" {
		return strings.ToLower(filepath.Ext(name)) == ".pdf"
	}
	return false
}

// FileFromMessage extracts file_id, file_name, and file_type from a Telegram message.
// Returns ("", "", "") if no supported file is found.
func FileFromMessage(msg *tgbotapi.Message) (fileID, fileName, fileType string, fileSize int64) {
	if msg.Document != nil {
		name := msg.Document.FileName
		if name == "" {
			name = "ملف"
		}
		return msg.Document.FileID, name, "document", int64(msg.Document.FileSize)
	}
	if msg.Video != nil {
		name := msg.Video.FileName
		if name == "" {
			name = "فيديو"
		}
		return msg.Video.FileID, name, "video", int64(msg.Video.FileSize)
	}
	if msg.Audio != nil {
		name := msg.Audio.FileName
		if name == "" {
			name = "ملف صوتي"
		}
		return msg.Audio.FileID, name, "audio", int64(msg.Audio.FileSize)
	}
	if msg.Photo != nil && len(msg.Photo) > 0 {
		largest := msg.Photo[len(msg.Photo)-1]
		return largest.FileID, "صورة", "photo", int64(largest.FileSize)
	}
	if msg.Voice != nil {
		return msg.Voice.FileID, "رسالة صوتية", "voice", int64(msg.Voice.FileSize)
	}
	if msg.Animation != nil {
		return msg.Animation.FileID, "GIF", "animation", int64(msg.Animation.FileSize)
	}
	if msg.VideoNote != nil {
		return msg.VideoNote.FileID, "رسالة فيديو", "video_note", int64(msg.VideoNote.FileSize)
	}
	if msg.Sticker != nil {
		return msg.Sticker.FileID, "ملصق", "sticker", int64(msg.Sticker.FileSize)
	}
	return "", "", "", 0
}

// HasFile checks if a message contains a file/media attachment.
func HasFile(msg *tgbotapi.Message) bool {
	return msg.Document != nil || msg.Video != nil || msg.Audio != nil ||
		(msg.Photo != nil && len(msg.Photo) > 0) || msg.Voice != nil ||
		msg.Animation != nil || msg.VideoNote != nil
}

// SafeInt parses a string to int, returning def on failure.
func SafeInt(s string, def int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if s == "" {
		return def
	}
	return n
}

// FormatUserLabel formats a user display name.
func FormatUserLabel(firstName, username string, userID int64) string {
	parts := []string{}
	if firstName != "" {
		parts = append(parts, strings.TrimSpace(firstName))
	}
	if username != "" {
		parts = append(parts, "@"+username)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return "ID:" + strconv.FormatInt(userID, 10)
}

// EditOrSend edits a callback query message, or sends a new one if editing fails.
func EditOrSend(bot *tgbotapi.BotAPI, callbackID string, chatID int64, messageID int, text string, markup *tgbotapi.InlineKeyboardMarkup, hasPhoto bool) {
	if hasPhoto {
		// If the original message had a photo, delete and send new text message
		del := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := bot.Send(del); err != nil {
			log.Printf("EditOrSend delete photo failed: chat_id=%d message_id=%d err=%v", chatID, messageID, err)
		}
		msg := tgbotapi.NewMessage(chatID, text)
		if markup != nil {
			msg.ReplyMarkup = markup
		}
		if _, err := bot.Send(msg); err != nil {
			log.Printf("EditOrSend send replacement failed: chat_id=%d message_id=%d err=%v", chatID, messageID, err)
		}
		return

	}

	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if markup != nil {
		edit.ReplyMarkup = markup
	}
	_, err := bot.Send(edit)
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "message is not modified") {
			return
		}
		log.Printf("EditOrSend edit failed: chat_id=%d message_id=%d err=%v", chatID, messageID, err)
		// Fallback: delete and send new
		del := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, deleteErr := bot.Send(del); deleteErr != nil {
			log.Printf("EditOrSend fallback delete failed: chat_id=%d message_id=%d err=%v", chatID, messageID, deleteErr)
		}
		msg := tgbotapi.NewMessage(chatID, text)
		if markup != nil {
			msg.ReplyMarkup = markup
		}
		if _, sendErr := bot.Send(msg); sendErr != nil {
			log.Printf("EditOrSend fallback send failed: chat_id=%d message_id=%d err=%v", chatID, messageID, sendErr)
		}
	}
}
