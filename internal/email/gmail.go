package email

import (
	"encoding/base64"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"google.golang.org/api/gmail/v1"
)

func FromGmail(msg *gmail.Message) (Message, error) {
	if msg == nil {
		return Message{}, fmt.Errorf("gmail message is nil")
	}
	parsed := Message{
		ID:         msg.Id,
		ThreadID:   msg.ThreadId,
		Snippet:    strings.TrimSpace(msg.Snippet),
		Labels:     append([]string(nil), msg.LabelIds...),
		InternalAt: internalDate(msg.InternalDate),
	}
	if msg.Payload != nil {
		parsed.DateHeader = headerValue(msg.Payload.Headers, "Date")
		parsed.From = headerValue(msg.Payload.Headers, "From")
		parsed.To = headerValue(msg.Payload.Headers, "To")
		parsed.Cc = headerValue(msg.Payload.Headers, "Cc")
		parsed.Bcc = headerValue(msg.Payload.Headers, "Bcc")
		parsed.Subject = headerValue(msg.Payload.Headers, "Subject")
		parsed.MessageID = headerValue(msg.Payload.Headers, "Message-ID")

		var plainParts []string
		var htmlParts []string
		walkPart(msg.Payload, &plainParts, &htmlParts, &parsed.Attachments)
		parsed.BodyText = cleanBody(chooseBody(plainParts, htmlParts, parsed.Snippet))
	}
	slices.Sort(parsed.Labels)
	return parsed, nil
}

func headerValue(headers []*gmail.MessagePartHeader, name string) string {
	for _, h := range headers {
		if h != nil && strings.EqualFold(h.Name, name) {
			return strings.TrimSpace(h.Value)
		}
	}
	return ""
}

func internalDate(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func walkPart(part *gmail.MessagePart, plainParts *[]string, htmlParts *[]string, attachments *[]Attachment) {
	if part == nil {
		return
	}
	filename := strings.TrimSpace(part.Filename)
	if filename != "" || part.Body != nil && part.Body.AttachmentId != "" {
		att := Attachment{PartID: part.PartId, Filename: filename, MimeType: part.MimeType}
		if part.Body != nil {
			att.AttachmentID = part.Body.AttachmentId
			att.Size = part.Body.Size
			if part.Body.Data != "" {
				att.Data, _ = decodeURLBase64(part.Body.Data)
			}
		}
		att.Inline = strings.Contains(strings.ToLower(headerValue(part.Headers, "Content-Disposition")), "inline")
		*attachments = append(*attachments, att)
		if filename != "" {
			// Named parts are attachments even when Gmail also includes small inline data.
			return
		}
	}
	if part.Body != nil && part.Body.Data != "" {
		data, err := decodeURLBase64(part.Body.Data)
		if err == nil {
			switch strings.ToLower(part.MimeType) {
			case "text/plain":
				*plainParts = append(*plainParts, string(data))
			case "text/html":
				*htmlParts = append(*htmlParts, htmlToText(string(data)))
			}
		}
	}
	for _, child := range part.Parts {
		walkPart(child, plainParts, htmlParts, attachments)
	}
}

func chooseBody(plainParts []string, htmlParts []string, snippet string) string {
	if len(plainParts) > 0 {
		return strings.Join(plainParts, "\n\n")
	}
	if len(htmlParts) > 0 {
		return strings.Join(htmlParts, "\n\n")
	}
	return snippet
}

func decodeURLBase64(value string) ([]byte, error) {
	if data, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(value)
}

var (
	scriptRE     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRE      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	breakRE      = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li|/tr|h[1-6])\b[^>]*>`)
	tagRE        = regexp.MustCompile(`(?s)<[^>]+>`)
	blankLinesRE = regexp.MustCompile(`\n{3,}`)
	spacesRE     = regexp.MustCompile(`[ \t\r]+`)
)

func htmlToText(value string) string {
	value = scriptRE.ReplaceAllString(value, "")
	value = styleRE.ReplaceAllString(value, "")
	value = breakRE.ReplaceAllString(value, "\n")
	value = tagRE.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	return cleanBody(value)
}

func cleanBody(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.Map(removeEmailNoiseRune, value)
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(spacesRE.ReplaceAllString(line, " "), " ")
	}
	value = strings.TrimSpace(strings.Join(lines, "\n"))
	value = blankLinesRE.ReplaceAllString(value, "\n\n")
	return value
}

func removeEmailNoiseRune(r rune) rune {
	switch r {
	case '\u034f', '\u00ad', '\u200b', '\u200c', '\u200d', '\u2060', '\ufeff':
		return -1
	}
	if unicode.Is(unicode.Cf, r) {
		return -1
	}
	return r
}
