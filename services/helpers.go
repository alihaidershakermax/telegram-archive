package services

import (
	"time"

	"telegram-archive-bot/config"
)

// GetBroadcastDelay returns the broadcast delay as a time.Duration.
func GetBroadcastDelay() time.Duration {
	return time.Duration(config.Cfg.BroadcastDelay * float64(time.Second))
}
