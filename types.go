package driftstack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// This file mirrors the Zod schemas in `packages/api-types/`. The
// schemas are the source of truth (Zod → OpenAPI 3.1 → these types).
// Re-generated when schemas change; tracked manually for now since
// oapi-codegen lacks OpenAPI 3.1 support (see the CHANGELOG for the
// codegen-vs-hand-written decision).
//
// Naming follows the Stripe-Go convention: PascalCase exported types,
// json tags using the underscore_case names the wire uses, omitempty
// on optional fields so customers can construct partial inputs.

// ──────────────────────────────────────────────────────────────────
// Common / shared
// ──────────────────────────────────────────────────────────────────

// AccountTier is the closed enum of plans an account can hold: the plans
// sold since 2026-09 (a 7-day trial, then Starter, Pro, Team and Scale, and
// Enterprise), and the earlier two-ladder plans (free, the manual plans and
// the API plans), which stay valid for accounts that hold them. The
// `trial_pack` tier was retired on 2026-05-27; those accounts read as `free`.
type AccountTier string

const (
	TierFree         AccountTier = "free"
	TierSoloManual   AccountTier = "solo_manual"
	TierTeamManual   AccountTier = "team_manual"
	TierAgencyManual AccountTier = "agency_manual"
	TierAPIStarter   AccountTier = "api_starter"
	TierAPIBuilder   AccountTier = "api_builder"
	TierAPIScale     AccountTier = "api_scale"
	TierEnterprise   AccountTier = "enterprise"
)

// The plans sold since 2026-09. Their ids are new strings: none reuses an id
// from an earlier plan, so a program comparing against an old constant never
// matches a new plan by accident.
const (
	// TierTrial is the 7-day free trial a new account starts on. When it
	// ends the account can still sign in and read everything, but cannot
	// start sessions until it subscribes (see AccountSelfProfile.TrialEnded).
	TierTrial AccountTier = "trial"
	// TierStarterV3 is the Starter plan.
	TierStarterV3 AccountTier = "starter_v3"
	// TierProV3 is the Pro plan.
	TierProV3 AccountTier = "pro_v3"
	// TierTeamV3 is the Team plan (not the earlier TierTeamManual).
	TierTeamV3 AccountTier = "team_v3"
	// TierScaleV3 is the Scale plan (not the deprecated TierScale).
	TierScaleV3 AccountTier = "scale_v3"
)

// The tier names below belong to the single pricing ladder that ran until
// 2026-05-05. They are restored so a program written against v0.1.6 still
// compiles, and will be removed in a later minor release. No account is on
// any of them: the server never returns these values, so a comparison
// against one is always false. Replace each with the constant its notice
// names.
const (
	// Deprecated: the Starter plan is now API Starter.
	// Use TierAPIStarter.
	TierStarter AccountTier = "starter"
	// Deprecated: the Solo plan was split across the manual and API ladders
	// and has no single successor. Use TierSoloManual or TierTeamManual,
	// whichever matches what the account pays for.
	TierSolo AccountTier = "solo"
	// Deprecated: the Builder plan is now API Builder.
	// Use TierAPIBuilder.
	TierBuilder AccountTier = "builder"
	// Deprecated: the Scale plan is now API Scale.
	// Use TierAPIScale.
	TierScale AccountTier = "scale"
)

// AccountStatus.
type AccountStatus string

const (
	AccountActive    AccountStatus = "active"
	AccountSuspended AccountStatus = "suspended"
	AccountDeleted   AccountStatus = "deleted"
)

// SessionStatus is the lifecycle state of a session.
type SessionStatus string

const (
	SessionCreating  SessionStatus = "creating"
	SessionReady     SessionStatus = "ready"
	SessionBusy      SessionStatus = "busy"
	SessionDestroyed SessionStatus = "destroyed"
	SessionErrored   SessionStatus = "errored"
)

// SessionPurpose declares what a session is for; it selects the browser
// driver the session runs on. PurposeProductionCustomer is the only value a
// customer session uses, and the default when Purpose is left empty.
type SessionPurpose string

// PurposeProductionCustomer is the only purpose the API reference offers.
const PurposeProductionCustomer SessionPurpose = "production_customer"

// DefaultSessionPurpose matches packages/api-types DEFAULT_SESSION_PURPOSE.
const DefaultSessionPurpose = PurposeProductionCustomer

// BehavioralProfile selects the human-behaviour persona a session uses
// when it taps, scrolls and types. These are the only values the
// server's BehavioralProfileSchema accepts.
type BehavioralProfile string

const (
	PersonaCasual    BehavioralProfile = "casual"
	PersonaRegular   BehavioralProfile = "regular"
	PersonaPowerUser BehavioralProfile = "power_user"
)

// DefaultBehavioralProfile matches packages/api-types DEFAULT_BEHAVIORAL_PROFILE.
const DefaultBehavioralProfile = PersonaRegular

// WebhookEventType — closed enum of supported webhook events.
type WebhookEventType string

const (
	EventSessionCompleted WebhookEventType = "session.completed"
	EventSessionFailed    WebhookEventType = "session.failed"
	EventAPIKeyRevoked    WebhookEventType = "api_key.revoked"
	// Fired when a SOCKS5 session reports what its proxy can do;
	// subscribe to react to proxy-health changes without polling.
	EventSessionEgressCapabilityChanged WebhookEventType = "session.egress_capability_changed"
	// A synthetic test event sent only via
	// POST /v1/webhooks/:id/test. Customers cannot subscribe to it
	// (the create / update Zod schemas reject it); it's dispatched
	// regardless of subscription so customers can verify their
	// handler signature-checks correctly before relying on real events.
	EventTestPing WebhookEventType = "test.ping"
	// Crypto-order terminal transitions, fired when an order moves from
	// pending/confirming/partial to paid or failed. Subscribe to settle
	// crypto checkouts in your own accounting.
	EventCryptoOrderPaid   WebhookEventType = "crypto.order.paid"
	EventCryptoOrderFailed WebhookEventType = "crypto.order.failed"
	// Fired when a session meets a bot-check challenge
	// (DataDome/Arkose/PerimeterX/AWS-WAF/GeeTest/…). Subscribe to route
	// challenge alerts into your own on-call surface; the session pauses
	// itself and waits for you to resume it.
	EventSessionChallengeDetected WebhookEventType = "session.challenge_detected"
	// Saving the profile back failed as the session ended (terminal; the
	// session itself succeeded). Subscribe if you depend on profile state,
	// so you learn that the next restore will be stale.
	EventSessionProfileSaveFailed WebhookEventType = "session.profile_save_failed"
)

// The two event names below are restored so a program written against
// v0.1.6 still compiles, and will be removed in a later minor release.
// Neither event is sent any more and neither can be subscribed to — the
// create and update schemas reject them — and no event replaced them.
// Read quota headroom from Usage.CurrentPeriod instead.
const (
	// Deprecated: quota warnings are no longer delivered by webhook.
	// Read Quotas from Usage.CurrentPeriod instead.
	EventQuotaWarning80Pct WebhookEventType = "quota.warning_80pct"
	// Deprecated: quota-exceeded is no longer delivered by webhook.
	// Read Quotas from Usage.CurrentPeriod instead.
	EventQuotaExceeded WebhookEventType = "quota.exceeded"
)

// WebhookDeliveryStatus.
type WebhookDeliveryStatus string

const (
	DeliveryPending   WebhookDeliveryStatus = "pending"
	DeliveryInFlight  WebhookDeliveryStatus = "in_flight"
	DeliveryDelivered WebhookDeliveryStatus = "delivered"
	DeliveryFailed    WebhookDeliveryStatus = "failed"
	DeliveryDLQ       WebhookDeliveryStatus = "dlq"
)

// UsageRecordType.
type UsageRecordType string

const (
	UsageSessionMinute     UsageRecordType = "session_minute"
	UsageNavigate          UsageRecordType = "navigate"
	UsageInteract          UsageRecordType = "interact"
	UsageWait              UsageRecordType = "wait"
	UsageStateCapture      UsageRecordType = "state_capture"
	UsageScreenshotCapture UsageRecordType = "screenshot_capture"
)

// ──────────────────────────────────────────────────────────────────
// Account
// ──────────────────────────────────────────────────────────────────

type Account struct {
	ID        string        `json:"id"`
	Email     string        `json:"email"`
	Name      *string       `json:"name"`
	Tier      AccountTier   `json:"tier"`
	Status    AccountStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ──────────────────────────────────────────────────────────────────
// Session
// ──────────────────────────────────────────────────────────────────

type Session struct {
	ID                 string              `json:"id"`
	AccountID          string              `json:"account_id"`
	APIKeyID           string              `json:"api_key_id"`
	Status             SessionStatus       `json:"status"`
	Archetype          string              `json:"archetype"`
	Purpose            SessionPurpose      `json:"purpose"`
	Label              *string             `json:"label"`
	Metadata           map[string]any      `json:"metadata"`
	EgressCapabilities *EgressCapabilities `json:"egress_capabilities"`
	// The raw egress report exactly as the session sent it, kept
	// beside the derived EgressCapabilities view so a newer
	// report shape is never lost. Opaque map; prefer
	// EgressCapabilities for typed access. Null until the
	// session reports.
	EgressCapabilityReport map[string]any `json:"egress_capability_report"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	LastStateAt            *time.Time     `json:"last_state_at"`
	DestroyedAt            *time.Time     `json:"destroyed_at"`
	// ArchetypeSource — where Archetype came from: "explicit" (the create
	// named it), "profile" (the launched profile's device) or "random" (the
	// create named neither, so the session runs as a fresh visitor on a random
	// current iPhone the plan includes; drawn once at create, never changed).
	// Nil on a session created before the server recorded it, and from older
	// servers.
	ArchetypeSource *string `json:"archetype_source,omitempty"`
	// ProxyID is the saved proxy this session's traffic goes out through: the
	// create's ProxyID, or the proxy the launched profile is bound to. Usually
	// one of the account's own proxies (Egress.ListProxies); a session a team
	// admin started with a proxy saved on the admin's own account reports that
	// admin's proxy. Set from the create response onwards and kept after the
	// session ends.
	//
	// nil means one of two things, and ProxyIDReported tells them apart: with
	// ProxyIDReported true the session was started without a saved proxy (the
	// server sent proxy_id: null); with it false the proxy was not reported (the
	// key was absent: a read by a team member without admin role, a read that
	// could not look it up at that moment, or an older server). Never treat a
	// nil ProxyID with ProxyIDReported false as "no proxy".
	ProxyID *string `json:"proxy_id,omitempty"`
	// ProxyIDReported is true when the response carried the proxy_id key,
	// null or not; see ProxyID. Set when a Session is decoded from JSON.
	ProxyIDReported bool `json:"-"`
}

// UnmarshalJSON decodes a Session and records whether the proxy_id key was
// present (ProxyIDReported), which a *string alone cannot: an absent key and
// a null both leave ProxyID nil.
func (s *Session) UnmarshalJSON(data []byte) error {
	type plainSession Session
	var decoded plainSession
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	_, decoded.ProxyIDReported = keys["proxy_id"]
	*s = Session(decoded)
	return nil
}

// EgressCapabilities is what a session reports about its SOCKS5 proxy:
// which capabilities that proxy actually offers. Null until the session
// reports `egress.capability_report`; non-SOCKS5 sessions stay null
// permanently.
type EgressCapabilities struct {
	UDPAssociate     bool   `json:"udp_associate"`
	QUICRoute        string `json:"quic_route"` // "proxy" | "direct" | "disabled"
	DNSRemoteResolve bool   `json:"dns_remote_resolve"`
	// Safeguards is "passed", "failed", or "unverified" — whether every
	// defence-in-depth egress safeguard held for this session. nil means the
	// row predates this field (the key was absent on the wire); never treat a
	// nil Safeguards as "unverified" or "passed".
	Safeguards *string  `json:"safeguards,omitempty"`
	Warnings   []string `json:"warnings"`
}

// CreateSessionRequest. All fields are optional; leave empty to let the
// server default (Archetype → a random current iPhone your plan includes,
// different from one session to the next, unless ProfileID names a profile,
// whose device is used — the session's ArchetypeSource says which;
// Purpose → DefaultSessionPurpose, BehavioralProfile →
// DefaultBehavioralProfile).
type CreateSessionRequest struct {
	Archetype string         `json:"archetype,omitempty"`
	Purpose   SessionPurpose `json:"purpose,omitempty"`
	Label     string         `json:"label,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	// ProfileID binds the session to a persistent antidetect profile
	// (cookies/localStorage/archetype inherited). Optional.
	ProfileID string `json:"profile_id,omitempty"`
	// BehavioralProfile selects the per-session persona (2026-06-05).
	BehavioralProfile BehavioralProfile `json:"behavioral_profile,omitempty"`
	// ProxyID is the id of one of your saved proxies; the session's traffic
	// goes out through it (2026-10-02). A deployment that requires a proxy of
	// your own refuses a create without one with a 422 *ProxyRequiredError;
	// from a profile bound to a proxy, that proxy is used when this is empty.
	ProxyID string `json:"proxy_id,omitempty"`
}

// CreateSessionResponse mirrors the server's POST /v1/sessions
// response: it's a Session row.
type CreateSessionResponse = Session

type SessionsListPage struct {
	Data       []Session `json:"data"`
	HasMore    bool      `json:"has_more"`
	NextCursor *string   `json:"next_cursor"`
}

type ListSessionsQuery struct {
	Limit  int    `url:"limit,omitempty"`
	Cursor string `url:"cursor,omitempty"`
}

type NavigateRequest struct {
	URL string `json:"url"`
	// When the call returns: "load" (the server default when omitted),
	// "domcontentloaded" or "networkidle". "networkidle" is accepted and
	// currently waits for the load event, the same as "load".
	WaitUntil string `json:"wait_until,omitempty"`
	// Per-call timeout in ms. Server clamps to 1000–120000. Zero/omit
	// = server default (currently 30s).
	TimeoutMS int `json:"timeout_ms,omitempty"`
}

type NavigateResponse struct {
	URL        string `json:"url"`
	Status     int    `json:"status"`
	FinalURL   string `json:"final_url"`
	DurationMS int    `json:"duration_ms"`
}

// InteractAction is a discriminated-union of action kinds. Use the
// constructors (NewTapAction, NewTypeAction, ...) to build one.
//
// Actions name their target by selector; the API has no coordinate taps.
type InteractAction struct {
	Kind     string `json:"kind"`               // tap | type | scroll | press
	Selector string `json:"selector,omitempty"` // tap, type, scroll
	Text     string `json:"text,omitempty"`     // type
	DelayMs  *int   `json:"delay_ms,omitempty"` // type
	// Sensitive marks the typed value (card number / OTP / PIN) so the
	// session does not act out visible typo-corrections while typing it.
	Sensitive *bool  `json:"sensitive,omitempty"` // type
	DeltaX    int    `json:"delta_x,omitempty"`   // scroll
	DeltaY    int    `json:"delta_y,omitempty"`   // scroll
	Key       string `json:"key,omitempty"`       // press
}

func NewTapAction(selector string) InteractAction {
	return InteractAction{Kind: "tap", Selector: selector}
}

func NewTypeAction(selector, text string) InteractAction {
	return InteractAction{Kind: "type", Selector: selector, Text: text}
}

// NewScrollAction scrolls the viewport (or selected element) by the
// given pixel deltas. Positive Y scrolls down.
func NewScrollAction(deltaX, deltaY int) InteractAction {
	return InteractAction{Kind: "scroll", DeltaX: deltaX, DeltaY: deltaY}
}

func NewPressAction(key string) InteractAction {
	return InteractAction{Kind: "press", Key: key}
}

type InteractRequest struct {
	Action    InteractAction `json:"action"`
	TimeoutMS int            `json:"timeout_ms,omitempty"`
}

type InteractResponse struct {
	OK         bool `json:"ok"`
	DurationMS int  `json:"duration_ms"`
}

// WaitCondition is a discriminated-union of wait conditions. Use the
// constructors (NewSelectorCondition, ...) to build one.
type WaitCondition struct {
	Kind     string `json:"kind"` // selector | selector_hidden | url_matches | time
	Selector string `json:"selector,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	MS       int    `json:"ms,omitempty"`
}

func NewSelectorCondition(selector string) WaitCondition {
	return WaitCondition{Kind: "selector", Selector: selector}
}

func NewSelectorHiddenCondition(selector string) WaitCondition {
	return WaitCondition{Kind: "selector_hidden", Selector: selector}
}

func NewURLMatchesCondition(pattern string) WaitCondition {
	return WaitCondition{Kind: "url_matches", Pattern: pattern}
}

func NewTimeCondition(ms int) WaitCondition {
	return WaitCondition{Kind: "time", MS: ms}
}

type WaitRequest struct {
	Condition WaitCondition `json:"condition"`
	TimeoutMS int           `json:"timeout_ms,omitempty"`
}

type WaitResponse struct {
	Satisfied  bool `json:"satisfied"`
	DurationMS int  `json:"duration_ms"`
}

// PageStateError describes a failed navigation as the browser saw it.
type PageStateError struct {
	Kind       string `json:"kind"` // http | tls | dns | net | timeout
	HTTPStatus *int   `json:"http_status,omitempty"`
	Message    string `json:"message"`
}

// PageState is the page lifecycle: loading | loaded | errored,
// with Error present only when errored. Nil on SessionState until the
// session reports a lifecycle event.
type PageState struct {
	State string          `json:"state"` // loading | loaded | errored
	Error *PageStateError `json:"error,omitempty"`
}

type SessionState struct {
	URL          *string           `json:"url"`
	Title        *string           `json:"title"`
	Cookies      []map[string]any  `json:"cookies"`
	LocalStorage map[string]string `json:"local_storage"`
	PageState    *PageState        `json:"page_state"`
	CapturedAt   time.Time         `json:"captured_at"`
}

// CaptureKind enumerates the supported capture outputs.
type CaptureKind string

const (
	CaptureScreenshot  CaptureKind = "screenshot"
	CaptureDOMSnapshot CaptureKind = "dom_snapshot"
	CapturePDF         CaptureKind = "pdf"
)

type CaptureRequest struct {
	Kind     CaptureKind `json:"kind"`
	FullPage bool        `json:"full_page,omitempty"`
	// FrameMatch — only with CaptureDOMSnapshot: read the embedded document
	// (iframe) whose current address matches, instead of the page. Exactly one
	// frame must match; with none or several, nothing is read and the call
	// returns a 409 *ConflictError.
	FrameMatch *FrameMatch `json:"frame_match,omitempty"`
}

// FrameMatch names an embedded frame by its current address. Host is compared
// exactly (letter case aside; the port is not compared); PathPrefix in whole
// path segments ("/embed" matches "/embed/card", not "/embedded"); each Query
// entry must be present with exactly that value once its URL encoding is
// undone, on every occurrence of the parameter. A Query name whose value the
// browser removes before it reaches Driftstack (client_secret, token,
// code_verifier and the like) is refused with a 400.
type FrameMatch struct {
	Host       string            `json:"host"`
	PathPrefix string            `json:"path_prefix,omitempty"`
	Query      map[string]string `json:"query,omitempty"`
}

type CaptureResponse struct {
	Kind       CaptureKind `json:"kind"`
	Data       string      `json:"data"`     // base64 or utf8 depending on Encoding
	Encoding   string      `json:"encoding"` // base64 | utf8
	ByteSize   int         `json:"byte_size"`
	DurationMS int         `json:"duration_ms"`
}

// ListFieldExtraction — per-field sub-extraction for a type:"list" extraction
// (runs against each matched element). Type is text|attribute only (no nested lists).
type ListFieldExtraction struct {
	Type      string `json:"type"`                // text | attribute
	Attribute string `json:"attribute,omitempty"` // required when Type=="attribute"
	Selector  string `json:"selector,omitempty"`  // optional sub-selector relative to the element
}

// Extraction — one named extraction in an ExtractRequest.
type Extraction struct {
	Name      string                         `json:"name"`
	Selector  string                         `json:"selector"`
	Type      string                         `json:"type"`                // text | attribute | list
	Attribute string                         `json:"attribute,omitempty"` // required when Type=="attribute"
	Transform string                         `json:"transform,omitempty"` // "number" parses the text as numeric
	Extract   map[string]ListFieldExtraction `json:"extract,omitempty"`   // per-field sub-extraction for Type=="list"
}

type ExtractRequest struct {
	Extractions []Extraction `json:"extractions"` // 1..100
}

type ExtractResponse struct {
	// Extracted values keyed by each extraction's Name (heterogeneous:
	// string | number | array per the extraction type — the page data).
	Value map[string]any `json:"value"`
}

type SearchRequest struct {
	Query          string `json:"query"`
	SearchSelector string `json:"search_selector,omitempty"`
	// Submit (Return) after typing. Defaults to true server-side; *bool so a
	// caller can send an explicit false (a plain bool's zero value can't).
	Submit                 *bool  `json:"submit,omitempty"`
	WaitForResultsSelector string `json:"wait_for_results_selector,omitempty"`
	// Caps the wait_for_results_selector wait (seconds; 1..120). Omit → server default (10s).
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

type SearchResponse struct {
	Submitted      bool `json:"submitted"`
	QueryTruncated bool `json:"query_truncated"`
	// Present only when WaitForResultsSelector was given (timeout → false).
	ResultsVisible *bool `json:"results_visible,omitempty"`
	DurationMS     int   `json:"duration_ms"`
}

// UnmarshalJSON enforces the strict complete-vs-safe-refusal search result.
// A truncated query is never submitted and cannot carry a results assessment.
func (r *SearchResponse) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowed := map[string]bool{
		"submitted": true, "query_truncated": true,
		"results_visible": true, "duration_ms": true,
	}
	for name := range fields {
		if !allowed[name] {
			return fmt.Errorf("invalid session search response field %q", name)
		}
	}
	if raw, present := fields["results_visible"]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("invalid session search response: results_visible cannot be null")
	}

	var wire struct {
		Submitted      *bool `json:"submitted"`
		QueryTruncated *bool `json:"query_truncated"`
		ResultsVisible *bool `json:"results_visible"`
		DurationMS     *int  `json:"duration_ms"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Submitted == nil || wire.QueryTruncated == nil || wire.DurationMS == nil {
		return fmt.Errorf("invalid session search response: missing required outcome field")
	}
	if *wire.DurationMS < 0 || *wire.DurationMS > 600_000 {
		return fmt.Errorf("invalid session search response: duration_ms outside 0..600000")
	}
	if *wire.QueryTruncated && (*wire.Submitted || wire.ResultsVisible != nil) {
		return fmt.Errorf("invalid session search response: contradictory truncated outcome")
	}

	r.Submitted = *wire.Submitted
	r.QueryTruncated = *wire.QueryTruncated
	r.ResultsVisible = wire.ResultsVisible
	r.DurationMS = *wire.DurationMS
	return nil
}

// SessionLoginRequest drives the in-browser credential-login op. Named
// SessionLogin* (not Login*) to avoid colliding with the account-login types.
type SessionLoginRequest struct {
	Username string `json:"username"`
	// Password is SENSITIVE — typed via the behavioural send-keys path; never logged.
	Password         string `json:"password"`
	UsernameSelector string `json:"username_selector,omitempty"`
	PasswordSelector string `json:"password_selector,omitempty"`
	SubmitSelector   string `json:"submit_selector,omitempty"`
	SuccessSelector  string `json:"success_selector,omitempty"`
	// Caps the post-submit success wait (seconds; 1..120). Omit → server default (10s).
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

type SessionLoginResponse struct {
	Submitted            bool   `json:"submitted"`
	CredentialsTruncated bool   `json:"credentials_truncated"`
	LoggedIn             bool   `json:"logged_in"`
	PostLoginURL         string `json:"post_login_url,omitempty"`
	DurationMS           int    `json:"duration_ms"`
}

// UnmarshalJSON enforces the public two-branch login result. A truncated
// credential is a safe zero-submit refusal and therefore cannot carry a URL;
// a complete credential flow must report submitted=true. This keeps the Go SDK
// from silently accepting a contradictory server response.
func (r *SessionLoginResponse) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowed := map[string]bool{
		"submitted": true, "credentials_truncated": true, "logged_in": true,
		"post_login_url": true, "duration_ms": true,
	}
	for name := range fields {
		if !allowed[name] {
			return fmt.Errorf("invalid session login response field %q", name)
		}
	}
	if raw, present := fields["post_login_url"]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("invalid session login response: post_login_url cannot be null")
	}

	var wire struct {
		Submitted            *bool   `json:"submitted"`
		CredentialsTruncated *bool   `json:"credentials_truncated"`
		LoggedIn             *bool   `json:"logged_in"`
		PostLoginURL         *string `json:"post_login_url"`
		DurationMS           *int    `json:"duration_ms"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Submitted == nil || wire.CredentialsTruncated == nil || wire.LoggedIn == nil || wire.DurationMS == nil {
		return fmt.Errorf("invalid session login response: missing required outcome field")
	}
	if *wire.DurationMS < 0 || *wire.DurationMS > 600_000 {
		return fmt.Errorf("invalid session login response: duration_ms outside 0..600000")
	}
	if *wire.CredentialsTruncated {
		if *wire.Submitted || *wire.LoggedIn || wire.PostLoginURL != nil {
			return fmt.Errorf("invalid session login response: contradictory truncated outcome")
		}
	} else if !*wire.Submitted {
		return fmt.Errorf("invalid session login response: complete credentials were not submitted")
	}

	r.Submitted = *wire.Submitted
	r.CredentialsTruncated = *wire.CredentialsTruncated
	r.LoggedIn = *wire.LoggedIn
	r.DurationMS = *wire.DurationMS
	r.PostLoginURL = ""
	if wire.PostLoginURL != nil {
		r.PostLoginURL = *wire.PostLoginURL
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────
// Usage
// ──────────────────────────────────────────────────────────────────

type UsageTotals map[UsageRecordType]int

// UsageQuotas — null entries mean unmetered (enterprise tier).
type UsageQuotas map[UsageRecordType]*int

type UsagePeriodSummary struct {
	PeriodStart time.Time   `json:"period_start"`
	PeriodEnd   time.Time   `json:"period_end"`
	Tier        AccountTier `json:"tier"`
	Totals      UsageTotals `json:"totals"`
	Quotas      UsageQuotas `json:"quotas"`
}

// ──────────────────────────────────────────────────────────────────
// Webhooks
// ──────────────────────────────────────────────────────────────────

// WebhookEndpointDeliveryCounts — aggregate per-endpoint delivery
// counts surfaced on every WebhookEndpoint response.
type WebhookEndpointDeliveryCounts struct {
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
	DLQ       int `json:"dlq"`
}

type WebhookEndpoint struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	SecretPrefix string `json:"secret_prefix"`
	// Secret rotation grace state. Both null when no rotation in flight.
	PrevSecretPrefix       *string                       `json:"prev_secret_prefix"`
	RotationGraceExpiresAt *time.Time                    `json:"rotation_grace_expires_at"`
	Events                 []WebhookEventType            `json:"events"`
	Description            *string                       `json:"description"`
	Active                 bool                          `json:"active"`
	ConsecutiveFailures    int                           `json:"consecutive_failures"`
	LastSuccessAt          *time.Time                    `json:"last_success_at"`
	LastFailureAt          *time.Time                    `json:"last_failure_at"`
	DisabledAt             *time.Time                    `json:"disabled_at"`
	DeliveryCounts         WebhookEndpointDeliveryCounts `json:"delivery_counts"`
	CreatedAt              time.Time                     `json:"created_at"`
}

type WebhookEndpointList struct {
	Data []WebhookEndpoint `json:"data"`
}

// CreateWebhookRequest — Description is a pointer so nil omits the field
// entirely while a pointer to "" transmits an explicit empty description
// (a plain string with omitempty could never send an empty value).
// Matches UpdateWebhookRequest and the nullable contract.
type CreateWebhookRequest struct {
	URL         string             `json:"url"`
	Events      []WebhookEventType `json:"events"`
	Description *string            `json:"description,omitempty"`
}

type CreateWebhookResponse struct {
	WebhookEndpoint
	Secret string `json:"secret"`
}

// UpdateWebhookRequest — partial update. Pointer fields so
// callers can distinguish "leave as-is" (nil) from "set explicitly"
// (non-nil). At least one field must be non-nil; the server returns
// 400 otherwise.
type UpdateWebhookRequest struct {
	URL         *string             `json:"url,omitempty"`
	Events      *[]WebhookEventType `json:"events,omitempty"`
	Description *string             `json:"description,omitempty"`
	Active      *bool               `json:"active,omitempty"`
}

type WebhookDelivery struct {
	ID                  string                `json:"id"`
	WebhookID           string                `json:"webhook_id"`
	EventID             string                `json:"event_id"`
	EventType           WebhookEventType      `json:"event_type"`
	Status              WebhookDeliveryStatus `json:"status"`
	Attempts            int                   `json:"attempts"`
	NextAttemptAt       time.Time             `json:"next_attempt_at"`
	LastResponseStatus  *int                  `json:"last_response_status"`
	LastResponseExcerpt *string               `json:"last_response_excerpt"`
	LastError           *string               `json:"last_error"`
	DeliveredAt         *time.Time            `json:"delivered_at"`
	CreatedAt           time.Time             `json:"created_at"`
}

type WebhookDeliveryListPage struct {
	Data       []WebhookDelivery `json:"data"`
	HasMore    bool              `json:"has_more"`
	NextCursor *string           `json:"next_cursor"`
}

type ListDeliveriesQuery struct {
	Limit  int                   `url:"limit,omitempty"`
	Cursor string                `url:"cursor,omitempty"`
	Status WebhookDeliveryStatus `url:"status,omitempty"`
}

// ──────────────────────────────────────────────────────────────────
// Webhook event payload (what the server POSTs to your endpoint)
// ──────────────────────────────────────────────────────────────────

// Event is the envelope every webhook delivery wraps. Customers
// typically un-marshal the body into this and switch on Type.
type Event struct {
	ID        string           `json:"id"`
	Type      WebhookEventType `json:"type"`
	CreatedAt time.Time        `json:"created_at"`
	Data      json.RawMessage  `json:"data"`
}

// SessionCompletedData is the Data shape for type=session.completed.
type SessionCompletedData struct {
	SessionID  string `json:"session_id"`
	DurationMS int    `json:"duration_ms"`
	OpsCount   int    `json:"ops_count"`
}

// APIKeyRevokedData is the Data shape for type=api_key.revoked.
type APIKeyRevokedData struct {
	APIKeyID  string    `json:"api_key_id"`
	Name      string    `json:"name"`
	RevokedAt time.Time `json:"revoked_at"`
}

// ──────────────────────────────────────────────────────────────────
// Profiles
// ──────────────────────────────────────────────────────────────────

// Profile matches the public ProfileSchema returned by
// /v1/profiles. The browser state a profile carries (persona /
// storage_state / notes) is held for you and is not readable through
// the API; this struct is the metadata you can read and set.
// `Description` is `*string` to capture explicit-null vs. unset.
type Profile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Archetype   string   `json:"archetype"`
	Description *string  `json:"description"`
	Folder      *string  `json:"folder"`
	Tags        []string `json:"tags"`
	// Icon + Note — per-account UI metadata (2026-06-16). Icon = short emoji
	// (nil/empty = monogram); Note = short inline annotation.
	Icon *string `json:"icon"`
	Note *string `json:"note"`
	// Notes — the free-text notes kept with the profile (the "Notes" section
	// of the desktop app's session window): plain text, UTF-8, at most 16 KiB. nil until
	// something is written. Anyone who launches the profile sees the same notes.
	Notes *string `json:"notes"`
	// DefaultProxyID / ProxyChoice — the saved proxy this profile launches
	// through (nil when none) and its three-state choice: "unset" (never
	// chose), "bound" (DefaultProxyID is used when a create names the profile
	// and no ProxyID), "detached" (its proxy was deleted; a create naming it
	// without a ProxyID is refused 409 until a proxy is chosen). Set by the
	// server; send DefaultProxyID on Create or Update to bind, or
	// ClearDefaultProxyID on Update to unset.
	DefaultProxyID *string `json:"default_proxy_id"`
	ProxyChoice    string  `json:"proxy_choice"`
	// Geolocation / StopOnExitIPChange — launch settings that default onto a
	// session created with this profile when the create omits them.
	Geolocation        *ProfileGeolocation `json:"geolocation"`
	StopOnExitIPChange bool                `json:"stop_on_exit_ip_change"`
	// Revision — moves on every edit. Send it back as ExpectedRevision on
	// Update to be told (409, code "stale_revision") when another computer
	// changed the profile first.
	Revision   int        `json:"revision"`
	LastUsedAt *time.Time `json:"last_used_at"`
	// SizeBytes + LastSavedAt. SizeBytes is the byte size of
	// the last saved sealed store (the opaque encrypted browser-state blob);
	// nil until the profile is first saved. *int64: a sealed store can exceed
	// the 2^31 int ceiling. LastSavedAt is when it was last saved back.
	SizeBytes   *int64     `json:"size_bytes"`
	LastSavedAt *time.Time `json:"last_saved_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// DeletedAt — L4b recycle bin. nil for a live profile; set to the trash
	// timestamp for a soft-deleted one (only ListTrash returns trashed rows).
	DeletedAt *time.Time `json:"deleted_at"`
	// PurgesAt — set on a profile in the Trash: the earliest time the profile
	// is permanently deleted (at least 7 days after the delete; removal
	// follows at the next daily clean-up). nil on a live profile, and from a
	// server older than the field.
	PurgesAt *time.Time `json:"purges_at"`
}

// ProfileGeolocation — a profile's fixed location: latitude/longitude in
// degrees, Accuracy in metres (0 = the device default).
type ProfileGeolocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy,omitempty"`
}

// CreateProfileRequest matches the server's create-profile request.
// Archetype defaults to the live catalog's default_archetype_id when omitted;
// call GET /v1/archetypes instead of hard-coding a device generation.
type CreateProfileRequest struct {
	Name        string   `json:"name"`
	Archetype   string   `json:"archetype,omitempty"`
	Description string   `json:"description,omitempty"`
	Folder      string   `json:"folder,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Icon        string   `json:"icon,omitempty"` // short emoji (≤16) — per-account UI metadata
	Note        string   `json:"note,omitempty"` // short inline note (≤280)
	// DefaultProxyID — the id of one of your saved proxies for the profile to
	// launch through (ProxyChoice then reads "bound"). The id of a proxy that
	// is not one of yours is a 404 *NotFoundError; an http proxy, which cannot
	// carry a session, is a 400 *BadRequestError. Empty sends nothing, and the
	// profile starts "unset".
	DefaultProxyID string `json:"default_proxy_id,omitempty"`
	// Geolocation / StopOnExitIPChange — launch settings that default onto a
	// session created with this profile when the create omits them. Nil and
	// false send nothing: the profile has no fixed location and does not stop
	// its sessions when their exit IP changes.
	Geolocation        *ProfileGeolocation `json:"geolocation,omitempty"`
	StopOnExitIPChange bool                `json:"stop_on_exit_ip_change,omitempty"`
}

// UpdateProfileRequest matches the server's update-profile request.
// Every field is optional. A nil pointer, a nil or empty Tags and a false
// Clear flag send nothing, and the server leaves that setting as it is.
//
// A pointer sets a value. A Clear flag removes one: ClearDescription,
// ClearFolder, ClearIcon, ClearNote, ClearNotes, ClearDefaultProxyID and
// ClearGeolocation send the field as JSON null, and ClearTags sends an empty
// list, which removes every tag. A pointer and its own Clear flag set together
// contradict each other, so Update returns an error and sends nothing;
// IsRetryable reports false for it, since resending cannot succeed. A
// pointer to "" sends an empty string, which the server stores as "" rather
// than null (and refuses for Folder, with a 400); only a Clear flag stores
// null.
type UpdateProfileRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Folder      *string `json:"folder,omitempty"`
	// Tags replaces the whole set; send ClearTags to remove every tag.
	Tags  []string `json:"tags,omitempty"`
	Icon  *string  `json:"icon,omitempty"`  // short emoji (≤16) — per-account UI metadata
	Note  *string  `json:"note,omitempty"`  // short inline note (≤280)
	Notes *string  `json:"notes,omitempty"` // free-text notes kept with the profile (≤16 KiB UTF-8)
	// ProxyChoice — only "detached" may be sent: the profile has no proxy and
	// is never given one you did not choose (what deleting its proxy does,
	// asked for directly). Nil leaves the choice alone. Sent beside a
	// DefaultProxyID it is a 400 *BadRequestError.
	ProxyChoice *string `json:"proxy_choice,omitempty"`
	// DefaultProxyID — the id of one of your saved proxies for the profile to
	// launch through (ProxyChoice then reads "bound"); a 404 *NotFoundError
	// for a proxy that is not one of yours, a 400 *BadRequestError for an http
	// proxy. ClearDefaultProxyID unsets it instead (ProxyChoice "unset").
	DefaultProxyID *string `json:"default_proxy_id,omitempty"`
	// Geolocation — the fixed location a session created with this profile
	// uses when its create does not send one; ClearGeolocation removes it.
	Geolocation *ProfileGeolocation `json:"geolocation,omitempty"`
	// StopOnExitIPChange — whether a session created with this profile stops
	// when its exit IP changes, when its create does not say. A pointer, so
	// false can be sent.
	StopOnExitIPChange *bool `json:"stop_on_exit_ip_change,omitempty"`
	// ExpectedRevision — the Profile.Revision you last read. When the profile
	// has moved on since (another computer changed it), the update is refused
	// with a 409 *ConflictError whose Problem["code"] is "stale_revision" and
	// Problem["current_revision"] the revision now stored, and nothing
	// changes. Nil writes unconditionally.
	ExpectedRevision *int `json:"expected_revision,omitempty"`

	// The Clear flags. Each sends its field as null (ClearTags: an empty
	// list), removing what is stored. Never sent themselves.
	ClearDescription    bool `json:"-"`
	ClearFolder         bool `json:"-"`
	ClearTags           bool `json:"-"`
	ClearIcon           bool `json:"-"`
	ClearNote           bool `json:"-"`
	ClearNotes          bool `json:"-"`
	ClearDefaultProxyID bool `json:"-"`
	ClearGeolocation    bool `json:"-"`
}

// MarshalJSON writes the body the server reads: nothing for a field left
// alone, null (or [] for tags) for a Clear flag, and the value for a pointer.
// It fails, so nothing is sent, when a field and its Clear flag are both set.
func (r UpdateProfileRequest) MarshalJSON() ([]byte, error) {
	if err := r.contradiction(); err != nil {
		return nil, err
	}
	// Field order is the order the body had before this method existed, so a
	// request that sets none of the new fields is written byte for byte as it was.
	var wire struct {
		Name               *string         `json:"name,omitempty"`
		Description        json.RawMessage `json:"description,omitempty"`
		Folder             json.RawMessage `json:"folder,omitempty"`
		Tags               json.RawMessage `json:"tags,omitempty"`
		Icon               json.RawMessage `json:"icon,omitempty"`
		Note               json.RawMessage `json:"note,omitempty"`
		Notes              json.RawMessage `json:"notes,omitempty"`
		ProxyChoice        *string         `json:"proxy_choice,omitempty"`
		DefaultProxyID     json.RawMessage `json:"default_proxy_id,omitempty"`
		Geolocation        json.RawMessage `json:"geolocation,omitempty"`
		StopOnExitIPChange *bool           `json:"stop_on_exit_ip_change,omitempty"`
		ExpectedRevision   *int            `json:"expected_revision,omitempty"`
	}
	var err error
	if wire.Description, err = clearableField(r.Description, r.ClearDescription); err != nil {
		return nil, err
	}
	if wire.Folder, err = clearableField(r.Folder, r.ClearFolder); err != nil {
		return nil, err
	}
	if wire.Icon, err = clearableField(r.Icon, r.ClearIcon); err != nil {
		return nil, err
	}
	if wire.Note, err = clearableField(r.Note, r.ClearNote); err != nil {
		return nil, err
	}
	if wire.Notes, err = clearableField(r.Notes, r.ClearNotes); err != nil {
		return nil, err
	}
	if wire.DefaultProxyID, err = clearableField(r.DefaultProxyID, r.ClearDefaultProxyID); err != nil {
		return nil, err
	}
	if wire.Geolocation, err = clearableField(r.Geolocation, r.ClearGeolocation); err != nil {
		return nil, err
	}
	switch {
	case r.ClearTags:
		wire.Tags = json.RawMessage("[]")
	case len(r.Tags) > 0:
		if wire.Tags, err = json.Marshal(r.Tags); err != nil {
			return nil, err
		}
	}
	wire.Name = r.Name
	wire.ProxyChoice = r.ProxyChoice
	wire.StopOnExitIPChange = r.StopOnExitIPChange
	wire.ExpectedRevision = r.ExpectedRevision
	return json.Marshal(wire)
}

// contradiction returns an error naming the first field set beside its own
// Clear flag, or nil. Update calls it before anything is encoded, so the
// caller gets a plain error, which IsRetryable reports false for: resending
// the same request can never succeed. (Left to json.Marshal, the failure
// would reach the caller as a *TransportError, which IsRetryable reports true
// for.) MarshalJSON calls it too, for a caller who encodes the request itself.
func (r *UpdateProfileRequest) contradiction() error {
	for _, f := range []struct {
		name       string
		set, clear bool
	}{
		{"Description", r.Description != nil, r.ClearDescription},
		{"Folder", r.Folder != nil, r.ClearFolder},
		{"Tags", len(r.Tags) > 0, r.ClearTags},
		{"Icon", r.Icon != nil, r.ClearIcon},
		{"Note", r.Note != nil, r.ClearNote},
		{"Notes", r.Notes != nil, r.ClearNotes},
		{"DefaultProxyID", r.DefaultProxyID != nil, r.ClearDefaultProxyID},
		{"Geolocation", r.Geolocation != nil, r.ClearGeolocation},
	} {
		if f.set && f.clear {
			return fmt.Errorf("driftstack: UpdateProfileRequest sets both %s and Clear%s; set one", f.name, f.name)
		}
	}
	return nil
}

// clearableField encodes one field that a request can leave alone (nil
// value, remove false: nothing is written), set (value) or clear (null).
// contradiction has already refused a value and remove set together.
func clearableField[T any](value *T, remove bool) (json.RawMessage, error) {
	switch {
	case remove:
		return json.RawMessage("null"), nil
	case value == nil:
		return nil, nil
	}
	return json.Marshal(value)
}

type ProfilesListPage struct {
	Data       []Profile `json:"data"`
	HasMore    bool      `json:"has_more"`
	NextCursor *string   `json:"next_cursor"`
}

// ProfilesTrashList — L4b recycle bin. The trashed-profiles list is small +
// ephemeral, so it's an unpaginated { data } envelope (no cursor).
type ProfilesTrashList struct {
	Data []Profile `json:"data"`
}

// ProfileListStatus selects which profiles Profiles.List returns.
type ProfileListStatus string

const (
	// ProfileListActive lists the live profiles (the default).
	ProfileListActive ProfileListStatus = "active"
	// ProfileListTrashed lists the profiles in the trash.
	ProfileListTrashed ProfileListStatus = "trashed"
)

// ListProfilesQuery is the query of Profiles.List. Status is empty for the
// live profiles (the default) or ProfileListTrashed for the trash; with
// ProfileListTrashed the whole trash comes back in one page: Limit and Cursor
// are not applied, HasMore is false and NextCursor is nil.
type ListProfilesQuery struct {
	Limit  int
	Cursor string
	Status ProfileListStatus
}
