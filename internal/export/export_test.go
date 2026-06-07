package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andrius/gmail-cli/internal/email"
)

func TestWriterSaveCreatesSafeTimestampedExport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := Writer{Root: root}
	msg := email.Message{
		ID:         "abc123",
		InternalAt: time.Date(2026, 6, 7, 14, 22, 33, 0, time.Local),
		From:       "Alice <alice@example.com>",
		Subject:    "../../Quarterly/Report?",
		BodyText:   "hello",
		Attachments: []email.Attachment{
			{Filename: "../../secret.txt", Data: []byte("secret")},
			{Filename: "secret.txt", Data: []byte("duplicate")},
		},
	}
	result, err := writer.Save(msg)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(result.Directory), "26-06-07 142233") {
		t.Fatalf("directory name = %q", filepath.Base(result.Directory))
	}
	body, err := os.ReadFile(result.EmailFile)
	if err != nil {
		t.Fatalf("read email file: %v", err)
	}
	if !strings.Contains(string(body), "Subject: ../../Quarterly/Report?") || !strings.Contains(string(body), "hello") {
		t.Fatalf("email.txt missing expected content:\n%s", body)
	}
	if len(result.Attachments) != 2 {
		t.Fatalf("attachments count = %d", len(result.Attachments))
	}
	for _, path := range result.Attachments {
		if !strings.HasPrefix(path, result.Directory+string(filepath.Separator)) {
			t.Fatalf("attachment escaped directory: %q", path)
		}
	}
}
