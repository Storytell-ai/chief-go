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

// SessionSummary is the per-item shape in a sessions listing. Turns are
// excluded so the listing stays metadata-only.
type SessionSummary struct {
	SessionID   string    `json:"session_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
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

// SessionResponse is the full session view. Brief is the user-supplied framing;
// Turns is the transcript.
type SessionResponse struct {
	SessionID   string        `json:"session_id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Brief       string        `json:"brief,omitempty"`
	Turns       []SessionTurn `json:"turns"`
	CreatedAt   time.Time     `json:"created_at"`
	ModifiedAt  time.Time     `json:"modified_at"`
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
