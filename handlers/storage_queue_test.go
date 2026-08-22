package handlers

import (
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-archive-bot/config"
)

func TestStorageQueueConfigUsesDefaultsAndOverrides(t *testing.T) {
	poll, attempts, retry, batch := storageQueueConfig(nil)
	if poll != defaultStoragePollSeconds || attempts != defaultStorageMaxAttempts || retry != defaultStorageRetryBase || batch != defaultStorageBatchSize {
		t.Fatalf("unexpected defaults: %d %d %d %d", poll, attempts, retry, batch)
	}
	poll, attempts, retry, batch = storageQueueConfig(&config.Config{StorageQueuePollSeconds: 2, StorageMaxAttempts: 7, StorageRetryBaseSeconds: 3, StorageQueueBatchSize: 4})
	if poll != 2 || attempts != 7 || retry != 3 || batch != 4 {
		t.Fatalf("unexpected overrides: %d %d %d %d", poll, attempts, retry, batch)
	}
}

func TestStorageMediaPreservesTelegramType(t *testing.T) {
	cases := []struct {
		name string
		want interface{}
		got  tgbotapi.Chattable
	}{
		{name: "photo", want: tgbotapi.PhotoConfig{}, got: storageMedia(1, "p", "photo", "")},
		{name: "video", want: tgbotapi.VideoConfig{}, got: storageMedia(1, "v", "video", "")},
		{name: "document", want: tgbotapi.DocumentConfig{}, got: storageMedia(1, "d", "document", "")},
	}
	for _, tc := range cases {
		switch tc.want.(type) {
		case tgbotapi.PhotoConfig:
			if _, ok := tc.got.(tgbotapi.PhotoConfig); !ok {
				t.Errorf("%s did not create PhotoConfig", tc.name)
			}
		case tgbotapi.VideoConfig:
			if _, ok := tc.got.(tgbotapi.VideoConfig); !ok {
				t.Errorf("%s did not create VideoConfig", tc.name)
			}
		case tgbotapi.DocumentConfig:
			if _, ok := tc.got.(tgbotapi.DocumentConfig); !ok {
				t.Errorf("%s did not create DocumentConfig", tc.name)
			}
		}
	}
}

func TestStorageErrorIsTruncated(t *testing.T) {
	longErr := errors.New(string(make([]byte, maxStorageErrorLength+25)))
	if got := truncateStorageError(longErr); len(got) != maxStorageErrorLength {
		t.Fatalf("expected %d error bytes, got %d", maxStorageErrorLength, len(got))
	}
}
