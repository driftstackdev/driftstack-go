package driftstack

import (
	"context"
	"net/url"
	"strconv"
)

// SupportConversation is one Help conversation with Driftstack support, as
// the customer sees it.
type SupportConversation struct {
	ID            string  `json:"id"`
	Subject       string  `json:"subject"`
	Status        string  `json:"status"` // waiting_for_driftstack | waiting_for_you | closed
	TopicHint     *string `json:"topic_hint"`
	Unread        bool    `json:"unread"`
	CreatedAt     string  `json:"created_at"`
	LastMessageAt string  `json:"last_message_at"`
	ClosedAt      *string `json:"closed_at"`
}

// SupportMessage is one message in a conversation.
type SupportMessage struct {
	ID              string         `json:"id"`
	Author          string         `json:"author"` // you | driftstack
	Via             string         `json:"via"`    // app | dashboard | email
	Body            string         `json:"body"`
	SentAt          string         `json:"sent_at"`
	IncludedDetails map[string]any `json:"included_details"`
	Helpful         *bool          `json:"helpful"`
}

// SupportConversationList is a page of conversations with the unread and open counts.
type SupportConversationList struct {
	Data        []SupportConversation `json:"data"`
	NextCursor  *string               `json:"next_cursor"`
	UnreadCount int                   `json:"unread_count"`
	OpenCount   int                   `json:"open_count"`
}

// SupportConversationDetail is a conversation with its messages, oldest first.
type SupportConversationDetail struct {
	Conversation SupportConversation `json:"conversation"`
	Messages     []SupportMessage    `json:"messages"`
}

// SupportConversationMessage is a conversation with the message a write produced.
type SupportConversationMessage struct {
	Conversation SupportConversation `json:"conversation"`
	Message      SupportMessage      `json:"message"`
}

// CreateSupportConversationParams starts a conversation with its first message.
type CreateSupportConversationParams struct {
	Body      string `json:"body"`
	Subject   string `json:"subject,omitempty"`
	TopicHint string `json:"topic_hint,omitempty"` // question | session | billing | account | other
	// Context is the set of details you choose to include (app_version,
	// platform, deployment, workspace_account_id, profile_id, session_id,
	// last_error). Unknown fields are refused.
	Context map[string]any `json:"context,omitempty"`
	// IdempotencyKey makes a retried create return the original conversation
	// instead of opening a second one. Sent as the Idempotency-Key header.
	IdempotencyKey string `json:"-"`
}

// ListSupportConversationsParams pages the conversation list.
type ListSupportConversationsParams struct {
	Cursor string
	Limit  int
}

// SupportResource is Help: your support conversations with Driftstack. A
// person reads and approves every reply; replies appear here and are copied
// to your email.
type SupportResource struct {
	client *Client
}

// List returns your conversations, newest first.
func (r *SupportResource) List(ctx context.Context, params *ListSupportConversationsParams) (*SupportConversationList, error) {
	q := url.Values{}
	if params != nil {
		if params.Cursor != "" {
			q.Set("cursor", params.Cursor)
		}
		if params.Limit > 0 {
			q.Set("limit", strconv.Itoa(params.Limit))
		}
	}
	var out SupportConversationList
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/support/conversations",
		query:  q,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create asks a question or reports a problem.
func (r *SupportResource) Create(ctx context.Context, params CreateSupportConversationParams) (*SupportConversationMessage, error) {
	var headers map[string]string
	if params.IdempotencyKey != "" {
		headers = map[string]string{"Idempotency-Key": params.IdempotencyKey}
	}
	var out SupportConversationMessage
	if err := r.client.do(ctx, requestOptions{
		method:  "POST",
		path:    "/v1/support/conversations",
		body:    params,
		out:     &out,
		headers: headers,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one conversation with its messages.
func (r *SupportResource) Get(ctx context.Context, conversationID string) (*SupportConversationDetail, error) {
	var out SupportConversationDetail
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/support/conversations/" + url.PathEscape(conversationID),
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddMessage adds a follow-up from you. Reopens a closed conversation and marks it read.
func (r *SupportResource) AddMessage(ctx context.Context, conversationID, body string) (*SupportConversationMessage, error) {
	var out SupportConversationMessage
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/support/conversations/" + url.PathEscape(conversationID) + "/messages",
		body:   map[string]string{"body": body},
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkRead marks the conversation read.
func (r *SupportResource) MarkRead(ctx context.Context, conversationID string) (*SupportConversation, error) {
	var out struct {
		Conversation SupportConversation `json:"conversation"`
	}
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/support/conversations/" + url.PathEscape(conversationID) + "/read",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out.Conversation, nil
}

// Close marks the conversation resolved. Either side writing again reopens it.
func (r *SupportResource) Close(ctx context.Context, conversationID string) (*SupportConversation, error) {
	var out struct {
		Conversation SupportConversation `json:"conversation"`
	}
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/support/conversations/" + url.PathEscape(conversationID) + "/close",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out.Conversation, nil
}
