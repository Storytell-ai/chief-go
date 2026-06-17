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

// SetVisibility changes a chat's access level. Setting Restricted keeps any
// audience the chat already had; switching to Project or Private clears it.
func (s *ChatsService) SetVisibility(ctx context.Context, chatID string, visibility ChatVisibility) (*ChatVisibilityResponse, error) {
	var resp ChatVisibilityResponse
	path := "/v1/chats/" + url.PathEscape(chatID) + "/visibility"
	if _, err := s.client.Do(ctx, http.MethodPost, path, &SetChatVisibilityRequest{Visibility: visibility}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListMembers returns a restricted chat's audience.
func (s *ChatsService) ListMembers(ctx context.Context, chatID string) (*ChatMemberList, error) {
	var list ChatMemberList
	path := "/v1/chats/" + url.PathEscape(chatID) + "/members"
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// AddMember adds one project member to a restricted chat's audience by email.
// Idempotent: re-adding an existing member succeeds without change. The chat's
// visibility must already be Restricted; other levels respond 409.
func (s *ChatsService) AddMember(ctx context.Context, chatID, email string) (*ChatMember, error) {
	var member ChatMember
	path := "/v1/chats/" + url.PathEscape(chatID) + "/members"
	if _, err := s.client.Do(ctx, http.MethodPost, path, &AddChatMemberRequest{Email: email}, &member); err != nil {
		return nil, err
	}
	return &member, nil
}

// RemoveMember removes one user from a restricted chat's audience. Use the
// UserID returned by ListMembers. The chat's visibility must already be
// Restricted; other levels respond 409.
func (s *ChatsService) RemoveMember(ctx context.Context, chatID, userID string) error {
	path := "/v1/chats/" + url.PathEscape(chatID) + "/members/" + url.PathEscape(userID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// CreateShareLink creates the chat's public share link. Anyone with the URL
// can read the conversation without authentication. Each chat has at most one
// active link: when one already exists it is returned unchanged; use
// RegenerateShareLink to rotate it.
func (s *ChatsService) CreateShareLink(ctx context.Context, chatID string) (*ShareLinkResponse, error) {
	var link ShareLinkResponse
	path := "/v1/chats/" + url.PathEscape(chatID) + "/share"
	if _, err := s.client.Do(ctx, http.MethodPost, path, nil, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// RegenerateShareLink revokes the chat's current share link, if any, and mints
// a new URL.
func (s *ChatsService) RegenerateShareLink(ctx context.Context, chatID string) (*ShareLinkResponse, error) {
	var link ShareLinkResponse
	path := "/v1/chats/" + url.PathEscape(chatID) + "/share?regenerate=true"
	if _, err := s.client.Do(ctx, http.MethodPost, path, nil, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// GetShareLink returns the chat's share-link status. URL and CreatedAt are
// empty when the chat isn't shared.
func (s *ChatsService) GetShareLink(ctx context.Context, chatID string) (*ShareLinkResponse, error) {
	var link ShareLinkResponse
	path := "/v1/chats/" + url.PathEscape(chatID) + "/share"
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// DeleteShareLink revokes the chat's share link. Responds 404 when no active
// link exists.
func (s *ChatsService) DeleteShareLink(ctx context.Context, chatID string) error {
	path := "/v1/chats/" + url.PathEscape(chatID) + "/share"
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
