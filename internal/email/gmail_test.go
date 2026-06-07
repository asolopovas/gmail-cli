package email

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/gmail/v1"
)

func TestFromGmailPrefersPlainTextAndFindsAttachments(t *testing.T) {
	t.Parallel()
	plain := base64.RawURLEncoding.EncodeToString([]byte("Hello,\n\nThis is plain text."))
	html := base64.RawURLEncoding.EncodeToString([]byte("<html><body><p>HTML body</p></body></html>"))
	msg, err := FromGmail(&gmail.Message{
		Id:           "msg-1",
		ThreadId:     "thread-1",
		InternalDate: time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC).UnixMilli(),
		LabelIds:     []string{"INBOX", "IMPORTANT"},
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: "Alice <alice@example.com>"},
				{Name: "To", Value: "Bob <bob@example.com>"},
				{Name: "Subject", Value: "Report"},
			},
			Parts: []*gmail.MessagePart{
				{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: html}},
				{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: plain}},
				{Filename: "report.pdf", MimeType: "application/pdf", Body: &gmail.MessagePartBody{AttachmentId: "att-1", Size: 42}},
			},
		},
	})
	if err != nil {
		t.Fatalf("FromGmail returned error: %v", err)
	}
	if msg.BodyText != "Hello,\n\nThis is plain text." {
		t.Fatalf("BodyText = %q", msg.BodyText)
	}
	if msg.Subject != "Report" || msg.From == "" || msg.To == "" {
		t.Fatalf("metadata not parsed: %+v", msg)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].AttachmentID != "att-1" {
		t.Fatalf("attachments not parsed: %+v", msg.Attachments)
	}
}

func TestFromGmailConvertsHTMLFallback(t *testing.T) {
	t.Parallel()
	htmlData := base64.RawURLEncoding.EncodeToString([]byte("<style>bad</style><p>Hello&nbsp;world</p><br><b>Next</b>"))
	msg, err := FromGmail(&gmail.Message{Payload: &gmail.MessagePart{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: htmlData}}})
	if err != nil {
		t.Fatalf("FromGmail returned error: %v", err)
	}
	if !strings.Contains(msg.BodyText, "Hello world") || !strings.Contains(msg.BodyText, "Next") {
		t.Fatalf("html fallback body = %q", msg.BodyText)
	}
}
