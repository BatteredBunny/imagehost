package cmd

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

func parseToken(rawToken string) (uuid.UUID, error) {
	return uuid.Parse(rawToken)
}

func formatTimeDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func mimeIsImage(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func mimeIsVideo(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}
