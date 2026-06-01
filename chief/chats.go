package chief

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ChatsService is the project-scoped chat surface. Every method needs a project
// ID configured on the parent Client. Chat turns are asynchronous: Create and
// SendMessage return as soon as the workflow is accepted, so poll GetMessage
// until the prompt and response populate.
type ChatsService struct {
	client *Client
}

// Create opens a chat with its first turn.
func (s *ChatsService) Create(ctx context.Context, req *CreateChatRequest) (*CreateChatResponse, error) {
	body := *req
	if body.Scope == nil && s.client.projectID != "" {
		body.Scope = &ScopeRequest{ProjectIDs: []string{s.client.projectID}}
	}
	if body.Intelligence == "" {
		body.Intelligence = "auto"
	}
	var resp CreateChatResponse
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/chats", &body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// List returns a cursor page of the caller's chats in the configured project.
func (s *ChatsService) List(ctx context.Context, opts ...ListOption) (*ChatPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/chats"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page ChatPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns a chat's metadata.
func (s *ChatsService) Get(ctx context.Context, chatID string) (*ChatResponse, error) {
	var chat ChatResponse
	path := "/v1/chats/" + url.PathEscape(chatID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &chat); err != nil {
		return nil, err
	}
	return &chat, nil
}

// Update renames a chat. Title is the only mutable chat field.
func (s *ChatsService) Update(ctx context.Context, chatID string, req *UpdateChatRequest) (*ChatResponse, error) {
	var chat ChatResponse
	path := "/v1/chats/" + url.PathEscape(chatID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &chat); err != nil {
		return nil, err
	}
	return &chat, nil
}

// Delete removes a chat.
func (s *ChatsService) Delete(ctx context.Context, chatID string) error {
	path := "/v1/chats/" + url.PathEscape(chatID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// SendMessage appends a turn to an existing chat.
func (s *ChatsService) SendMessage(ctx context.Context, chatID string, req *SendMessageRequest) (*SendMessageResponse, error) {
	body := *req
	if body.Scope == nil && s.client.projectID != "" {
		body.Scope = &ScopeRequest{ProjectIDs: []string{s.client.projectID}}
	}
	if body.Intelligence == "" {
		body.Intelligence = "auto"
	}
	var resp SendMessageResponse
	path := "/v1/chats/" + url.PathEscape(chatID) + "/messages"
	if _, err := s.client.Do(ctx, http.MethodPost, path, &body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListMessages returns metadata for every message in a chat.
func (s *ChatsService) ListMessages(ctx context.Context, chatID string) (*MessageList, error) {
	var list MessageList
	path := "/v1/chats/" + url.PathEscape(chatID) + "/messages"
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// GetMessage returns a single message with its prompt and response. Both stay
// empty until the turn finishes; poll until the content is present.
func (s *ChatsService) GetMessage(ctx context.Context, chatID, messageID string) (*Message, error) {
	var msg Message
	path := "/v1/chats/" + url.PathEscape(chatID) + "/messages/" + url.PathEscape(messageID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DeleteMessage removes a single message from a chat.
func (s *ChatsService) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	path := "/v1/chats/" + url.PathEscape(chatID) + "/messages/" + url.PathEscape(messageID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// WaitForResponse polls a message until its turn finishes and the response
// populates. Returns the message once Response is non-empty, or an error when
// the timeout elapses first.
func (s *ChatsService) WaitForResponse(ctx context.Context, chatID, messageID string, timeout time.Duration) (*Message, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var msg *Message
	for {
		got, err := s.GetMessage(ctx, chatID, messageID)
		// The message row lands a beat after the turn is accepted, so a 404 means
		// not ready yet rather than a real failure.
		if err != nil && !IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			msg = got
			if msg.Response != "" {
				return msg, nil
			}
		}

		if time.Now().After(deadline) {
			return msg, fmt.Errorf("message %s has no response after %s", messageID, timeout)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
