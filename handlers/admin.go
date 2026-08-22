package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-archive-bot/config"
	"telegram-archive-bot/keyboards"
	"telegram-archive-bot/models"
	"telegram-archive-bot/services"
)

// autoFinishUpload waits 5 seconds then shows the category picker for uploaded files.
func autoFinishUpload(bot *tgbotapi.BotAPI, userID int64, state *models.UserState) {
	time.Sleep(5 * time.Second)
	defer func() { state.Mu.Lock(); state.UploadTimerActive = false; state.Mu.Unlock() }()

	state.Mu.Lock()
	ready := state.Awaiting != nil && state.Awaiting.Type == "upload" && len(state.PendingUploads) > 0
	count := len(state.PendingUploads)
	state.Mu.Unlock()
	if !ready {
		return
	}

	ctx := archiveContext(bot)
	cats, err := services.GetCategories(ctx)
	if err != nil || len(cats) == 0 {
		msg := tgbotapi.NewMessage(userID, "❌ لا توجد تصنيفات. أضف تصنيفاً أولاً من لوحة التحكم.")
		bot.Send(msg)
		return
	}

	kb := keyboards.UploadCategoriesKeyboard(cats)
	text := fmt.Sprintf("✅ تم استلام (%d) ملفات.\n📂 اختر التصنيف لحفظها:", count)
	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = kb
	bot.Send(msg)
}

// sendFileToChannel sends a file to the archive channel.
func sendFileToChannel(bot *tgbotapi.BotAPI, fileID, fileType, name string) (int, string, error) {
	chatID := config.Cfg.ArchiveChannelID
	sender := storageBot(bot)
	var msg tgbotapi.Chattable

	if sender != bot {
		data, err := downloadTelegramBytes(bot, fileID)
		if err != nil {
			return 0, "", fmt.Errorf("transfer file from managed bot: %w", err)
		}
		msg, err = fileMessageFromBytes(chatID, data, fileType, name)
		if err != nil {
			return 0, "", err
		}
	} else {
		switch fileType {
		case "document":
			doc := tgbotapi.NewDocument(chatID, tgbotapi.FileID(fileID))
			doc.Caption = name
			msg = doc
		case "video":
			msg = tgbotapi.NewVideo(chatID, tgbotapi.FileID(fileID))
		case "audio":
			msg = tgbotapi.NewAudio(chatID, tgbotapi.FileID(fileID))
		case "voice":
			msg = tgbotapi.NewVoice(chatID, tgbotapi.FileID(fileID))
		case "animation":
			msg = tgbotapi.NewAnimation(chatID, tgbotapi.FileID(fileID))
		case "photo":
			msg = tgbotapi.NewPhoto(chatID, tgbotapi.FileID(fileID))
		default:
			return 0, "", fmt.Errorf("unsupported file type: %s", fileType)
		}
	}

	sent, err := sender.Send(msg)
	if err != nil {
		return 0, "", err
	}
	storedID := fileIDFromMessage(&sent, fileType)
	if storedID == "" {
		return sent.MessageID, "", fmt.Errorf("storage gateway returned no file_id")
	}
	return sent.MessageID, storedID, nil
}

func fileIDFromMessage(message *tgbotapi.Message, fileType string) string {
	if message == nil {
		return ""
	}
	switch fileType {
	case "document":
		if message.Document != nil {
			return message.Document.FileID
		}
	case "video":
		if message.Video != nil {
			return message.Video.FileID
		}
	case "audio":
		if message.Audio != nil {
			return message.Audio.FileID
		}
	case "voice":
		if message.Voice != nil {
			return message.Voice.FileID
		}
	case "animation":
		if message.Animation != nil {
			return message.Animation.FileID
		}
	case "photo":
		if len(message.Photo) > 0 {
			return message.Photo[len(message.Photo)-1].FileID
		}
	}
	return ""
}

func downloadTelegramBytes(bot *tgbotapi.BotAPI, fileID string) ([]byte, error) {
	file, err := bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(file.Link(bot.Token))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("telegram file download returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func fileMessageFromBytes(chatID int64, data []byte, fileType, name string) (tgbotapi.Chattable, error) {
	input := tgbotapi.FileBytes{Name: name, Bytes: bytes.Clone(data)}
	switch fileType {
	case "document":
		doc := tgbotapi.NewDocument(chatID, input)
		doc.Caption = name
		return doc, nil
	case "video":
		return tgbotapi.NewVideo(chatID, input), nil
	case "audio":
		return tgbotapi.NewAudio(chatID, input), nil
	case "voice":
		return tgbotapi.NewVoice(chatID, input), nil
	case "animation":
		return tgbotapi.NewAnimation(chatID, input), nil
	case "photo":
		return tgbotapi.NewPhoto(chatID, input), nil
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}
}

// saveAllFiles saves all pending uploads to the database and channel.
func saveAllFiles(bot *tgbotapi.BotAPI, chatID int64, userID int64, state *models.UserState) {
	ctx := archiveContext(bot)
	state.Mu.Lock()
	pending := append([]models.PendingUpload(nil), state.PendingUploads...)
	var locCopy *models.UploadLocation
	if state.UploadLoc != nil {
		v := *state.UploadLoc
		locCopy = &v
	}
	state.Mu.Unlock()
	loc := locCopy

	if len(pending) == 0 || loc == nil || loc.CatID == 0 || loc.SubID == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ بيانات غير مكتملة، ابدأ من جديد.")
		bot.Send(msg)
		clearUploadState(state)
		return
	}

	statusMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⏳ جاري حفظ %d ملف...", len(pending)))
	sent, _ := bot.Send(statusMsg)

	saved, failed := 0, 0
	for _, p := range pending {
		if err := checkFileCapacity(ctx, bot, p.FileSize); err != nil {
			log.Printf("upload quota rejected: %v", err)
			failed++
			continue
		}
		msgID := 0
		channelMsgID, storedFileID, err := sendFileToChannel(bot, p.TelegramFileID, p.FileType, p.Name)
		if err != nil {
			log.Printf("Send to channel failed: %v", err)
			failed++
			continue
		}
		msgID = channelMsgID

		fileID, saveErr := services.SaveFile(ctx, p.Name, storedFileID, p.FileType, loc.SubID, msgID, p.FileSize)
		err = saveErr

		if err != nil {
			log.Printf("Upload save failed: %v", err)
			failed++
		} else {
			saved++
			go func(fID int, name, telegramID, fileType string, subjectID int, size int64) {
				_ = services.NotifySubjectSubscribers(ctx, bot, bot.Self.ID, models.File{FileID: fID, Name: name, TelegramFileID: telegramID, FileType: fileType, SubjectID: subjectID, FileSize: size})
			}(fileID, p.Name, storedFileID, p.FileType, loc.SubID, p.FileSize)
		}

	}

	services.LogAdminAction(ctx, userID, "upload_files", map[string]interface{}{
		"count":  len(pending),
		"saved":  saved,
		"failed": failed,
		"loc":    map[string]interface{}{"cat_id": loc.CatID, "sub_id": loc.SubID},
	})

	clearUploadState(state)

	resultText := fmt.Sprintf("✅ تم حفظ %d ملف.", saved)
	if failed > 0 {
		resultText += fmt.Sprintf("\n❌ فشل %d ملف.", failed)
	}
	edit := tgbotapi.NewEditMessageText(chatID, sent.MessageID, resultText)
	bot.Send(edit)

	// Show admin panel
	perms, _ := services.GetUserPermissions(ctx, userID)
	kb := keyboards.AdminPanelKeyboard(perms)
	panelMsg := tgbotapi.NewMessage(chatID, "⚙️ لوحة التحكم")
	panelMsg.ReplyMarkup = kb
	bot.Send(panelMsg)
}

func clearUploadState(state *models.UserState) {
	state.Mu.Lock()
	state.PendingUploads = nil
	state.UploadLoc = nil
	state.Awaiting = nil
	state.UploadTimerActive = false
	state.Mu.Unlock()
}

// GetBroadcastDelay returns the broadcast delay duration from config.
func init() {
	// Register GetBroadcastDelay in services package
}

// Helper functions
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen])
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func parseInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
