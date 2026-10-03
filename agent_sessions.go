package driftstack

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

// AgentSessionsResource provides typed access to /v1/agent-sessions
// and its control subresources. An agent session is a browser the AI drives
// for you: Create one, send it a task with Message, read the outcome, and
// Close it.
//
// Availability depends on the deployment's agent-runtime configuration.
// Unsupported deployments return a typed FeatureUnavailable error.
type AgentSessionsResource struct {
	client *Client
}

// LiveKitInfo is the live-video join info returned on session-create
// (when live video is available) and by the dedicated
// POST /v1/agent-sessions/:id/livekit-token endpoint (LivekitToken). Use
// with the official livekit-server-sdk-go consumer side.
//
// Token TTL is 24h; the token expires at ExpiresAt. Room name is always
// the agent_session id.
type LiveKitInfo struct {
	WSURL               string `json:"ws_url"`
	Room                string `json:"room"`
	Token               string `json:"token"`
	ParticipantIdentity string `json:"participant_identity"`
	ExpiresAt           string `json:"expires_at"`
}

// AgentSession is the read envelope returned by Create / Get / and as
// the .Session field of every Message response.
type AgentSession struct {
	ID                  string  `json:"id"`
	AccountID           string  `json:"account_id"`
	DriftstackSessionID *string `json:"driftstack_session_id"`
	Status              string  `json:"status"`
	ClosedReason        *string `json:"closed_reason"`
	ProvisioningDetail  *string `json:"provisioning_detail,omitempty"`
	// Ready is whether the session's browser has finished starting: true once
	// the session has reported it ready. Status reads "active" from
	// the moment the session is created, before that. Wait for IsReady before
	// sending the first message (a message sent earlier waits, then may be
	// refused with SessionNotReadyError). A VPN session can take longer to
	// become ready. A session that closes while Ready is false failed to start;
	// ClosedReason says why. nil when an older server does not send it — that
	// server does not hold messages either, so IsReady reads nil as ready.
	Ready *bool `json:"ready,omitempty"`
	// ReadyAt is when the session reported its browser ready (ISO-8601); nil
	// until then, on a session that never became ready, on one that never
	// reports, and from an older server.
	ReadyAt              *string `json:"ready_at,omitempty"`
	TokenBudgetTotal     int     `json:"token_budget_total"`
	TokenBudgetRemaining int     `json:"token_budget_remaining"`
	TranscriptLength     int     `json:"transcript_length"`
	// ClosedAt is the ISO-8601 time the session left "active"; nil while
	// active. Distinct from UpdatedAt, which moves on every message.
	ClosedAt *string `json:"closed_at"`
	// CreatedByUserID is the team member who created the session; nil when
	// the API key is not tied to one.
	CreatedByUserID *string `json:"created_by_user_id"`
	// Mode is how the session is driven: "ai" (the default) for a session
	// your program runs. "manual" and "pair" sessions are driven by a person
	// in the Driftstack desktop app.
	Mode string `json:"mode"`
	// Model is the Claude model the AI runs for this session (set at create;
	// defaults to "claude-sonnet-5").
	Model string `json:"model"`
	// StopOnExitIPChange is whether the session stops when its exit IP
	// changes (set at create; default false).
	StopOnExitIPChange bool `json:"stop_on_exit_ip_change"`
	// ProfileID — the saved profile this session runs ("prof_…"), or nil for
	// a session with no profile. Lets a second computer show a profile as
	// running. Absent from older servers.
	ProfileID *string `json:"profile_id"`
	// PairModeState is nil unless the session is a desktop-app "pair"
	// session, where it says whether a person has taken over from the AI.
	//
	// Deprecated: not part of the public API — it describes the desktop
	// app's takeover state, and a program's own sessions are "ai". It may be
	// removed in a later release.
	PairModeState map[string]any `json:"pair_mode_state,omitempty"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	// LiveKit is the live-video join info, returned on create when live video
	// is available for the session; nil otherwise. Mint one at any time with
	// LivekitToken.
	LiveKit *LiveKitInfo `json:"livekit,omitempty"`
	// Liveness says whether the browser behind this session is still
	// reporting in. Distinct from Status, which stays "active" until close
	// even if the browser has stopped. nil when nothing has been reported —
	// treat nil as "unknown", never "dead".
	Liveness *SessionLiveness `json:"liveness,omitempty"`
	// CapabilityReport is the latest report of what this live session can
	// do; nil when no report has arrived.
	CapabilityReport *AgentSessionCapabilityReport `json:"capability_report,omitempty"`
	// ProxyID is the id of the account proxy this session's traffic goes out
	// through: the create's proxy_id, or the one a successful
	// POST /v1/agent-sessions/{id}/egress moved it to — never what a profile
	// has been set to use since. Every agent session created since proxy_id
	// became required has one; nil only on an older session created without a
	// proxy of yours, and when an older server does not send the field.
	ProxyID *string `json:"proxy_id,omitempty"`
	// UploadMaxFileBytes is the largest file, in bytes, one upload to this
	// session can carry right now. Each session takes a file up to its own size,
	// so this can be smaller than the 64 MiB per-file maximum. Set by Get while
	// the session is running and has reported it; nil means not known.
	UploadMaxFileBytes *int64 `json:"upload_max_file_bytes,omitempty"`
	// The structured reason a session degraded or failed. nil when nothing has
	// gone wrong. Carries Severity, CustomerActionable and Retryable, which is
	// what a caller needs to decide whether to surface the failure to a human or
	// simply try again — none of which was reachable from Go before.
	ErrorEvent *AgentSessionErrorEvent `json:"error_event,omitempty"`
}

// IsReady reports whether the session's browser has finished starting: Ready,
// or true when the server did not say (an older server, which does not hold
// messages for a session that is still starting).
func (s *AgentSession) IsReady() bool {
	return s.Ready == nil || *s.Ready
}

// AgentSessionCapabilityReport is the latest report of what a session can do.
// Pointer fields are the ones the API models as nullable: nil means "not
// reported", which is distinct from a zero value.
type AgentSessionCapabilityReport struct {
	Timestamp            string `json:"timestamp"`
	ManualInputAvailable *bool  `json:"manual_input_available"`
	// "provisioning" | "live" | "blank" | "failed", or nil when unreported.
	StreamingState *string `json:"streaming_state"`
	// "live" | "dead_proxy" | "default_connection_down", or nil when
	// unreported. "default_connection_down" appears only on a session created
	// before proxy_id became required, with no proxy of its own: the
	// connection Driftstack provided then stopped. Every agent session now runs
	// through one of your proxies.
	EgressState *string `json:"egress_state"`
	// "socks5" | "openvpn" | "wireguard".
	ProxyKind         string `json:"proxy_kind"`
	ProxyUDPSupported bool   `json:"proxy_udp_supported"`
	// "h2-only" | "h2-and-h3".
	TransportModeRequested string `json:"transport_mode_requested"`
	TransportModeActive    string `json:"transport_mode_active"`
	SafeguardsPassed       bool   `json:"safeguards_passed"`
	// The live exit identity this session's traffic leaves through, and the
	// IPs its WebRTC candidates surface. Pointer/slice fields are nil when NOT
	// OBSERVED (not reported yet), distinct from a zero value.
	ExitIP             *string  `json:"exit_ip"`
	ExitCountry        *string  `json:"exit_country"`
	ExitTimezone       *string  `json:"exit_timezone"`
	WebRTCCandidateIPs []string `json:"webrtc_candidate_ips"`
	ObservedAt         *string  `json:"observed_at"`
}

// AgentSessionErrorEvent is the structured failure report for a session.
//
// Severity is "info" | "warn" | "error" | "fatal". CustomerActionable says
// whether a human can do anything about it; Retryable says whether the same
// call is worth repeating. Detail is nil when the server has nothing to add
// beyond Summary.
type AgentSessionErrorEvent struct {
	Timestamp          string  `json:"timestamp"`
	Code               string  `json:"code"`
	Severity           string  `json:"severity"`
	Summary            string  `json:"summary"`
	Detail             *string `json:"detail"`
	CustomerActionable bool    `json:"customer_actionable"`
	Retryable          bool    `json:"retryable"`
}

// SessionLiveness is the reported liveness of the browser behind an agent
// session. State is its latest state ("active" | "provisioning" | "idle" |
// "terminating") or nil when the server reports null (seen but no live state).
// Fresh is whether that report is recent enough to trust.
type SessionLiveness struct {
	State *string `json:"state"`
	Fresh bool    `json:"fresh"`
}

// CreateAgentSessionRequest is the optional body for Create.
type CreateAgentSessionRequest struct {
	DriftstackSessionID string `json:"driftstack_session_id,omitempty"`
	// TokenBudget is the tokens the AI may spend over the whole session.
	// Zero omits it (default 100,000; at most 10,000,000). When it runs out
	// the session closes with ClosedReason "budget-exhausted".
	TokenBudget int `json:"token_budget,omitempty"`
	// Mode is how the session is driven. Empty string
	// omits the field on the wire so the server applies its default
	// ('ai': the AI plans and runs each message). "manual" records messages
	// for a person driving the browser; "pair" lets a person take over.
	Mode string `json:"mode,omitempty"`
	// Model is the Claude model the AI runs. Empty string omits the field so
	// the server applies its default ("claude-sonnet-5").
	// Valid: "claude-fable-5-1" | "claude-opus-5-5" | "claude-opus-5" |
	// "claude-sonnet-5" | "claude-opus-4-8" | "claude-opus-4-7" |
	// "claude-sonnet-4-6" | "claude-haiku-4-5". Opus models and Claude Fable
	// 5.1 run only on your own Anthropic key: when the session would run on
	// Driftstack's included AI, Create returns a 403 *ForbiddenError whose
	// RequiresOwnKey() is true.
	Model string `json:"model,omitempty"`
	// Attach a saved profile (persistent browser identity) so the session
	// resumes its stored state + saves back on end. Must be an owned profile id
	// (unknown/not-owned → 404); a profile can have one live session at a time
	// (409 *ProfileInUseError otherwise), and a profile whose device is not
	// offered right now is refused (409 *DeviceUnavailableError). Empty string
	// omits it (stateless session).
	ProfileID string `json:"profile_id,omitempty"`
	// REQUIRED. The id of one of your saved proxies (client.Egress.ListProxies
	// lists them; it needs a key with the account_owner scope): every session's
	// traffic goes out through a proxy you chose. Left empty, the create is
	// refused with a 422 *ProxyRequiredError and nothing is created. Must be an
	// owned proxy id (a team admin may also use one saved on their own account;
	// unknown/not-owned → 404).
	// The proxy is tested before launch (422 *ProxyValidationFailedError when
	// it fails).
	ProxyID string `json:"proxy_id,omitempty"`
	// SkipProxyProbe skips the pre-launch proxy test for this launch only —
	// for a proxy you know works but the test reports as unreachable.
	SkipProxyProbe bool `json:"skip_proxy_probe,omitempty"`
	// ContinueFromAgentSessionID carries a CLOSED session's conversation into
	// the new one, so the agent still has it. Unknown or not owned → 404; not
	// closed yet → 409. Empty string omits it.
	ContinueFromAgentSessionID string `json:"continue_from_agent_session_id,omitempty"`
	// A start page for the browser. Must be an absolute http(s) URL; file:,
	// javascript:, data: schemes are rejected (400). For an AI task, also put
	// the URL in your message. Empty string omits it.
	InitialURL string `json:"initial_url,omitempty"`
	// Explicit geolocation override. By default the device's
	// navigator.geolocation derives from the proxy exit IP (coherent with the
	// session's apparent network location) — omit this for most sessions.
	// Supply it only when you know the proxy's true physical location better
	// than IP geolocation; coordinates diverging from the exit country make
	// the fingerprint internally inconsistent (a detection signal).
	Geolocation *SessionGeolocation `json:"geolocation,omitempty"`
	// End the session if its exit IP changes mid-run. When true, the first
	// exit IP seen for the session is remembered and the session stops the
	// moment a later report shows a different one (ClosedReason
	// "exit_ip_changed"). Omit (false) → default.
	StopOnExitIPChange bool `json:"stop_on_exit_ip_change,omitempty"`
}

// SessionGeolocation is the explicit per-session geolocation override.
// Latitude -90..90, Longitude -180..180; Accuracy is meters (nil → device
// default).
type SessionGeolocation struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Accuracy  *float64 `json:"accuracy,omitempty"`
}

// AgentUsage is the per-turn usage/cost block attached by the server on
// every Claude-backed message response (DecomposerKind == "claude");
// deterministic turns set DecomposerKind == "deterministic" with the
// token/cost fields absent (nil). Surface it as a
// "$0.0023 · 145 tok · <model>" badge; render "—" when the pointer
// fields are nil. Mirrors the TS SDK's AgentUsage field-for-field.
type AgentUsage struct {
	DecomposerKind        string  `json:"decomposer_kind"`
	AnthropicInputTokens  *int    `json:"anthropic_input_tokens,omitempty"`
	AnthropicOutputTokens *int    `json:"anthropic_output_tokens,omitempty"`
	CostUSDCents          *int    `json:"cost_usd_cents,omitempty"`
	Model                 *string `json:"model,omitempty"`
}

// AgentMessageResponse is the discriminated turn-result. Branch on
// Kind: "plan-executed" (Intents + Results + OK populated),
// "clarify" (ClarifyingQuestion populated), "refuse"
// (RefuseReason populated), or "stopped" (the turn was stopped with
// Stop: Intents + Results are the steps that ran, Notice says how far it
// got, StoppedDuring what it was doing). A "manual"-mode session answers
// "logged-manual" with only Session set. Treat any other Kind as a result
// this SDK version does not know.
//
// On "plan-executed", Answer is what you asked for (when you asked for
// information), AnswerUnavailable says why there is none when one could not be
// produced, Notice, when set, says why the task is not finished yet, and
// NoticeReason says the same in one word you can switch on.
// OK is true when the last planned steps ran cleanly; it does not by itself
// mean the task is finished. It is false when a step failed (a tap that changed
// nothing on the page is a failed step, Diagnosis.Category "no_effect"), when a
// step is waiting for approval, and whenever NoticeReason is "no_progress" —
// the turn stopped because nothing was changing, so it did not succeed. A
// planned step that would have acted on the page but could not be sent is a
// failed step too (Diagnosis.Category "invalid_request"), and OK stays false
// after it unless a later step acts on the page: a screenshot, a read or a
// pause after it does not make up for it, so the last entry of Results can
// then be a success. OK is false, too, when the turn delivered nothing — no
// step that changed the page or scrolled it, no screenshot and no Answer —
// unless the message only asked to wait, and when an English message asks for
// more things to be done than distinct steps that changed the page ran: every
// entry of Results can then be a success, and there is no failed step to find
// — read Notice, or AnswerUnavailable when the answer asked for could not be
// read. Read typed steps with ParsedResults.
type AgentMessageResponse struct {
	Kind    string       `json:"kind"`
	Session AgentSession `json:"session"`
	// Intents are every step the turn attempted, in order, across every plan
	// it made. Read each step's outcome from Results, which carries the step it
	// ran; the two need not line up by index — Intents is longer when a plan
	// was abandoned part-way, because the steps that did not run have no
	// result. Decode them with ParsedIntents.
	Intents []json.RawMessage `json:"intents,omitempty"`
	// Results are every step that ran, in order. Decode them with
	// ParsedResults.
	Results            []json.RawMessage `json:"results,omitempty"`
	OK                 bool              `json:"ok,omitempty"`
	ClarifyingQuestion string            `json:"clarifying_question,omitempty"`
	// RefuseReason says why the agent will not do this. A refuse can also
	// mean the AI was briefly unavailable; the session stays active and you
	// can send the message again. When the refusal was not the agent's choice,
	// NoticeReason says why in one word (see there).
	RefuseReason string `json:"refuse_reason,omitempty"`
	// Answer is the agent's answer to the question the turn asked, read back
	// from the page; empty when the turn only acted (navigate, tap,
	// screenshot) or no answer could be read.
	Answer string `json:"answer,omitempty"`
	// AnswerUnavailable says why there is no Answer, when the message asked
	// for information and none could be produced: one sentence, in plain
	// words. Never set together with Answer, and empty on a message that only
	// asked for actions. Open text: show it, do not match on it.
	AnswerUnavailable string `json:"answer_unavailable,omitempty"`
	// Notice is set on a "plan-executed" turn that ended before the task was
	// finished — it reached a limit on steps, time or budget, stopped rather
	// than repeat itself, or the session stopped answering automated steps —
	// or when the agent asked you something part-way through; when it asks for
	// "continue", send that as the next message. On a "stopped" turn it is one
	// sentence saying how far it got.
	Notice string `json:"notice,omitempty"`
	// NoticeReason is the same ending as Notice, in one word to switch on. Set
	// on a "plan-executed" turn whenever Notice is, and never without it:
	//
	//	"step_limit"     the task needs more steps than one message runs; send "continue"
	//	"time_limit"     the message was taking too long; send "continue"
	//	"budget_low"     too little of the session's AI budget is left; start a new session
	//	"no_progress"    the page stopped changing and the next step would repeat; ask a person (OK is false)
	//	"repeated_step"  the next step would repeat an action that already ran; check, then "continue"
	//	"ai_unavailable" the next steps could not be worked out just now; send "continue" to try again
	//	"page_unreadable" the page could not be read to plan the next step; send "continue" to try again
	//	"session_unresponsive" this session stopped answering automated steps (the live view may still show the page); end the session and launch a new one ("continue" will not help)
	//	"question"       the agent asked you something part-way; Notice is the question, answer it
	//	"declined"       the agent stopped rather than carry on; a person should decide
	//
	// Five say the session is PAUSED, and nothing carries on until
	// it is resumed (Resume, or the live view's Resume button):
	//
	//	"challenge_solver_pending"   a verification check was handed to your configured solver; when the page clears, resume and send "continue"
	//	"challenge_customer_handoff" a verification check needs a person; solve it in the live view, resume, send "continue"
	//	"verification_email"         the site sent a code by email; enter it in the live view, resume, send "continue"
	//	"verification_sms"           the site sent a code by text message; the same
	//	"unexplained_pause"          the session is paused and no check is showing, or the reason could not be confirmed; open the live view, resume, send "continue"
	//
	// The set is OPEN: a turn can end a way this SDK version has never heard
	// of, so a default branch that shows Notice is required, not optional.
	// Empty on older servers.
	//
	// On a "refuse" turn NoticeReason is set only when the refusal was not the
	// agent's choice, and Notice is empty there (RefuseReason is the
	// sentence): "session_unresponsive" — this session stopped answering
	// automated steps (the live view may still show the page) before anything
	// was planned, so nothing was done with the message; end the session and
	// launch a new one. Empty on every other refusal.
	NoticeReason string `json:"notice_reason,omitempty"`
	// StoppedDuring is what a "stopped" turn was doing when it noticed the
	// stop: "planning", "executing", "reading_page" or "answering".
	StoppedDuring string `json:"stopped_during,omitempty"`
	// Usage is the per-turn usage/cost block (nil on older servers or
	// turns that omit it).
	Usage *AgentUsage `json:"usage,omitempty"`
}

// AgentIntent is one step the agent planned. Kind is "navigate", "interact",
// "wait", "capture", "scroll", "behavioral_pause", "back", "extract" or
// "tap_at"; only the fields for that kind are set. The set of kinds is open:
// treat one you do not recognise as a step you cannot describe, not as an
// error.
//
// AgentIntent cannot be compared with == or used as a map key, because Frame
// is a slice: compare the fields you need (slices.Equal for Frame), or whole
// values with reflect.DeepEqual.
type AgentIntent struct {
	Kind string `json:"kind"`
	// navigate
	URL string `json:"url,omitempty"`
	// interact: Action is "tap" | "type" | "scroll" | "swipe" | "press".
	// Sensitive is true on a "type" step whose value (a card number, a
	// one-time code, a PIN) is withheld from the response.
	// On a "type" step Value is added to what the field holds, and an empty
	// Value clears the field instead (send an empty one, then the new text,
	// to replace what it held).
	Action    string `json:"action,omitempty"`
	Selector  string `json:"selector,omitempty"`
	Value     string `json:"value,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	// wait: Condition is "idle" | "selector_visible" (Selector for the latter).
	Condition string `json:"condition,omitempty"`
	TimeoutMs *int   `json:"timeoutMs,omitempty"`
	// capture: "screenshot" | "dom_snapshot" | "pdf".
	Capture string `json:"capture,omitempty"`
	// Frame, only on a "dom_snapshot" capture, a whole-page "extract"
	// (Body true, no Selector) or an "interact" that types — and, on a session
	// whose device can act inside a frame, an "interact" that taps and a
	// "selector_visible" "wait" (never a scroll or a key press): the embedded
	// document (an iframe) that was read, typed or tapped into, or waited in
	// instead of the page, as a path of positions — its place among the page's
	// frames, then its place inside that frame for a nested one, outermost
	// first. Positions follow the order the browser created the frames, which
	// is not always the order of the iframe tags in the markup. Empty: the
	// page itself.
	Frame []int `json:"frame,omitempty"`
	// scroll: Direction is "up" | "down".
	Direction string `json:"direction,omitempty"`
	AmountPx  *int   `json:"amount_px,omitempty"`
	// behavioral_pause
	DurationMs       *int `json:"duration_ms,omitempty"`
	ReadingWordCount *int `json:"reading_word_count,omitempty"`
	// extract: the text of one element (Selector) or of the whole page (Body
	// true) — exactly one of the two.
	Body bool `json:"body,omitempty"`
	// tap_at: the point tapped, in viewport pixels from the top-left corner.
	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`
}

// AgentFailureDiagnosis is the machine-readable companion to a failed step's
// Reason. Category is an open set ("element_not_found", "page_load_failed",
// "condition_not_met", "capture_failed", "scroll_failed", "session_error",
// "invalid_request", "result_too_large", "element_covered",
// "target_unverified", "credential_site_not_allowed", "session_unresponsive",
// "no_effect", "unknown", and more over time): treat a value you do not
// recognise as "unknown".
// "no_effect" means a tap WAS made and nothing on the page changed in
// response: whatever the step was for did not happen; look at the page and try
// something else.
// "credential_site_not_allowed" means a saved credential was not typed because
// the page was not an https:// page on a website the credential is saved for.
// "session_unresponsive" means this session stopped answering automated steps
// (the live view may still show the page), so the step was not sent; it is
// never retryable — end the session and launch a new one. Retryable true
// means replaying the same step automatically is safe; false means never
// auto-replay — the request may need correcting, or the step's outcome is
// unknown and the page must be checked.
type AgentFailureDiagnosis struct {
	Category  string `json:"category"`
	Retryable bool   `json:"retryable"`
}

// AgentStepWarning is something worth knowing about a step that SUCCEEDED;
// a nil Warning means there is nothing to report. Kind is an open set — treat
// a value you do not recognise as a note and read the step's Summary. Today's
// kinds are "http_error_status": a navigation reached the site and the site
// answered with an HTTP status of 400 or above, carried in Status. The step
// still succeeded: the page that loaded may be an error page, a page asking to
// sign in or to complete a verification step, or the whole page served under
// that status, and Summary says what the site answered. And "effect_unknown":
// a tap was made, and whether it changed anything on the page could not be
// checked — do not count the step as having done what it was for until the
// page shows it. (A tap known to have changed nothing is a "failure" with
// Diagnosis.Category "no_effect", never a success.)
type AgentStepWarning struct {
	Kind   string `json:"kind"`
	Status *int   `json:"status,omitempty"`
}

// AgentIntentResult is the outcome of one step. Kind is "success" (Summary,
// CaptureID for a capture, and Warning when there is something worth knowing),
// "failure" (Reason, and Diagnosis on current servers) or
// "confirmation_required": the agent stopped BEFORE a purchase, a payment or
// an account deletion and is waiting for your approval (Category,
// MatchedText). Approve it by sending the next message with
// ApproveConsequentialActions: []ConsequentialActionApproval{ApprovalFor(r)}.
// Kind and Category are open sets. It holds an AgentIntent, so like that type
// it cannot be compared with == or used as a map key.
type AgentIntentResult struct {
	Kind        string                 `json:"kind"`
	Intent      AgentIntent            `json:"intent"`
	Summary     string                 `json:"summary,omitempty"`
	CaptureID   string                 `json:"captureId,omitempty"`
	Warning     *AgentStepWarning      `json:"warning,omitempty"`
	Reason      string                 `json:"reason,omitempty"`
	Diagnosis   *AgentFailureDiagnosis `json:"diagnosis,omitempty"`
	Category    string                 `json:"category,omitempty"`
	MatchedText string                 `json:"matchedText,omitempty"`
}

// ParsedResults decodes Results into typed step outcomes.
func (r *AgentMessageResponse) ParsedResults() ([]AgentIntentResult, error) {
	out := make([]AgentIntentResult, 0, len(r.Results))
	for _, raw := range r.Results {
		var res AgentIntentResult
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

// ParsedIntents decodes Intents into typed steps.
func (r *AgentMessageResponse) ParsedIntents() ([]AgentIntent, error) {
	out := make([]AgentIntent, 0, len(r.Intents))
	for _, raw := range r.Intents {
		var intent AgentIntent
		if err := json.Unmarshal(raw, &intent); err != nil {
			return nil, err
		}
		out = append(out, intent)
	}
	return out, nil
}

// ApprovalFor turns a "confirmation_required" step result into the approval
// that releases it (the result spells the text MatchedText; the request field
// is matched_text).
func ApprovalFor(res AgentIntentResult) ConsequentialActionApproval {
	return ConsequentialActionApproval{Category: res.Category, MatchedText: res.MatchedText}
}

// AgentStepEvent is one step as it lands on a turn's stream: Index is the
// step's 0-based position in the final Results. It holds an AgentIntentResult,
// so like that type it cannot be compared with == or used as a map key.
type AgentStepEvent struct {
	Index  int               `json:"index"`
	Result AgentIntentResult `json:"result"`
}

// CreateOptions carries optional per-call overrides for Create.
//
// IdempotencyKey is the Stripe-pattern idempotency token.
// Forwarded as the Idempotency-Key request header so retries collapse
// onto the same server-side row. Server enforces (account_id,
// idempotency_key) uniqueness via a partial unique index; SDK just
// plumbs the header.
//
// ByokAPIKey is your own Anthropic API key, sent as the
// x-byok-anthropic-api-key header. Create uses it only to decide whether an
// Opus model is allowed (Opus runs only on your own key); send it on every
// Message too. NEVER logged.
type CreateOptions struct {
	IdempotencyKey string
	ByokAPIKey     string
}

// Create starts a new agent session. While the returned session's Status is
// "provisioning" its browser is still starting: poll Get until it reads
// "active" before sending a message ("closed" means it could not start — read
// ClosedReason).
//
// Pass `nil` for opts to skip the Idempotency-Key and key headers.
//
// Errors: 429 *ConcurrencyLimitError (your plan's concurrent-session limit),
// 409 *ProfileInUseError / *DeviceUnavailableError / *StorageQuotaExceededError, 422
// *ProxyValidationFailedError, 403 *ForbiddenError (no AI on the plan, or an
// Opus model without your own key — RequiresOwnKey), 404 *NotFoundError.
func (r *AgentSessionsResource) Create(ctx context.Context, body *CreateAgentSessionRequest, opts *CreateOptions) (*AgentSession, error) {
	var out AgentSession
	if body == nil {
		body = &CreateAgentSessionRequest{}
	}
	req := requestOptions{
		method: "POST",
		path:   "/v1/agent-sessions",
		body:   body,
		out:    &out,
	}
	if opts != nil {
		headers := map[string]string{}
		if opts.IdempotencyKey != "" {
			headers["Idempotency-Key"] = opts.IdempotencyKey
		}
		if opts.ByokAPIKey != "" {
			headers["x-byok-anthropic-api-key"] = opts.ByokAPIKey
		}
		if len(headers) > 0 {
			req.headers = headers
		}
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get reads agent session state.
func (r *AgentSessionsResource) Get(ctx context.Context, agentSessionID string) (*AgentSession, error) {
	var out AgentSession
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID),
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// AgentSessionsListPage is the GET /v1/agent-sessions envelope — newest
// first, cursor-paginated (the standard { data, has_more, next_cursor }
// shape shared by recipes / crypto-orders). Was a non-paginated { data }
// hard-capped at 100, leaving older sessions unreachable.
type AgentSessionsListPage struct {
	Data       []AgentSession `json:"data"`
	HasMore    bool           `json:"has_more"`
	NextCursor *string        `json:"next_cursor"`
}

// ListAgentSessionsQuery holds the pagination knobs for List / Iterate.
type ListAgentSessionsQuery struct {
	Limit  int
	Cursor string
}

// List returns a page of the account's agent sessions, newest first. Pass nil
// for defaults; pass a Cursor (the prior page's NextCursor) to page. Mirrors
// the TS + Python SDK list().
func (r *AgentSessionsResource) List(ctx context.Context, query *ListAgentSessionsQuery) (*AgentSessionsListPage, error) {
	var out AgentSessionsListPage
	q := url.Values{}
	if query != nil {
		if query.Limit > 0 {
			q.Set("limit", strconv.Itoa(query.Limit))
		}
		if query.Cursor != "" {
			q.Set("cursor", query.Cursor)
		}
	}
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/agent-sessions",
		query:  q,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// Iterate yields every agent session across cursor pages (newest first). The
// callback returns false to stop early; an error from it is propagated back.
// Replaces the old hard 100-cap — a busy account can now reach its full
// AI-session history.
func (r *AgentSessionsResource) Iterate(ctx context.Context, query *ListAgentSessionsQuery, fn func(*AgentSession) (bool, error)) error {
	cursor := ""
	limit := 0
	if query != nil {
		limit = query.Limit
		cursor = query.Cursor
	}
	for {
		page, err := r.List(ctx, &ListAgentSessionsQuery{Limit: limit, Cursor: cursor})
		if err != nil {
			return err
		}
		for i := range page.Data {
			cont, err := fn(&page.Data[i])
			if err != nil {
				return err
			}
			if !cont {
				return nil
			}
		}
		next, done, err := advanceCursor(cursor, page.NextCursor)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		cursor = next
	}
}

// ConsequentialActionApproval approves an action a previous turn stopped on
// (a "confirmation_required" step result), so the paused steps continue.
// Category + MatchedText echo the result's fields; ApprovalFor builds one.
type ConsequentialActionApproval struct {
	Category    string `json:"category"`
	MatchedText string `json:"matched_text"`
}

// MessageOptions carries optional per-call overrides for Message.
//
// ByokAPIKey is your own Anthropic API key. Forwarded as the
// x-byok-anthropic-api-key request header so callers don't construct it by
// hand. It takes precedence over a stored key and over Driftstack's included
// AI. NEVER logged.
//
// ApproveConsequentialActions approves the actions the previous turn stopped
// on (omitted from the request body when empty). Send it as the very next
// message on the session: the paused steps then continue from where they
// stopped, without planning again. Any other message in between discards the
// paused steps, and the agent plans afresh.
type MessageOptions struct {
	ByokAPIKey string
	// IdempotencyKey identifies one logical turn. Reuse it after a lost or
	// ambiguous stream so the server replays the durable terminal result instead
	// of executing browser actions twice.
	//
	// A refusal raised BEFORE the turn did any work gives the key back, so the
	// same key runs the turn once the cause is gone: a *ConflictError whose
	// TurnInProgress() is true, a *RateLimitError (the message rate, or too
	// many AI turns running at once), ErrBundledLlmConsentRequired,
	// ErrBundledLlmBudgetExhausted, a *ForbiddenError about the plan's AI or
	// the model (RequiresOwnKey()), and a *ByokAnthropicRequiredError whose
	// KeyRejected() is false. Fix the cause or wait, then send the same request
	// again with the SAME key. So is a *ConflictError whose
	// IdempotencyStatus() is "in_progress": the first attempt is still being
	// resolved, and the same key replays its result.
	//
	// Every other answer is final for that key and sending it again replays it
	// — every completed turn, every failure after the turn started, a rejected
	// own key (KeyRejected()), a 500, a "refuse" result, and the 409 for a
	// session that is closed or paused. To send one of those again, fix the
	// cause and use a NEW key. Change it too when the session, message or
	// approvals change.
	IdempotencyKey              string
	ApproveConsequentialActions []ConsequentialActionApproval
	// Timeout is the absolute heartbeat-stream backstop. Zero uses
	// AgentMessageStreamTimeout (50 minutes). An earlier caller context wins.
	Timeout time.Duration
	// OnStep, when set, is called with each step as it lands, before Message
	// returns. Best-effort: a malformed step frame is skipped.
	OnStep func(step AgentStepEvent)
	// OnEvent, when set, is called for every OTHER progress event on the
	// turn's stream, by name, with its JSON payload — today "phase", "plan",
	// "step_start", "answer" and "notice" (whose data carries the same Notice
	// and NoticeReason the final result does). The set of names is open: ignore the
	// ones you do not recognise. The final result is always Message's return
	// value, never one of these.
	OnEvent func(name string, data json.RawMessage)
}

// Message sends one message — a task or a question — and waits for the
// outcome. The call streams, so it can take several minutes; it returns when
// the turn ends. Branch on the response's Kind.
//
// Pass `nil` for opts when no key, idempotency key or approval is needed.
//
// Errors you should expect: 409 *ConflictError (TurnInProgress(): another
// message is still running — wait, or Stop it; SessionStatus(): the session
// is not active — "closed", with ClosedReason() saying why, so start a new
// one, or "paused"), 429 *RateLimitError (the message rate, or too many AI
// turns running at once: across your sessions, or on Driftstack's included AI
// — no step ran; wait RetryAfterSeconds, then send the same request again,
// the same idempotency key and all; IsRetryable is true), 403 *ForbiddenError (no AI on the
// plan, or RequiresOwnKey()), 402 *BundledLlmBudgetExhaustedError /
// *BundledLlmConsentRequiredError (the included AI's budget is used up, or the
// account has not opted in), and 502 *ByokAnthropicRequiredError: the turn has
// no AI key (a plan that runs AI only on its own key is answered this way
// too), or Anthropic refused your key (KeyRejected(); KeySource() and
// KeyRejectedReason() say which key and why). No step ran, and it is not
// retryable: fix the key first.
//
// A 503 *FeatureUnavailableError on a message sent WITH an idempotency key
// means this deployment cannot record keys at all, so nothing ran. It is NOT
// transient: the same key fails the same way for as long as the deployment is
// in that state, and a retry loop never ends. The same message without an
// idempotency key runs the turn — send it that way only if running the task
// twice would be safe, because that is the protection you are giving up.
func (r *AgentSessionsResource) Message(ctx context.Context, agentSessionID, userMessage string, opts *MessageOptions) (*AgentMessageResponse, error) {
	var out AgentMessageResponse
	body := map[string]any{"user_message": userMessage}
	// Re-send approved consequential actions in the wire's snake_case shape
	// so the paused steps continue. Omitted when empty (matches the route's
	// optional schema + the TS/Python SDKs).
	if opts != nil && len(opts.ApproveConsequentialActions) > 0 {
		body["approve_consequential_actions"] = opts.ApproveConsequentialActions
	}
	req := requestOptions{
		method:        "POST",
		path:          "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/message",
		body:          body,
		out:           &out,
		eventStream:   true,
		streamTimeout: AgentMessageStreamTimeout,
	}
	if opts != nil && opts.Timeout > 0 {
		req.streamTimeout = opts.Timeout
	}
	if opts != nil {
		headers := map[string]string{}
		if opts.ByokAPIKey != "" {
			headers["x-byok-anthropic-api-key"] = opts.ByokAPIKey
		}
		if opts.IdempotencyKey != "" {
			headers["Idempotency-Key"] = opts.IdempotencyKey
		}
		if len(headers) > 0 {
			req.headers = headers
		}
		if opts.OnStep != nil || opts.OnEvent != nil {
			req.onFrame = progressDispatcher(opts.OnStep, opts.OnEvent)
		}
	}
	if err := r.client.doEventStream(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// progressDispatcher routes one progress frame to the caller's callbacks.
func progressDispatcher(onStep func(AgentStepEvent), onEvent func(string, json.RawMessage)) func(string, []byte) {
	return func(name string, data []byte) {
		if name == "step" {
			if onStep == nil {
				return
			}
			var step AgentStepEvent
			if err := json.Unmarshal(data, &step); err != nil {
				return
			}
			onStep(step)
			return
		}
		if onEvent != nil && json.Valid(data) {
			onEvent(name, json.RawMessage(append([]byte(nil), data...)))
		}
	}
}

// AgentCapture is a screenshot fetched with GetCapture. ContentType is
// "image/png" or "image/jpeg" — which one this screenshot is — and Bytes is the
// image itself; write it to a file as-is.
type AgentCapture struct {
	ContentType string
	Bytes       []byte
}

// GetCapture fetches a screenshot the agent took. A "capture" step's result
// carries a CaptureID; this returns the image behind it.
//
// Screenshots are kept only briefly — at most the 20 most recent per session,
// and they can be removed once 30 minutes pass without a new one in that
// session — so fetch one as soon as its turn ends.
//
// Errors: 404 *NotFoundError — the session is unknown, or no screenshot with
// this id is kept for it any more.
func (r *AgentSessionsResource) GetCapture(ctx context.Context, agentSessionID, captureID string) (*AgentCapture, error) {
	var raw rawBody
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/captures/" + url.PathEscape(captureID),
		rawOut: &raw,
	}); err != nil {
		return nil, err
	}
	return &AgentCapture{ContentType: raw.contentType, Bytes: raw.bytes}, nil
}

// AgentTranscriptEntry is one entry of a session's conversation. Role is who
// wrote it: "user" (a message you sent), "agent" (the AI's outcome) or
// "operator" (a message recorded by a "manual"-mode session); an open set.
// Body is always plain text, never JSON. Intents is set on an agent entry
// whose plan ran; sensitive typed values are withheld from it.
type AgentTranscriptEntry struct {
	Role string `json:"role"`
	Body string `json:"body"`
	// At is the ISO-8601 time the entry was written.
	At      string        `json:"at"`
	Intents []AgentIntent `json:"intents,omitempty"`
}

// AgentTranscriptEvent is one item handed to Transcript's callback: the
// entry and its 0-based position in the conversation. Pass the last Index you
// saw as TranscriptOptions.LastEventID to carry on from there.
type AgentTranscriptEvent struct {
	Index int                  `json:"index"`
	Entry AgentTranscriptEntry `json:"entry"`
}

// TranscriptOptions carries optional per-call overrides for Transcript.
type TranscriptOptions struct {
	// LastEventID resumes after this entry index (the Index of the last event
	// you saw): the stream starts with the entry after it, so nothing is
	// repeated. A pointer because 0 is an index: nil replays from the
	// beginning, and a pointer to 0 resumes after entry 0.
	LastEventID *int
	// Timeout is the absolute limit on how long one call may stay open. Zero
	// uses AgentMessageStreamTimeout (50 minutes), the same as Message. It is
	// not an idle timeout. An earlier caller context wins.
	Timeout time.Duration
}

// Transcript reads a session's conversation, then follows it live. fn is
// called with every entry already in the transcript, oldest first, and then
// with each new entry as it is written — so the call does not return by itself
// while the session is open. Return false from fn to stop (Transcript
// then returns nil), or an error to stop with that error; cancelling ctx stops
// it too. Each of these closes the connection.
//
// To read only what is there now, read TranscriptLength with Get first and
// return false at Index == TranscriptLength-1 (skip the call when it is 0).
//
// The call also returns nil when the server closes the stream (your key lost
// access, or the connection was recycled); call again with LastEventID to
// carry on. When the time limit passes it returns context.DeadlineExceeded.
//
// Entries are returned as the session recorded them: Body is free text, and
// may contain whatever was sent to the agent. Treat the transcript as
// sensitive.
//
// Pass nil for opts to replay from the beginning with the default limit.
//
// Errors: 404 *NotFoundError; 429 *RateLimitError — an account may hold at most
// 10 transcript streams open at once (wait RetryAfterSeconds).
func (r *AgentSessionsResource) Transcript(ctx context.Context, agentSessionID string, opts *TranscriptOptions, fn func(AgentTranscriptEvent) (bool, error)) error {
	req := requestOptions{
		method:        "GET",
		path:          "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/transcript",
		eventStream:   true,
		streamTimeout: AgentMessageStreamTimeout,
		onOpenFrame: func(name string, data []byte) (bool, error) {
			// The set of event names is open: anything that is not a transcript
			// entry, or does not decode as one, is skipped, never an error.
			if name != "transcript.entry" {
				return true, nil
			}
			var event struct {
				Index *int                  `json:"index"`
				Entry *AgentTranscriptEntry `json:"entry"`
			}
			if err := json.Unmarshal(data, &event); err != nil || event.Index == nil || event.Entry == nil {
				return true, nil
			}
			return fn(AgentTranscriptEvent{Index: *event.Index, Entry: *event.Entry})
		},
	}
	if opts != nil {
		if opts.Timeout > 0 {
			req.streamTimeout = opts.Timeout
		}
		if opts.LastEventID != nil {
			req.headers = map[string]string{"Last-Event-ID": strconv.Itoa(*opts.LastEventID)}
		}
	}
	return r.client.doEventStream(ctx, req)
}

// Close ends the agent session and its browser (idempotent). Close every
// session you start — an open session keeps counting toward your plan's
// concurrent-session limit.
func (r *AgentSessionsResource) Close(ctx context.Context, agentSessionID string) error {
	return r.client.do(ctx, requestOptions{
		method: "DELETE",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID),
	})
}

// LivekitToken mints a fresh live-video token for the agent session's
// video room. Use this when the LiveKit field on the created session is
// absent OR when the 24h token TTL has expired. Returns the same 5-field
// LiveKitInfo shape that AgentSession.LiveKit carries; one type, two paths.
//
// Errors (mapped to typed Driftstack errors):
//   - 403 — session is closed; cannot mint
//   - 404 — session unknown (or cross-account; existence not leaked)
//   - 503 — live video is not available for this session right now; try
//     again later, or contact support if it persists
func (r *AgentSessionsResource) LivekitToken(ctx context.Context, agentSessionID string) (*LiveKitInfo, error) {
	var out LiveKitInfo
	req := requestOptions{
		method: "POST",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/livekit-token",
		out:    &out,
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// AgentSessionEgressResult is the discriminated result of an egress
// swap. Only Status "ok" means the egress changed; every other
// status leaves the session exactly as it was, with Reason saying why.
// ApplyPoint is present on success and is nil when the session accepted
// the swap without confirming when it takes effect — treat nil as
// possibly-immediate.
type AgentSessionEgressResult struct {
	Status     string  `json:"status"`
	ApplyPoint *string `json:"apply_point,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// SetEgress moves a RUNNING session onto a different egress without
// restarting it.
//
// NOT AVAILABLE YET: no session can change egress while it is running,
// so this currently returns Status "unavailable" for every call —
// create a new session with the proxyID you want instead. It is not in
// the API reference until it works. The shapes are stable and will not
// change when device support lands.
//
// The page keeps its tabs, cookies and scroll position; only the exit
// changes.
//
// proxyID must be a proxy on your own account that has been tested at
// least once: the swap carries the exit's MEASURED identity — IP,
// country, timezone — to the session so the page keeps seeing a
// consistent origin. An untested proxy has no measured identity to
// carry, and the response is status "unavailable" rather than a
// guessed one.
//
// applyPoint may be "" (defaults to "next_navigation", swapping on the
// next page load and leaving connections in flight alone) or
// "immediate", which swaps at once and may reset connections mid-page.
//
// Read Status before assuming anything moved: only "ok" means the
// egress changed.
func (r *AgentSessionsResource) SetEgress(ctx context.Context, agentSessionID, proxyID, applyPoint string) (*AgentSessionEgressResult, error) {
	var out AgentSessionEgressResult
	body := map[string]string{"proxy_id": proxyID}
	if applyPoint != "" {
		body["apply_point"] = applyPoint
	}
	req := requestOptions{
		method: "POST",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/egress",
		body:   body,
		out:    &out,
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResumeAgentSessionRequest is the optional body for Resume. ChallengeID
// (from the session.challenge_detected webhook) targets a specific active
// challenge; leave it empty for a manual override resume.
type ResumeAgentSessionRequest struct {
	ChallengeID string `json:"challenge_id,omitempty"`
}

// ResumeAgentSessionResponse is the 202 acknowledgement returned by Resume.
type ResumeAgentSessionResponse struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
}

// Resume resumes an agent session that paused on a detected bot check (a
// CAPTCHA or challenge page), once you've resolved it (e.g. in the live
// view). The session's Status stays "active" while it is paused; the
// session.challenge_detected webhook tells you it happened. Pass a body with
// ChallengeID to target a specific challenge; pass nil for a manual override
// resume.
//
// Errors (mapped to typed Driftstack errors):
//   - 404 — session unknown (or cross-account; existence not leaked)
//   - 409 — session not active (terminal sessions can't be resumed)
func (r *AgentSessionsResource) Resume(ctx context.Context, agentSessionID string, body *ResumeAgentSessionRequest) (*ResumeAgentSessionResponse, error) {
	if body == nil {
		body = &ResumeAgentSessionRequest{}
	}
	var out ResumeAgentSessionResponse
	req := requestOptions{
		method: "POST",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/resume",
		body:   body,
		out:    &out,
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// StopAgentTurnResponse is returned by Stop. Status is "stop_requested"
// (202: a turn was running and has been asked to stop) or "no_turn_running"
// (200: there was nothing to stop).
type StopAgentTurnResponse struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
}

// Stop stops the session's running turn. It returns as soon as the stop is
// requested and does not wait for the turn to wind down: the turn ends on its
// own Message call, which returns Kind "stopped" (or, if it was already
// finishing, its ordinary result) — that response is the signal that the
// session will accept the next message. A step that was already running when
// the stop arrived is given a short, bounded time to finish so its result is
// known; nothing is started after it. Safe to call again. Because Message
// blocks, call Stop from another goroutine (a timer, for example).
//
// Errors (mapped to typed Driftstack errors):
//   - 404 — session unknown (or cross-account; existence not leaked)
//   - 503 *FeatureUnavailableError — when its StopUnconfirmed() is true, the
//     stop could not be confirmed just now and the turn may still be running:
//     call Stop again. (IsRetryable is false for this type, because the same
//     503 without the flag means AI is not enabled and calling again would not
//     help; the SDK does not retry Stop by itself.)
func (r *AgentSessionsResource) Stop(ctx context.Context, agentSessionID string) (*StopAgentTurnResponse, error) {
	var out StopAgentTurnResponse
	req := requestOptions{
		method: "POST",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/stop",
		body:   struct{}{},
		out:    &out,
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// AgentSessionSecret is a secret registered for one agent session with
// RegisterSecret. No field holds the value: it is write-only, and Driftstack
// puts it only into the step that types it.
type AgentSessionSecret struct {
	// Handle is "sec_<32 hex>". Write {{credential:<handle>}} in a message to
	// this session as the whole value of a step that types it.
	Handle string `json:"handle"`
	Label  string `json:"label"`
	// Sites are the websites it may be typed on (https:// pages on these hosts only).
	Sites             []string `json:"sites"`
	IncludeSubdomains bool     `json:"include_subdomains"`
	CreatedAt         string   `json:"created_at"`
	// ExpiresAt is when its time limit runs out: from then on it is never typed
	// or listed.
	ExpiresAt string `json:"expires_at"`
}

// RegisterAgentSessionSecretRequest is the body of RegisterSecret.
type RegisterAgentSessionSecretRequest struct {
	// Label is your own words for it, 1-120 characters. It is named in place of
	// the value when a step that would type it is refused, in a sentence the
	// model also reads, so put nothing secret in it.
	Label string `json:"label"`
	// Secret is the value: a password, a one-time code or a payment card
	// number, 1-4096 bytes of UTF-8.
	Secret string `json:"secret"`
	// Sites are 1-10 host names it may be typed on, such as "shop.example.com".
	Sites []string `json:"sites"`
	// IncludeSubdomains also allows subdomains of each site. Defaults to false.
	IncludeSubdomains *bool `json:"include_subdomains,omitempty"`
	// TTLSeconds is how long it may be typed, 60-86400. Defaults to 3600. One a
	// step has typed, or tried to, is kept past it, never typed again, until the
	// session ends, so it can still be hidden from the model.
	TTLSeconds *int `json:"ttl_seconds,omitempty"`
}

// RegisterSecret holds a value — a password, a one-time code or a payment card
// number — for this session only, and returns the handle to type it by: write
// {{credential:<handle>}} in a Message as the whole value of the step that
// types it. The value is put into the step only as it is sent to the browser,
// so the plan, the transcript and every response carry the handle, and it is
// typed only on an https:// page on one of Sites. Once typed it is on the page;
// where the page shows it again it is replaced by its placeholder in what the
// model is shown, the step results and the transcript, as for a saved
// credential (a card number however its digits are grouped, but not part of
// it, such as the last four digits).
//
// Held in server memory only, and dropped when the session ends, when you
// delete it, when the service restarts — register it again then — or when
// TTLSeconds runs out (default one hour); one a step has typed, or tried to, is
// kept past TTLSeconds, never typed again, only so it can still be hidden from
// the model. Not retried by the SDK on a network error: a retry could register
// it twice.
//
// Errors (mapped to typed Driftstack errors):
//   - 400 *BadRequestError — the detail names the field, never the value
//   - 404 — session unknown (or cross-account; existence not leaked)
//   - 409 *ConflictError — the session is not active, or already holds 20 secrets
func (r *AgentSessionsResource) RegisterSecret(ctx context.Context, agentSessionID string, body RegisterAgentSessionSecretRequest) (*AgentSessionSecret, error) {
	var out AgentSessionSecret
	req := requestOptions{
		method: "POST",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/secrets",
		body:   body,
		out:    &out,
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSecrets returns the secrets this session holds now — handles, labels,
// sites and times; never a value.
func (r *AgentSessionsResource) ListSecrets(ctx context.Context, agentSessionID string) ([]AgentSessionSecret, error) {
	var out struct {
		Data []AgentSessionSecret `json:"data"`
	}
	req := requestOptions{
		method: "GET",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/secrets",
		out:    &out,
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// DeleteSecret drops a secret now. A step of a turn already running that would
// type it is refused. A handle the session does not hold is a 404.
func (r *AgentSessionsResource) DeleteSecret(ctx context.Context, agentSessionID, handle string) error {
	return r.client.do(ctx, requestOptions{
		method: "DELETE",
		path:   "/v1/agent-sessions/" + url.PathEscape(agentSessionID) + "/secrets/" + url.PathEscape(handle),
	})
}
