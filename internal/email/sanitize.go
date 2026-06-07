package email

import (
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeFilenameChars = regexp.MustCompile(`[^[:alnum:] ._@()+,=\[\]-]+`)

func SafeFilename(value string, fallback string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return normalizeFilename(filepath.Base(value), fallback, maxLen)
}

func SafeLabel(value string, fallback string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return normalizeFilename(value, fallback, maxLen)
}

func normalizeFilename(value string, fallback string, maxLen int) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = unsafeFilenameChars.ReplaceAllString(value, "_")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, ". ")
	if value == "" || value == "." || value == ".." {
		value = fallback
	}
	if maxLen > 0 && len(value) > maxLen {
		ext := filepath.Ext(value)
		stem := strings.TrimSuffix(value, ext)
		limit := maxLen - len(ext)
		if limit < 1 {
			return value[:maxLen]
		}
		if len(stem) > limit {
			stem = stem[:limit]
		}
		value = strings.TrimRight(stem, ". ") + ext
	}
	return value
}
