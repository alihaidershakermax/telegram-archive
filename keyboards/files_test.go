package keyboards

import (
	"testing"
)

func TestFileActionsKeyboardUsesDocumentDownloadOnly(t *testing.T) {
	keyboard := FileActionsKeyboard(42, "back_cats", "photo", "lecture.png")
	callbacks := make(map[string]bool)
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData != nil {
				callbacks[*button.CallbackData] = true
			}
		}
	}

	if !callbacks["download_42"] {
		t.Fatal("expected the standard document download callback")
	}
	if callbacks["dlimg_42"] {
		t.Fatal("image-specific photo download callback must not be present")
	}
}
