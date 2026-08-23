package handlers

import "testing"

func TestLegacyReplyKeyboardActions(t *testing.T) {
	tests := map[string]string{
		"📁 الملفات":     "archive",
		"📂 الملفات":     "archive",
		"📢 اشتراكاتي":   "subscriptions",
		"🔙 الرئيسية":    "home",
		"🛠 لوحة الأدمن": "panel",
		"📤 رفع ملفات":   "upload",
		"ℹ️ المساعدة":   "help",
	}
	for text, expected := range tests {
		if got := legacyKeyboardAction(text); got != expected {
			t.Errorf("legacy keyboard %q: got %q, want %q", text, got, expected)
		}
	}
	if got := legacyKeyboardAction("نص عادي غير معروف"); got != "" {
		t.Fatalf("unknown text should not be treated as a keyboard action, got %q", got)
	}
}
