package email

import (
	"fmt"
	"strings"
)

func Render(message Message) string {
	var out strings.Builder
	writeHeader(&out, "Gmail-ID", message.ID)
	writeHeader(&out, "Thread-ID", message.ThreadID)
	if !message.InternalAt.IsZero() {
		writeHeader(&out, "Timestamp", message.InternalAt.Local().Format("2006-01-02 15:04:05 MST"))
	}
	writeHeader(&out, "Date", message.DateHeader)
	writeHeader(&out, "From", message.From)
	writeHeader(&out, "To", message.To)
	writeHeader(&out, "Cc", message.Cc)
	writeHeader(&out, "Bcc", message.Bcc)
	writeHeader(&out, "Subject", message.Subject)
	writeHeader(&out, "Message-ID", message.MessageID)
	if len(message.Labels) > 0 {
		writeHeader(&out, "Labels", strings.Join(message.Labels, ", "))
	}
	if len(message.Attachments) > 0 {
		parts := make([]string, 0, len(message.Attachments))
		for _, att := range message.Attachments {
			name := att.Filename
			if name == "" {
				name = att.AttachmentID
			}
			if att.Size > 0 {
				name = fmt.Sprintf("%s (%d bytes)", name, att.Size)
			}
			parts = append(parts, name)
		}
		writeHeader(&out, "Attachments", strings.Join(parts, ", "))
	}
	out.WriteString("\n")
	if message.BodyText != "" {
		out.WriteString(message.BodyText)
		out.WriteString("\n")
	}
	return out.String()
}

func Summary(message Message) string {
	at := "unknown"
	if !message.InternalAt.IsZero() {
		at = message.InternalAt.Local().Format("2006-01-02 15:04")
	}
	marker := " "
	if message.HasAttachments() {
		marker = "📎"
	}
	from := compact(message.From, 32)
	subject := compact(message.Subject, 72)
	return fmt.Sprintf("%s %s %-32s %s [%s]", at, marker, from, subject, message.ID)
}

func writeHeader(out *strings.Builder, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	out.WriteString(key)
	out.WriteString(": ")
	out.WriteString(strings.TrimSpace(value))
	out.WriteString("\n")
}

func compact(value string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1] + "…"
}
