package email

import "time"

type Message struct {
	ID          string
	ThreadID    string
	InternalAt  time.Time
	DateHeader  string
	From        string
	To          string
	Cc          string
	Bcc         string
	Subject     string
	MessageID   string
	Labels      []string
	Snippet     string
	BodyText    string
	Attachments []Attachment
}

type Attachment struct {
	PartID       string
	AttachmentID string
	Filename     string
	MimeType     string
	Size         int64
	Inline       bool
	Data         []byte
}

func (m Message) HasAttachments() bool {
	return len(m.Attachments) > 0
}
