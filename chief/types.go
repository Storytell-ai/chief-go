package chief

import "time"

// AssetStatus is the ingest lifecycle reported by the API. Progress is
// monotonic until it reaches Ready or Failed.
type AssetStatus string

// Ready and Failed are terminal.
const (
	AssetStatusUploaded  AssetStatus = "uploaded"
	AssetStatusIngesting AssetStatus = "ingesting"
	AssetStatusReady     AssetStatus = "ready"
	AssetStatusFailed    AssetStatus = "failed"
)

// CreateAssetRequest is the body for minting an asset row plus a signed upload
// URL. When MD5 is set and an asset with that content already exists in the
// project, the server returns it instead of a new upload URL.
type CreateAssetRequest struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	MD5      string `json:"md5,omitempty"`
}

// CreateAssetResponse has two shapes. When AlreadyExists is false it carries a
// signed upload target: UploadHeaders must be applied verbatim to the upload
// request and ExpiresAt is the deadline after which the URL is rejected. When
// AlreadyExists is true the content was already present; the upload fields are
// empty and Status reflects the existing asset's ingest state.
type CreateAssetResponse struct {
	AssetID       string            `json:"asset_id"`
	AlreadyExists bool              `json:"already_exists"`
	Status        AssetStatus       `json:"status,omitempty"`
	UploadURL     string            `json:"upload_url,omitempty"`
	UploadMethod  string            `json:"upload_method,omitempty"`
	UploadHeaders map[string]string `json:"upload_headers,omitempty"`
	ExpiresAt     time.Time         `json:"expires_at,omitzero"`
}

// Asset is an asset's current ingest state.
type Asset struct {
	AssetID     string      `json:"asset_id"`
	Status      AssetStatus `json:"status"`
	Filename    string      `json:"filename"`
	MimeType    string      `json:"mime_type"`
	SizeInBytes int64       `json:"size_in_bytes"`
	CreatedAt   time.Time   `json:"created_at"`
}

// LabelSummary is a label attached to an asset.
type LabelSummary struct {
	LabelID string `json:"label_id"`
	Name    string `json:"name"`
}

// AssetSummary is the per-item shape in a list response, with attached labels
// inline.
type AssetSummary struct {
	AssetID     string         `json:"asset_id"`
	Status      AssetStatus    `json:"status"`
	Filename    string         `json:"filename"`
	MimeType    string         `json:"mime_type"`
	SizeInBytes int64          `json:"size_in_bytes"`
	CreatedAt   time.Time      `json:"created_at"`
	Labels      []LabelSummary `json:"labels"`
}

// AssetPage is one page of a cursor-paginated asset listing. FirstID and
// LastID feed the after_id / before_id cursors; HasMore signals another page
// follows.
type AssetPage struct {
	Data    []AssetSummary `json:"data"`
	FirstID string         `json:"first_id"`
	LastID  string         `json:"last_id"`
	HasMore bool           `json:"has_more"`
}

// CreateLabelRequest is the body for minting a label. Color, when set, must be
// a 6-digit hex code like #6b7280; the server applies a default when omitted.
type CreateLabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// AttachLabelRequest attaches a label to an asset by name. A name with no
// matching label is auto-created as a bare label (no color or icon).
type AttachLabelRequest struct {
	LabelName string `json:"label_name"`
}

// LabelPage is one page of a label listing. The per-project label cap keeps the
// listing to a single page, so HasMore is always false today.
type LabelPage struct {
	Data    []LabelSummary `json:"data"`
	FirstID string         `json:"first_id"`
	LastID  string         `json:"last_id"`
	HasMore bool           `json:"has_more"`
}

// ActionRequest is the body for creating or updating an action. Prompt is
// plain text; the server base64-encodes it for storage. Update is
// full-replacement: a schedule, trigger, scope, or email omitted from the body
// is cleared. Enabled is honored on update to pause or resume; create always
// starts enabled.
type ActionRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Prompt      string           `json:"prompt"`
	Schedule    *ScheduleRequest `json:"schedule,omitempty"`
	Trigger     *TriggerRequest  `json:"trigger,omitempty"`
	Scope       *ScopeRequest    `json:"scope,omitempty"`
	Email       *EmailOutcome    `json:"email,omitempty"`
	Enabled     *bool            `json:"enabled,omitempty"`
}

// ScheduleRequest is a recurring run schedule expressed as cron-style fields.
// Each field accepts the cron value for its position; Timezone is an IANA name.
type ScheduleRequest struct {
	Hour     string `json:"hour"`
	Weekday  string `json:"weekday"`
	MonthDay string `json:"month_day"`
	Timezone string `json:"timezone"`
}

// TriggerRequest fires an action on an event rather than a schedule. CreatedBy
// narrows the trigger to events originating from the listed users.
type TriggerRequest struct {
	Kind      string   `json:"kind"`
	CreatedBy []string `json:"created_by,omitempty"`
}

// ScopeRequest bounds the data an action operates over. An empty scope means
// the action sees the whole project.
type ScopeRequest struct {
	AssetIDs   []string `json:"asset_ids,omitempty"`
	ChatIDs    []string `json:"chat_ids,omitempty"`
	LabelIDs   []string `json:"label_ids,omitempty"`
	ConceptIDs []string `json:"concept_ids,omitempty"`
	ProjectIDs []string `json:"project_ids,omitempty"`
	ViewIDs    []string `json:"view_ids,omitempty"`
}

// EmailOutcome delivers an action's result by email to the listed recipients.
type EmailOutcome struct {
	To                   []string `json:"to"`
	Subject              string   `json:"subject,omitempty"`
	IncludeDateInSubject bool     `json:"include_date_in_subject,omitempty"`
}

// ActionResponse is an action's full state as returned by create, get, update,
// enable, and disable.
type ActionResponse struct {
	ActionID    string           `json:"action_id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Prompt      string           `json:"prompt"`
	Schedule    *ScheduleRequest `json:"schedule,omitempty"`
	Trigger     *TriggerRequest  `json:"trigger,omitempty"`
	Scope       *ScopeRequest    `json:"scope,omitempty"`
	Email       *EmailOutcome    `json:"email,omitempty"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   time.Time        `json:"created_at"`
	ModifiedAt  time.Time        `json:"modified_at"`
}

// ActionSummary is the per-item shape in an action list response.
type ActionSummary struct {
	ActionID  string    `json:"action_id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// ActionPage is the action listing. The endpoint returns every action in one
// response, so HasMore is always false today.
type ActionPage struct {
	Data    []ActionSummary `json:"data"`
	FirstID string          `json:"first_id"`
	LastID  string          `json:"last_id"`
	HasMore bool            `json:"has_more"`
}

// UpdateAssetRequest patches an asset's mutable metadata. Both fields are
// optional; omitting one leaves it unchanged.
type UpdateAssetRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// LabelResponse is the full label view returned by get and update.
type LabelResponse struct {
	LabelID string `json:"label_id"`
	Name    string `json:"name"`
	Color   string `json:"color,omitempty"`
	Icon    string `json:"icon,omitempty"`
}

// UpdateLabelRequest patches a label's display metadata. Each field is
// optional; omitting one leaves it unchanged. Color, when set, must be a
// 6-digit hex code.
type UpdateLabelRequest struct {
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
	Icon  *string `json:"icon,omitempty"`
}

// UpdateSessionRequest patches a session's name and/or description. Each field
// is optional; omitting one leaves it unchanged.
type UpdateSessionRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// Lifecycle values a SessionState can carry. The server may add to the set, so
// treat an unrecognized value as data rather than an error.
const (
	SessionStateScheduled = "session.scheduled"
	SessionStateStarted   = "session.started"
	SessionStateEnded     = "session.ended"
)

// SessionState is where a session sits in its lifecycle and when it got there.
// The timestamps are the call's own clock, unlike a session's CreatedAt, which
// is when the row was opened — a session scheduled from a calendar exists long
// before it starts. Each is nil until the session reaches that point, and State
// is empty against a server predating the field.
type SessionState struct {
	State       string     `json:"state"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	// CalendarEventID is set when the session was scheduled from a calendar
	// event, empty for an ad-hoc session. The API does not expose the call's
	// join link.
	CalendarEventID string `json:"calendar_event_id,omitempty"`
}

// SessionSummary is the per-item shape in a sessions listing. Turns are
// excluded so the listing stays metadata-only.
type SessionSummary struct {
	SessionID   string       `json:"session_id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	State       SessionState `json:"state"`
	CreatedAt   time.Time    `json:"created_at"`
	ModifiedAt  time.Time    `json:"modified_at"`
}

// SessionPage is one page of a session listing.
type SessionPage struct {
	Data    []SessionSummary `json:"data"`
	FirstID string           `json:"first_id"`
	LastID  string           `json:"last_id"`
	HasMore bool             `json:"has_more"`
}

// SessionTurn is one transcribed segment of a session. Start and End are
// offsets in seconds.
type SessionTurn struct {
	Speaker int     `json:"speaker"`
	Source  string  `json:"source"`
	Text    string  `json:"text"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
}

// SessionLiveSummary is the running summary the model keeps as a session
// unfolds.
type SessionLiveSummary struct {
	Headline string                   `json:"headline,omitempty"`
	Items    []SessionLiveSummaryItem `json:"items"`
}

// Kind and State values a SessionLiveSummaryItem can carry. The server may add
// to either set, so treat an unrecognized value as data rather than an error.
const (
	SummaryKindContext  = "context"
	SummaryKindDecision = "decision"
	SummaryKindTodo     = "todo"

	SummaryStateOpen      = "open"
	SummaryStateApproved  = "approved"
	SummaryStateDismissed = "dismissed"
	SummaryStateDone      = "done"
)

// SessionLiveSummaryItem is one entry in the live summary. State records the
// user's commitment rather than the model's — a dismissed item is an explicit
// human call, not a model retraction. ID is stable for the life of the session
// and survives the model rewording Text, so it keys an item across successive
// snapshots. FirstSeenSec is an offset in seconds.
type SessionLiveSummaryItem struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Topic    string `json:"topic,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Text     string `json:"text"`
	// Owner is a participant name heard in the transcript, empty when nobody was
	// named. It is never derived from the audio streams.
	Owner        string  `json:"owner,omitempty"`
	State        string  `json:"state"`
	Active       bool    `json:"active"`
	FirstSeenSec float64 `json:"first_seen_sec"`
}

// SessionResponse is the full session view. Brief is the user-supplied framing;
// Turns is the transcript. LiveSummary is nil only against a server predating
// the field — a session that has no summary yet comes back with one whose Items
// are empty, so nil and empty are not the same answer.
type SessionResponse struct {
	SessionID   string `json:"session_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Brief       string `json:"brief,omitempty"`
	// Language is the transcription code the session was recorded in: "en-US",
	// "multi" (auto-detect), or a bare "es", "fr", "de", "hi", "it", "ja", "nl",
	// "pt", "ru".
	Language string       `json:"language,omitempty"`
	State    SessionState `json:"state"`
	// Summary and ActionItems are legacy: they carry the separate post-session
	// writeup that sessions recorded before the live summary became canonical
	// were given, and are empty for every session recorded since. LiveSummary is
	// where a current session's decisions and to-dos live.
	Summary     string              `json:"summary,omitempty"`
	ActionItems []string            `json:"action_items,omitempty"`
	Turns       []SessionTurn       `json:"turns"`
	LiveSummary *SessionLiveSummary `json:"live_summary,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	ModifiedAt  time.Time           `json:"modified_at"`
}

// CreateSkillRequest mints a skill. Scope is "project" or "user" ("system" is
// compiled and read-only). Category is "skill" or "persona".
type CreateSkillRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
	Icon        string `json:"icon,omitempty"`
	Scope       string `json:"scope"`
	Category    string `json:"category"`
}

// UpdateSkillRequest patches a skill. Each field is optional; omitting one
// leaves it unchanged. Category, when set, must be "skill" or "persona".
type UpdateSkillRequest struct {
	Name        *string `json:"name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	Description *string `json:"description,omitempty"`
	Content     *string `json:"content,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	Category    *string `json:"category,omitempty"`
}

// SkillResponse is the full skill view. Enabled reflects the caller's
// preference.
type SkillResponse struct {
	SkillID     string `json:"skill_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
	Icon        string `json:"icon,omitempty"`
	Scope       string `json:"scope"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
}

// SkillPage is one page of a skill listing.
type SkillPage struct {
	Data    []SkillResponse `json:"data"`
	FirstID string          `json:"first_id"`
	LastID  string          `json:"last_id"`
	HasMore bool            `json:"has_more"`
}

// CreateMemoryRequest stores a memory. Scope is optional and, for now, only
// accepts empty or "project". Category is one of: identity, preference, fact,
// context, instruction.
type CreateMemoryRequest struct {
	Content    string `json:"content"`
	Category   string `json:"category"`
	Importance int    `json:"importance"`
	Scope      string `json:"scope,omitempty"`
}

// UpdateMemoryRequest updates a memory. Content is replaced; category and
// importance are optional patches.
type UpdateMemoryRequest struct {
	Content    string  `json:"content"`
	Category   *string `json:"category,omitempty"`
	Importance *int    `json:"importance,omitempty"`
}

// MemoryResponse is the full memory view.
type MemoryResponse struct {
	MemoryID   string    `json:"memory_id"`
	Scope      string    `json:"scope"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`
	Importance int       `json:"importance"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

// MemoryPage is one page of a memory listing.
type MemoryPage struct {
	Data    []MemoryResponse `json:"data"`
	FirstID string           `json:"first_id"`
	LastID  string           `json:"last_id"`
	HasMore bool             `json:"has_more"`
}

// Project is the public wire shape for a project the caller can access.
type Project struct {
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default"`
}

// ProjectPage is one page of the project listing.
type ProjectPage struct {
	Data    []Project `json:"data"`
	FirstID string    `json:"first_id"`
	LastID  string    `json:"last_id"`
	HasMore bool      `json:"has_more"`
}

// CreateProjectRequest mints a project. It lands in the org and workspace of
// the caller's root grant; billing is not caller-settable.
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateProjectRequest replaces a project's two mutable fields. It is a full
// set, not a patch: an empty Description clears it.
type UpdateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ProjectMember is one user holding a grant in a project. Role is the internal
// grant name (role.owner, role.collaborator, role.reader); JoinMethod is how
// the grant was gained (joined.email, joined.domain, joined.link).
type ProjectMember struct {
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name,omitempty"`
	Role       string    `json:"role"`
	JoinMethod string    `json:"join_method"`
	AddedAt    time.Time `json:"added_at"`
}

// ProjectMemberList is a project's full participant list.
type ProjectMemberList struct {
	Data []ProjectMember `json:"data"`
}

// CreateProjectInvitationRequest invites one user to the project by email.
// Role is the short public name: "collaborator", "reader", or "owner".
type CreateProjectInvitationRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// ProjectInvitationResponse is the created invitation. Role echoes the short
// public name from the request. URL is the shareable accept link — the same
// one sent in the invitation email.
type ProjectInvitationResponse struct {
	InvitationID string    `json:"invitation_id"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateChatRequest opens a chat with its first turn. Intelligence selects a
// mode — "fast", "expert", or "research"; an empty value is sent as "auto".
// Provider biases vendor choice — "automatic", "anthropic", "openai", or
// "google" — and empty uses the router's default order. PublicData nil follows
// the mode default; false disables public-web search. A nil Scope defaults to
// the configured project; set Scope to narrow to specific assets, chats, or
// labels.
type CreateChatRequest struct {
	Prompt       string        `json:"prompt"`
	Intelligence string        `json:"intelligence,omitempty"`
	Provider     string        `json:"provider,omitempty"`
	Skills       []string      `json:"skills,omitempty"`
	PublicData   *bool         `json:"public_data,omitempty"`
	Scope        *ScopeRequest `json:"scope,omitempty"`
}

// SendMessageRequest appends a turn to an existing chat. The tuning fields
// carry the same semantics as on CreateChatRequest.
type SendMessageRequest struct {
	Prompt       string        `json:"prompt"`
	Intelligence string        `json:"intelligence,omitempty"`
	Provider     string        `json:"provider,omitempty"`
	Skills       []string      `json:"skills,omitempty"`
	PublicData   *bool         `json:"public_data,omitempty"`
	Scope        *ScopeRequest `json:"scope,omitempty"`
}

// UpdateChatRequest renames a chat. Title is the only mutable chat field and
// the server rejects an empty value.
type UpdateChatRequest struct {
	Title string `json:"title"`
}

// CreateChatResponse acknowledges an accepted chat turn. The turn runs
// asynchronously: poll GetMessage with MessageID until its prompt and response
// populate.
type CreateChatResponse struct {
	ChatID    string    `json:"chat_id"`
	MessageID string    `json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SendMessageResponse acknowledges an accepted follow-up turn. The turn runs
// asynchronously: poll GetMessage with MessageID until its prompt and response
// populate.
type SendMessageResponse struct {
	MessageID string    `json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ChatResponse is chat-level metadata. ModifiedAt is nil until the chat has a
// turn. Visibility and CanManageVisibility are populated on the single-chat
// read.
type ChatResponse struct {
	ChatID              string         `json:"chat_id"`
	ModifiedAt          *time.Time     `json:"modified_at,omitempty"`
	Visibility          ChatVisibility `json:"visibility,omitempty"`
	CanManageVisibility *bool          `json:"can_manage_visibility,omitempty"`
}

// ChatSummary is the per-item shape in a chat listing.
type ChatSummary struct {
	ChatID     string         `json:"chat_id"`
	Title      string         `json:"title,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	ModifiedAt *time.Time     `json:"modified_at,omitempty"`
	Visibility ChatVisibility `json:"visibility,omitempty"`
}

// ChatPage is one page of a chat listing.
type ChatPage struct {
	Data    []ChatSummary `json:"data"`
	FirstID string        `json:"first_id"`
	LastID  string        `json:"last_id"`
	HasMore bool          `json:"has_more"`
}

// ChatVisibility is a chat's access level: Project lets every project member
// read the chat, Restricted limits reads to the owner plus the audience
// managed under the chat's members, and Private is owner-only.
type ChatVisibility string

// The three levels the API accepts.
const (
	ChatVisibilityProject    ChatVisibility = "project"
	ChatVisibilityRestricted ChatVisibility = "restricted"
	ChatVisibilityPrivate    ChatVisibility = "private"
)

// SetChatVisibilityRequest changes a chat's access level. Setting Restricted
// keeps any audience the chat already had; switching to Project or Private
// clears it.
type SetChatVisibilityRequest struct {
	Visibility ChatVisibility `json:"visibility"`
}

// ChatVisibilityResponse reports the chat's access level after a change.
type ChatVisibilityResponse struct {
	ChatID     string         `json:"chat_id"`
	Visibility ChatVisibility `json:"visibility"`
}

// ChatMember is one user in a restricted chat's audience. Audience membership
// only narrows who can read the chat; it never grants project access.
type ChatMember struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name,omitempty"`
}

// ChatMemberList is a restricted chat's full audience.
type ChatMemberList struct {
	Members []ChatMember `json:"members"`
}

// AddChatMemberRequest adds one user to a restricted chat's audience by email.
// The email must resolve to a current member of the chat's project.
type AddChatMemberRequest struct {
	Email string `json:"email"`
}

// ShareLinkResponse is a chat's public share-link status. URL and CreatedAt
// are empty when the chat isn't shared.
type ShareLinkResponse struct {
	IsShared  bool       `json:"is_shared"`
	URL       string     `json:"url,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// Message is one turn: the user prompt and the assistant response under a
// single id. Prompt and Response stay empty until the async turn finishes
// writing its recording; there is no status field, so poll GetMessage until
// the content is present. The credit fields are nil until the turn's debit
// posts, which happens after the workflow completes.
type Message struct {
	ID             string     `json:"id"`
	Prompt         string     `json:"prompt,omitempty"`
	Response       string     `json:"response,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	IngressCredits *int64     `json:"ingress_credits,omitempty"`
	EgressCredits  *int64     `json:"egress_credits,omitempty"`
	TotalCredits   *int64     `json:"total_credits,omitempty"`
}

// MessageSummary is the per-item shape in a message listing. Content is fetched
// separately so the listing stays small.
type MessageSummary struct {
	ID        string     `json:"id"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// MessageList is a chat's full message listing. It's a wrapper rather than a
// bare slice because the endpoint is unpaginated today and the wire shape gains
// a cursor field additively when pagination lands, keeping that change
// non-breaking.
type MessageList struct {
	Messages []MessageSummary `json:"messages"`
}
