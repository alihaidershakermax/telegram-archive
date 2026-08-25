package utils

import "testing"

func TestTelegramHTTPClientHasBoundedTimeout(t *testing.T) {
	if got := newTelegramHTTPClient().Timeout; got != TelegramHTTPTimeout {
		t.Fatalf("Telegram HTTP timeout = %s, want %s", got, TelegramHTTPTimeout)
	}
}

func TestFormatUserLabelWithoutNames(t *testing.T) {
	if got := FormatUserLabel("", "", 123456789); got != "ID:123456789" {
		t.Fatalf("FormatUserLabel without names = %q, want %q", got, "ID:123456789")
	}
}

func TestIsImageFile(t *testing.T) {
	for _, test := range []struct {
		fileType string
		name     string
		want     bool
	}{
		{fileType: "photo", name: "", want: true},
		{fileType: "document", name: "lecture.PNG", want: true},
		{fileType: "document", name: "scan.webp", want: true},
		{fileType: "document", name: "notes.pdf", want: false},
		{fileType: "video", name: "lesson.mp4", want: false},
	} {
		if got := IsImageFile(test.fileType, test.name); got != test.want {
			t.Fatalf("IsImageFile(%q, %q) = %v, want %v", test.fileType, test.name, got, test.want)
		}
	}
}
