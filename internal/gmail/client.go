package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/andrius/gmail-cli/internal/email"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const userID = "me"

type Client struct {
	service *gmailapi.Service
	logger  *slog.Logger
}

func New(ctx context.Context, httpClient *http.Client, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}
	service, err := gmailapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &Client{service: service, logger: logger}, nil
}

type SearchOptions struct {
	Query              string
	Limit              int
	IncludePayload     bool
	IncludeAttachments bool
}

func (c *Client) Search(ctx context.Context, opts SearchOptions) ([]email.Message, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	query := opts.Query
	c.logger.Info("gmail_search", "limit", limit, "include_payload", opts.IncludePayload, "include_attachments", opts.IncludeAttachments)

	ids, err := c.searchIDs(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	messages := make([]email.Message, 0, len(ids))
	for _, id := range ids {
		msg, err := c.getMessage(ctx, id, opts.IncludePayload, opts.IncludeAttachments)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	slices.SortFunc(messages, func(a, b email.Message) int {
		if a.InternalAt.Equal(b.InternalAt) {
			return cmpString(a.ID, b.ID)
		}
		if a.InternalAt.Before(b.InternalAt) {
			return -1
		}
		return 1
	})
	return messages, nil
}

func (c *Client) searchIDs(ctx context.Context, query string, limit int) ([]string, error) {
	ids := make([]string, 0, limit)
	pageToken := ""
	for len(ids) < limit {
		remaining := min(int64(limit-len(ids)), 100)
		req := c.service.Users.Messages.List(userID).MaxResults(remaining).Context(ctx)
		if query != "" {
			req = req.Q(query)
		}
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("search gmail messages: %w", err)
		}
		for _, item := range resp.Messages {
			if item != nil && item.Id != "" {
				ids = append(ids, item.Id)
				if len(ids) >= limit {
					break
				}
			}
		}
		if resp.NextPageToken == "" || len(resp.Messages) == 0 {
			break
		}
		pageToken = resp.NextPageToken
	}
	return ids, nil
}

func (c *Client) getMessage(ctx context.Context, id string, includePayload bool, includeAttachments bool) (email.Message, error) {
	format := "metadata"
	if includePayload || includeAttachments {
		format = "full"
	}
	req := c.service.Users.Messages.Get(userID, id).Format(format).Context(ctx)
	if format == "metadata" {
		req = req.MetadataHeaders("Date", "From", "To", "Cc", "Bcc", "Subject", "Message-ID")
	}
	raw, err := req.Do()
	if err != nil {
		return email.Message{}, fmt.Errorf("get gmail message %s: %w", id, err)
	}
	msg, err := email.FromGmail(raw)
	if err != nil {
		return email.Message{}, err
	}
	if includeAttachments {
		if err := c.populateAttachments(ctx, &msg); err != nil {
			return email.Message{}, err
		}
	}
	return msg, nil
}

func (c *Client) populateAttachments(ctx context.Context, msg *email.Message) error {
	for i := range msg.Attachments {
		att := &msg.Attachments[i]
		if len(att.Data) > 0 || att.AttachmentID == "" {
			continue
		}
		data, err := c.service.Users.Messages.Attachments.Get(userID, msg.ID, att.AttachmentID).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("get attachment %s for message %s: %w", att.AttachmentID, msg.ID, err)
		}
		decoded, err := decodeGmailData(data.Data)
		if err != nil {
			return fmt.Errorf("decode attachment %s for message %s: %w", att.AttachmentID, msg.ID, err)
		}
		att.Data = decoded
	}
	return nil
}

func decodeGmailData(value string) ([]byte, error) {
	if data, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(value)
}

func cmpString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
