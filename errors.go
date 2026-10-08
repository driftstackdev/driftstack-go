package driftstack

import (
	"encoding/json"
	"errors"
	"fmt"
)

// apiError is the base error payload embedded by every typed error
// returned by the SDK. Renamed from "Error" so the embedded field
// name doesn't shadow Go's `error` interface's Error() method.
//
// Callers don't construct apiError directly — switch on the typed
// errors below with errors.As:
//
//	var rl *driftstack.RateLimitError
//	if errors.As(err, &rl) {
//	    time.Sleep(time.Duration(rl.RetryAfterSeconds) * time.Second)
//	}
type apiError struct {
	// Status is the HTTP status code, or 0 for transport-level failures
	// (network error, timeout, parse error) that didn't reach the server.
	Status int
	// ProblemType is the stable RFC 7807 type URI from the server. Empty
	// for transport-level failures.
	ProblemType string
	// Message is the human-readable error detail.
	Message string
	// Problem is the full parsed problem document so callers can read
	// fields the SDK didn't lift to typed properties.
	Problem map[string]any
	// Cause is the underlying error (e.g., a net.OpError) when one exists.
	Cause error
}

func (e *apiError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("driftstack: %s (status=%d, cause=%v)", e.Message, e.Status, e.Cause)
	}
	return fmt.Sprintf("driftstack: %s (status=%d)", e.Message, e.Status)
}

func (e *apiError) Unwrap() error { return e.Cause }

// Sentinel errors so callers can use errors.Is for category matching
// without unwrapping to the typed shape. errors.As is still the right
// path when the typed payload (RetryAfterSeconds, etc.) matters.
var (
	ErrAuth                    = errors.New("authentication failed")
	ErrForbidden               = errors.New("forbidden")
	ErrDashboardOnly           = errors.New("dashboard only")
	ErrInvalidKey              = errors.New("invalid api key")
	ErrExpiredKey              = errors.New("api key expired")
	ErrRevokedKey              = errors.New("api key revoked")
	ErrBadRequest              = errors.New("bad request")
	ErrValidation              = errors.New("validation failed")
	ErrNotFound                = errors.New("not found")
	ErrConflict                = errors.New("conflict")
	ErrRateLimit               = errors.New("rate limited")
	ErrConcurrencyLimit        = errors.New("concurrency limit hit")
	ErrQuotaExceeded           = errors.New("quota exceeded")
	ErrStorageQuotaExceeded    = errors.New("storage quota reached")
	ErrSessionDestroyed        = errors.New("session destroyed")
	ErrSessionTimeout          = errors.New("session timeout")
	ErrLegalAcceptanceRequired = errors.New("legal acceptance required")
	ErrDriverError             = errors.New("driver error")
	ErrTransport               = errors.New("transport-level failure")
	// The auth-flow problem types.
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidAuthToken       = errors.New("invalid auth token")
	ErrEmailNotVerified       = errors.New("email not verified")
	// The remaining problem types.
	ErrFeatureUnavailable = errors.New("feature unavailable")
	ErrMfaStepUpRequired  = errors.New("mfa step-up required")
	ErrInternal           = errors.New("internal error")
	// Your own Anthropic key is required. errors.Is(err,
	// driftstack.ErrByokAnthropicRequired) tells a caller to supply one,
	// or to fall back, before retrying.
	ErrByokAnthropicRequired = errors.New("byok anthropic key required")
	// Bundled-LLM 402 paths.
	ErrBundledLlmBudgetExhausted = errors.New("bundled-llm monthly cap reached")
	ErrBundledLlmConsentRequired = errors.New("bundled-llm consent required")
	// Pair-mode 409 paths.
	ErrPairModeConflict               = errors.New("pair-mode takeover already in flight")
	ErrPairModeStateInvalidTransition = errors.New("invalid pair-mode transition")
	// Live pre-launch proxy validation (422 at launch).
	ErrProxyValidationFailed = errors.New("proxy validation failed")
	// Single-active-session-per-profile guard (409 at launch).
	ErrProfileInUse = errors.New("profile already in use")
	// Too large for the endpoint or for the session (413).
	ErrPayloadTooLarge = errors.New("payload too large")
	// The profile's device is not offered right now (409 at launch).
	ErrDeviceUnavailable = errors.New("device unavailable")
	// A session asked for without one of your saved proxies (422 at create).
	ErrProxyRequired = errors.New("proxy required")
	// A desktop build too old to write or launch (426); reads still work.
	ErrUpgradeRequired = errors.New("upgrade required")
	// An AI turn the account's AI credits could not run (402).
	ErrAiCreditsExhausted = errors.New("ai credits exhausted")
	// The account's trial is over; sessions start again once it subscribes (402).
	ErrTrialEnded = errors.New("trial ended")
	// The agent session's browser is still starting (409, retryable).
	ErrSessionNotReady = errors.New("session not ready")
)

// AuthError covers any of the auth-related problem types. Use the
// sentinel siblings (ErrInvalidKey, ErrExpiredKey, ErrRevokedKey) for
// finer-grained discrimination via errors.Is.
type AuthError struct {
	apiError
}

func (e *AuthError) Is(target error) bool { return target == ErrAuth }

type InvalidKeyError struct{ apiError }

func (e *InvalidKeyError) Is(target error) bool { return target == ErrInvalidKey || target == ErrAuth }

type ExpiredKeyError struct{ apiError }

func (e *ExpiredKeyError) Is(target error) bool { return target == ErrExpiredKey || target == ErrAuth }

type RevokedKeyError struct{ apiError }

func (e *RevokedKeyError) Is(target error) bool { return target == ErrRevokedKey || target == ErrAuth }

// ForbiddenError — 403: the key is valid but may not do this. Usually a
// missing scope or a plan without the feature; on an agent session it can
// also mean the chosen model needs your own Anthropic key (RequiresOwnKey).
type ForbiddenError struct{ apiError }

func (e *ForbiddenError) Is(target error) bool { return target == ErrForbidden || target == ErrAuth }

// RequiresOwnKey reports whether an Opus-class model (or Claude Fable 5.1) was
// refused because it runs only on your own Anthropic key and the session or
// turn would have run on Driftstack's included AI. Add a key (stored, or
// ByokAPIKey on the call) or pick another model. False for every other 403.
func (e *ForbiddenError) RequiresOwnKey() bool {
	v, _ := e.Problem["requires_own_key"].(bool)
	return v
}

// Model returns the model that was refused when RequiresOwnKey is true, and
// "" otherwise.
func (e *ForbiddenError) Model() string {
	v, _ := e.Problem["model"].(string)
	return v
}

// DashboardOnlyError — 403 dashboard-only: something a person does in the
// Driftstack dashboard (checkout, orders, creating API keys, accepting an
// invite or the terms, signing other devices out), asked for with an API key.
// Do it in the dashboard. errors.Is matches ErrDashboardOnly, ErrForbidden
// and ErrAuth, so code that handles a 403 still handles it.
type DashboardOnlyError struct{ apiError }

func (e *DashboardOnlyError) Is(target error) bool {
	return target == ErrDashboardOnly || target == ErrForbidden || target == ErrAuth
}

// BadRequestError — 400 with the generic bad-request problem type (no
// field-level issues breakdown). Distinguished from ValidationError (the
// validation-failed problem type, which carries an issues list) so callers
// can tell a structural "couldn't make sense of the request at all"
// failure apart from "these specific fields are invalid". Mirrors the TS +
// Python SDKs' BadRequestError.
type BadRequestError struct{ apiError }

func (e *BadRequestError) Is(target error) bool { return target == ErrBadRequest }

// ValidationError — 400 or 422 with the validation-failed problem type.
type ValidationError struct{ apiError }

func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

// NotFoundError — 404.
type NotFoundError struct{ apiError }

func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

// ConflictError — 409: the request conflicts with the current state. On an
// agent-session message the accessors below say which conflict it is; each
// returns its zero value when the server did not send the field.
type ConflictError struct{ apiError }

func (e *ConflictError) Is(target error) bool { return target == ErrConflict }

// TurnInProgress reports that another message is still running on this agent
// session. Wait for it to finish (or Stop it), then send again.
func (e *ConflictError) TurnInProgress() bool {
	v, _ := e.Problem["turn_in_progress"].(bool)
	return v
}

// SessionStatus is set when the agent session is not active ("closed" or
// "paused"): it already was when the message arrived, or this turn ended it —
// for example its token budget ran out. An open string.
func (e *ConflictError) SessionStatus() string {
	v, _ := e.Problem["session_status"].(string)
	return v
}

// ClosedReason is why the session ended, when SessionStatus is "closed" and
// the session records a reason: the same value Get returns as ClosedReason
// ("customer-closed", "budget-exhausted", "transcript-limit", …), so no second
// call is needed. An open string; empty for a paused session and on older
// servers.
func (e *ConflictError) ClosedReason() string {
	v, _ := e.Problem["closed_reason"].(string)
	return v
}

// ClosedReasonDetail is ClosedReason as one plain sentence you can show a
// person, when SessionStatus is "closed": the same value Get returns as
// ClosedReasonDetail. The wording may change; branch on ClosedReason. Empty
// for a paused session and on older servers.
func (e *ConflictError) ClosedReasonDetail() string {
	v, _ := e.Problem["closed_reason_detail"].(string)
	return v
}

// IdempotencyStatus is set when the conflict is about the Idempotency-Key:
// "in_progress" (the first request with this key is still running — retry
// the SAME key later and it replays the result) or "mismatch" (the key was
// already used for a different request). An open string.
func (e *ConflictError) IdempotencyStatus() string {
	v, _ := e.Problem["idempotency_status"].(string)
	return v
}

// AIControlUnavailable reports that AI control of the session changed while
// the turn was running, so it stopped early. Check PartialResults first.
func (e *ConflictError) AIControlUnavailable() bool {
	v, _ := e.Problem["ai_control_unavailable"].(bool)
	return v
}

// Phase is where the turn was when AI control changed. An open string.
func (e *ConflictError) Phase() string {
	v, _ := e.Problem["phase"].(string)
	return v
}

// TokensConsumed is the tokens the turn spent before it ended; ok is false
// when the server did not say.
func (e *ConflictError) TokensConsumed() (tokens int, ok bool) {
	v, isNumber := e.Problem["tokens_consumed"].(float64)
	if !isNumber {
		return 0, false
	}
	return int(v), true
}

// Usage is the turn's usage block when it did any work; nil otherwise.
func (e *ConflictError) Usage() *AgentUsage {
	var usage AgentUsage
	if !reDecodeProblemField(e.Problem, "usage", &usage) {
		return nil
	}
	return &usage
}

// PartialResults are the steps that ran before the turn ended; nil when
// none did (or the field could not be read). Do not repeat them without
// checking the page.
func (e *ConflictError) PartialResults() []AgentIntentResult {
	var results []AgentIntentResult
	if !reDecodeProblemField(e.Problem, "partial_results", &results) {
		return nil
	}
	return results
}

// reDecodeProblemField decodes one problem member into out via a JSON round
// trip. False when the member is absent, null, or does not fit out.
func reDecodeProblemField(problem map[string]any, key string, out any) bool {
	raw, present := problem[key]
	if !present || raw == nil {
		return false
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return false
	}
	return json.Unmarshal(buf, out) == nil
}

// RateLimitError — 429 token-bucket. RetryAfterSeconds is the server's
// hint; the SDK's retry policy already honours it automatically, so most
// callers don't need to read this field.
type RateLimitError struct {
	apiError
	RetryAfterSeconds int
}

func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimit }

// ConcurrencyLimitError — 429 because the active-session count would
// exceed the tier's concurrent ceiling. CurrentSessions and Limit are
// the values reported in the problem document.
type ConcurrencyLimitError struct {
	apiError
	CurrentSessions int
	Limit           int
}

func (e *ConcurrencyLimitError) Is(target error) bool { return target == ErrConcurrencyLimit }

// QuotaExceededError — 429 because a per-period usage quota is
// exhausted. Current/Limit/RecordType describe which quota. RecordType
// carries the resource whose cap was reached ("profile" today); the server
// spells that field `resource` on the wire.
//
// TrashCount (wire `trash_count`), on the profile limit, is how many of
// Current are in the trash. Trashed profiles count until they are deleted
// permanently: Profiles.Delete with &DeleteProfileOptions{Permanent: true},
// or Profiles.Purge for one already in the trash. 0 when the trash is empty,
// on other limits, and from servers that do not send it.
type QuotaExceededError struct {
	apiError
	Current    int
	Limit      int
	RecordType string
	TrashCount int
}

func (e *QuotaExceededError) Is(target error) bool { return target == ErrQuotaExceeded }

// StorageQuotaExceededError — 409. A profile-backed
// session-launch was refused because the account's aggregate profile
// storage reached its tier's hard cap. UsedBytes/CapBytes/Tier report the
// overage. Only profile-backed launches raise this; enterprise is soft-only
// and never does.
type StorageQuotaExceededError struct {
	apiError
	// UsedBytes/CapBytes are int64: a tier's storage cap is GiB-scale and can
	// exceed 2^31 bytes (e.g. api_scale = 250 GiB), so a 32-bit `int` would
	// truncate the value on a 32-bit build (GOARCH=386/arm). int64 is exact on
	// every target.
	UsedBytes int64
	CapBytes  int64
	Tier      string
}

func (e *StorageQuotaExceededError) Is(target error) bool { return target == ErrStorageQuotaExceeded }

// ProxyValidationFailedError — 422. The proxy attached to a launch failed the
// server's LIVE pre-launch connectivity test (a real egress round-trip THROUGH
// the proxy). The launch was BLOCKED before any session or worker started. Reason
// is a stable enum for branching: "unreachable" (check host/port/online),
// "auth_failed" (re-enter credentials), "timeout" (proxy slow/down), or
// "egress_blocked" (proxy connects but its upstream can't reach the internet).
type ProxyValidationFailedError struct {
	apiError
	Reason string
}

func (e *ProxyValidationFailedError) Is(target error) bool {
	return target == ErrProxyValidationFailed
}

// ProxyRequiredError — 422. A session was asked for without one of your saved
// proxies, and nothing was created: no session, no charge, nothing recorded
// under your idempotency key. Every agent session runs through a proxy you chose, so
// set ProxyID to the id of a saved proxy (client.Egress.ListProxies lists them;
// it needs a key with the account_owner scope, and a team admin may also use
// one saved on their own account) and create it again. Code is
// "proxy_required"; the message is a sentence you can show your user. Not
// retryable as-is.
type ProxyRequiredError struct {
	apiError
	Code string
}

func (e *ProxyRequiredError) Is(target error) bool { return target == ErrProxyRequired }

// UpgradeRequiredError — 426. A write or a launch from a desktop sign-in whose
// Driftstack app build is older than the server's minimum, or that sent no
// build. Reads still work. Never raised for an SDK or API key; it reaches SDK
// code only through a desktop-provisioned credential. Code is
// "desktop_update_required"; MinDesktopBuild names the build to update to.
type UpgradeRequiredError struct {
	apiError
	Code            string
	MinDesktopBuild string
}

func (e *UpgradeRequiredError) Is(target error) bool { return target == ErrUpgradeRequired }

// ProfileInUseError — 409. A session-create carried a
// profile_id that already has a live (non-terminal) session for the account.
// Two sessions on the same profile would both restore + overwrite the same
// saved cookie/state blob (losing the customer's logins), so the launch is
// REFUSED. ActiveSessionID is the id of the live session (e.g. "ses_…" /
// "agt_…") — end it (or wait for it to finish) before launching another. A
// create without a profile_id never raises this. errors.Is matches both
// ErrProfileInUse and the broader ErrConflict.
type ProfileInUseError struct {
	apiError
	ActiveSessionID string
}

func (e *ProfileInUseError) Is(target error) bool {
	return target == ErrProfileInUse || target == ErrConflict
}

// DeviceUnavailableError — 409. The profile's device cannot start a session
// right now: it is on hold until its evidence exists, or still in development,
// so the device picker shows it but does not offer it. The launch was refused
// before a session existed, so nothing started and no concurrent-session slot
// was used. Archetype is the profile's device id and HeldReason the reason for
// the hold in one plain sentence ("" when there is none); Error() is a
// sentence you can show.
// Launch a profile on another device (GET /v1/archetypes lists the ones
// offered). errors.Is matches both ErrDeviceUnavailable and the broader
// ErrConflict.
type DeviceUnavailableError struct {
	apiError
	Archetype  string
	HeldReason string
}

func (e *DeviceUnavailableError) Is(target error) bool {
	return target == ErrDeviceUnavailable || target == ErrConflict
}

// SessionNotReadyError — 409. A message reached an agent session whose browser
// is still starting: it has not yet reported ready (the session's Ready is
// still false). The server held the message for a while
// first; nothing ran. Retryable: send the same message again after
// RetryAfterSeconds (the same Idempotency-Key is safe to reuse), or wait for
// Get to report Ready before sending. A VPN session can take longer to become
// ready. Code is "session_not_ready"; Retryable is true (nothing ran).
// errors.Is matches both ErrSessionNotReady and the broader ErrConflict.
type SessionNotReadyError struct {
	apiError
	Code              string
	Retryable         bool
	RetryAfterSeconds int
}

func (e *SessionNotReadyError) Is(target error) bool {
	return target == ErrSessionNotReady || target == ErrConflict
}

// PayloadTooLargeError — 413. The request is too large for where it has to
// go, and nothing was sent there: a body over the endpoint's size limit, or a
// file or cookie jar larger than the session takes at once. When the
// session is the limit, LimitBytes is the most that fits and
// SizeBytes what was sent; both are 0 otherwise. For uploads, check a file
// against the session's UploadMaxFileBytes first. errors.Is matches both
// ErrPayloadTooLarge and ErrBadRequest (a 413 used to arrive as bad-request).
type PayloadTooLargeError struct {
	apiError
	LimitBytes int64
	SizeBytes  int64
}

func (e *PayloadTooLargeError) Is(target error) bool {
	return target == ErrPayloadTooLarge || target == ErrBadRequest
}

// SessionDestroyedError — 410 when an op targets a destroyed session.
type SessionDestroyedError struct{ apiError }

func (e *SessionDestroyedError) Is(target error) bool { return target == ErrSessionDestroyed }

// SessionTimeoutError — 504 when an op exceeds the per-call
// timeout_ms. Distinguished from DriverError so customers can react
// specifically to "didn't finish in time" without conflating with
// downstream driver failures. TimeoutMs is the bound the server
// actually applied (may differ from the request if the server
// clamped it).
type SessionTimeoutError struct {
	apiError
	TimeoutMs int
}

func (e *SessionTimeoutError) Is(target error) bool { return target == ErrSessionTimeout }

// PendingAcceptance is one entry in LegalAcceptanceRequiredError's payload.
type PendingAcceptance struct {
	DocumentKey    string `json:"document_key"`
	CurrentVersion string `json:"current_version"`
}

// LegalAcceptanceRequiredError — 409 when a dashboard action (creating an
// API key) is gated on the account accepting one or more legal documents.
// The PendingAcceptances slice carries the document keys + current versions.
//
// Deprecated: raised only by dashboard actions; the dashboard walks you
// through the acceptance. A program using an API key does not receive it.
// Kept so code that already matches it keeps compiling.
type LegalAcceptanceRequiredError struct {
	apiError
	PendingAcceptances []PendingAcceptance
}

func (e *LegalAcceptanceRequiredError) Is(target error) bool {
	return target == ErrLegalAcceptanceRequired
}

// DriverError — 502 when the underlying driver (mock or real WebKit)
// returns an unrecoverable error.
type DriverError struct{ apiError }

func (e *DriverError) Is(target error) bool { return target == ErrDriverError }

// TransportError — network failure, parse failure, or any condition
// that didn't reach the server with a problem-json body. Status will
// be 0 for true transport failures (no response received) and the HTTP
// status for "got a response but it's not parseable as a problem doc".
type TransportError struct{ apiError }

func (e *TransportError) Is(target error) bool { return target == ErrTransport }

// UnknownError is the catch-all for problem-json responses whose
// `type` URI isn't in our mapping table. Future server-added problem
// types surface here until the SDK is updated; callers can still read
// the .Message and .Problem map.
type UnknownError struct{ apiError }

// Dashboard sign-in errors. Sign-in is not part of the API, so a program
// using an API key does not receive these; they stay for compatibility.

// EmailAlreadyRegisteredError — signing up with an email already on file.
//
// Deprecated: raised only by dashboard sign-in, which is not part of the
// API; a program using an API key does not receive it. Kept so code that
// already matches it keeps compiling.
type EmailAlreadyRegisteredError struct{ apiError }

func (e *EmailAlreadyRegisteredError) Is(target error) bool {
	return target == ErrEmailAlreadyRegistered
}

// InvalidCredentialsError — signing in with an email and password that do
// not match.
//
// Deprecated: raised only by dashboard sign-in, which is not part of the
// API; a program using an API key does not receive it. Kept so code that
// already matches it keeps compiling.
type InvalidCredentialsError struct{ apiError }

func (e *InvalidCredentialsError) Is(target error) bool { return target == ErrInvalidCredentials }

// InvalidAuthTokenError — a magic-link / password-reset / verify-
// email token is malformed, already-consumed, or expired.
//
// Deprecated: raised only by dashboard sign-in, which is not part of the
// API; a program using an API key does not receive it. Kept so code that
// already matches it keeps compiling.
type InvalidAuthTokenError struct{ apiError }

func (e *InvalidAuthTokenError) Is(target error) bool { return target == ErrInvalidAuthToken }

// EmailNotVerifiedError — signing in before the email address is verified.
//
// Deprecated: raised only by dashboard sign-in, which is not part of the
// API; a program using an API key does not receive it. Kept so code that
// already matches it keeps compiling.
type EmailNotVerifiedError struct{ apiError }

func (e *EmailNotVerifiedError) Is(target error) bool { return target == ErrEmailNotVerified }

// Additional typed errors closing the remaining
// problem-type gap.

// FeatureUnavailableError — an endpoint requires infrastructure not
// configured in this deployment (e.g. avatar uploads when R2 isn't
// wired). HTTP 503.
type FeatureUnavailableError struct{ apiError }

func (e *FeatureUnavailableError) Is(target error) bool { return target == ErrFeatureUnavailable }

// StopUnconfirmed is true only on the 503 AgentSessions.Stop gets when the
// stop could not be confirmed just now: the turn may still be running, so call
// Stop again. False for every other 503 of this type — including "AI is not
// enabled", where calling again would not help — which is why IsRetryable
// stays false for the type and this flag exists.
func (e *FeatureUnavailableError) StopUnconfirmed() bool {
	v, _ := e.Problem["stop_unconfirmed"].(bool)
	return v
}

// MfaStepUpRequiredError — a dashboard action needs a fresh second factor;
// the dashboard asks for the code.
//
// Deprecated: raised only by dashboard actions. A program using an API key
// does not receive it. Kept so code that already matches it keeps compiling.
type MfaStepUpRequiredError struct{ apiError }

func (e *MfaStepUpRequiredError) Is(target error) bool { return target == ErrMfaStepUpRequired }

// InternalError — unhandled server-side error. The detail message
// may be sanitized; check Driftstack status / contact support if
// this persists.
type InternalError struct{ apiError }

func (e *InternalError) Is(target error) bool { return target == ErrInternal }

// ByokAnthropicRequiredError — 502: the turn has no usable AI key. Two cases,
// told apart by KeyRejected():
//
//   - false — there is no key to run on: none on the request, none stored, and
//     Driftstack's included AI is not available to the account (a plan that
//     runs AI only on its own key is answered this way too). Store your
//     Anthropic key in the dashboard's Settings, or send it with the call
//     (MessageOptions.ByokAPIKey).
//   - true — Anthropic refused YOUR key on the turn's first planning call.
//     KeySource() says which key and KeyRejectedReason() why.
//
// No step ran in either case. IsRetryable is false although the status is a
// 502: sending the same request again gets the same answer until the key is
// added, replaced or fixed.
type ByokAnthropicRequiredError struct{ apiError }

func (e *ByokAnthropicRequiredError) Is(target error) bool {
	return target == ErrByokAnthropicRequired
}

// KeyRejected is true when Anthropic refused your own key, and false when
// there was no key.
func (e *ByokAnthropicRequiredError) KeyRejected() bool {
	v, _ := e.Problem["key_rejected"].(bool)
	return v
}

// KeySource is which key was refused: "header" (the ByokAPIKey sent with the
// call) or "stored" (the one saved on the account). An open string; empty
// unless KeyRejected.
func (e *ByokAnthropicRequiredError) KeySource() string {
	v, _ := e.Problem["key_source"].(string)
	return v
}

// KeyRejectedReason is why it was refused: "invalid_or_unauthorized" (invalid,
// revoked, or not permitted to run the model — replace it) or "billing" (the
// Anthropic account behind it cannot pay for the call — fix billing with
// Anthropic). An open string; empty unless KeyRejected.
func (e *ByokAnthropicRequiredError) KeyRejectedReason() string {
	v, _ := e.Problem["key_rejected_reason"].(string)
	return v
}

// BundledLlmBudgetExhaustedError — 402: the account's monthly budget for
// Driftstack's included AI is used up. SpentCents / CapCents say how far.
// Raise the cap in the dashboard's Settings, use your own Anthropic key, or
// wait for the next calendar month.
type BundledLlmBudgetExhaustedError struct {
	apiError
	SpentCents int
	CapCents   int
}

func (e *BundledLlmBudgetExhaustedError) Is(target error) bool {
	return target == ErrBundledLlmBudgetExhausted
}

// BundledLlmConsentRequiredError — 402: the turn would run on Driftstack's
// included AI, but the account has not opted in to it. Opt in in the
// dashboard's Settings, or use your own Anthropic key
// (MessageOptions.ByokAPIKey).
type BundledLlmConsentRequiredError struct{ apiError }

func (e *BundledLlmConsentRequiredError) Is(target error) bool {
	return target == ErrBundledLlmConsentRequired
}

// AiCreditsExhaustedError — 402: an AI turn could not run on the account's AI
// credits. Nothing ran. Reason tells the three shapes apart:
//
//   - "balance" — the credits the plan includes are used up;
//     AvailableCredits / RequiredCredits say how far short. ResetsAt, when
//     non-empty, is when the next credits arrive.
//   - "debt" — AI is paused on the account (DebtReason "payment_reversed" or
//     "plan_change") until it is settled.
//   - "task_too_large" — this one request is larger than credits can run.
//     Shorten it or start a new chat.
//
// A key of your own still runs the turn. Distinct from
// BundledLlmBudgetExhaustedError, the earlier plans' monthly cap.
type AiCreditsExhaustedError struct {
	apiError
	Reason           string
	DebtReason       string
	AvailableCredits int
	RequiredCredits  int
	DebtCredits      int
	// ResetsAt is ISO-8601, when known; empty otherwise.
	ResetsAt string
}

func (e *AiCreditsExhaustedError) Is(target error) bool {
	return target == ErrAiCreditsExhausted
}

// TrialEndedError — 402: the account's free trial has ended, so it cannot start
// a session until it chooses a plan. Every other call keeps working, and the
// account keeps its profiles, proxies and settings.
type TrialEndedError struct{ apiError }

func (e *TrialEndedError) Is(target error) bool { return target == ErrTrialEnded }

// Pair-mode takeover lock contention.
// WinnerClientID surfaces the holder; loser can show "X is taking over".
type PairModeConflictError struct {
	apiError
	WinnerClientID string
}

func (e *PairModeConflictError) Is(target error) bool { return target == ErrPairModeConflict }

// Invalid pair-mode transition.
// From + Transition carry the state-machine diagnostic context.
type PairModeStateInvalidTransitionError struct {
	apiError
	From       string
	Transition string
}

func (e *PairModeStateInvalidTransitionError) Is(target error) bool {
	return target == ErrPairModeStateInvalidTransition
}

// IsRetryable is the public retry predicate. Mirrors the TS /
// Python implementations. Returns true when err is a Driftstack
// error whose kind is retryable; false otherwise.
//
// Retryable: TransportError, InternalError, RateLimitError,
// SessionNotReadyError (the agent session's browser is still starting and
// nothing ran; wait RetryAfterSeconds, then send again).
// NOT retryable: ValidationError, AuthError, NotFoundError,
// ConflictError, ConcurrencyLimitError, all auth-flow errors,
// FeatureUnavailableError, MfaStepUpRequiredError.
//
// Use this from your own retry/backoff loop when the built-in
// retry in retry.go doesn't fit. Honour the Retry-After hint on
// RateLimitError.RetryAfterSeconds when set.
//
// Non-Driftstack errors return false — the SDK wraps known errors
// in a typed Driftstack error, so a non-Driftstack error is
// something the caller produced and the caller should decide.
//
//	for attempt := 0; attempt < 5; attempt++ {
//	    sess, err := client.Sessions.Create(ctx, opts)
//	    if err == nil {
//	        return sess
//	    }
//	    if !driftstack.IsRetryable(err) {
//	        return nil, err
//	    }
//	    var rl *driftstack.RateLimitError
//	    if errors.As(err, &rl) && rl.RetryAfterSeconds > 0 {
//	        time.Sleep(time.Duration(rl.RetryAfterSeconds) * time.Second)
//	    } else {
//	        time.Sleep(backoff(attempt))
//	    }
//	}
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var transport *TransportError
	if errors.As(err, &transport) {
		return true
	}
	var internal *InternalError
	if errors.As(err, &internal) {
		return true
	}
	var rateLimit *RateLimitError
	if errors.As(err, &rateLimit) {
		return true
	}
	var notReady *SessionNotReadyError
	if errors.As(err, &notReady) {
		return true
	}
	return false
}
