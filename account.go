package driftstack

import (
	"context"
)

// AccountResource holds the account reads a program needs: Whoami (which
// account, key, tier and scopes an API key has), RateLimits, Limits (rate and
// counted limits with what is in use), GetBundledLlmStatus and ListCredentials (the handles of the account's saved
// credentials). Me is deprecated.
//
// Everything else about the account — the profile, avatar, sign-ins, MFA, API
// keys, team, billing, notification settings and the AI settings — is managed
// in the Driftstack dashboard and is not part of the public API. Saving and
// deleting a credential belongs to that list too: a person types the value in
// the dashboard. Only the list is here, because a program needs the handle to
// write a placeholder.
type AccountResource struct {
	client *Client
}

// WhoamiResponse — GET /v1/whoami.
type WhoamiResponse struct {
	AccountID string      `json:"account_id"` // acc_<uuid>
	APIKeyID  string      `json:"api_key_id"` // key_<uuid>; a signed-in dashboard session reads wsk_<uuid>
	Tier      AccountTier `json:"tier"`
	Scopes    []string    `json:"scopes"`
}

// Whoami — GET /v1/whoami. The account, key, tier and scopes behind this API
// key. Needs no scope, so it works as a key check for any key. It never
// honors the X-Driftstack-Account header: it always names the key's own
// account, even when WithEffectiveAccount is set.
func (r *AccountResource) Whoami(ctx context.Context) (*WhoamiResponse, error) {
	var out WhoamiResponse
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/whoami",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// AccountTeamMembership — one entry per team the calling
// account is a member of.
type AccountTeamMembership struct {
	OwnerAccountID string  `json:"owner_account_id"`
	OwnerEmail     string  `json:"owner_email"` // owner's email (falls back to acc_<id> server-side)
	OwnerName      *string `json:"owner_name"`  // nullable — owner's display name if set
	Role           string  `json:"role"`        // "admin" | "member"
	MembershipID   string  `json:"membership_id"`
}

// AccountSelfProfile — full /v1/account/me response. Includes all
// the identity, avatar, MFA and quota fields the server adds beyond the
// base AccountSchema. Pointer fields are nullable; absent in the
// JSON means nil.
type AccountSelfProfile struct {
	ID                      string        `json:"id"`
	Email                   string        `json:"email"`
	Name                    *string       `json:"name"`
	Tier                    AccountTier   `json:"tier"`
	Status                  AccountStatus `json:"status"`
	Timezone                *string       `json:"timezone"`                // IANA zone name; null = unset
	Slug                    *string       `json:"slug"`                    // URL-safe account handle; null = unset
	Region                  *string       `json:"region"`                  // "us"|"eu"|"apac"|null
	OnboardingCompletedAt   *string       `json:"onboarding_completed_at"` // ISO instant; null = never
	AvatarURL               *string       `json:"avatar_url"`              // short-lived presigned URL
	AvatarSource            string        `json:"avatar_source"`           // "user"|"idp"|"none"
	MfaEnrolled             bool          `json:"mfa_enrolled"`            // true once TOTP MFA is enrolled
	ConcurrentSessionCap    int           `json:"concurrent_session_cap"`
	ConcurrentSessionActive int           `json:"concurrent_session_active"`
	ProfileCap              *int          `json:"profile_cap"` // null = enterprise
	ProfileCount            int           `json:"profile_count"`
	// ProfileTrashCount — profiles in the Trash. They count toward no limit
	// (since 2026-10-08): ProfileCap and ProfileCount are both live profiles
	// only.
	ProfileTrashCount int `json:"profile_trash_count"`
	// ProfileRestoresUsed / ProfileRestoresLimit — restores from the Trash
	// this account has used in the rolling 30 days, and the number its plan
	// allows (10% of the plan's profiles, never fewer than 3; Enterprise is
	// a flat 100). Both are 0 from a
	// server older than the fields.
	ProfileRestoresUsed  int `json:"profile_restores_used"`
	ProfileRestoresLimit int `json:"profile_restores_limit"`
	// ProxyCap / ProxyCount — the personal account's saved-proxy allowance
	// (nil = unmetered) and usage. A team workspace's ride its proxy list.
	ProxyCap    *int                    `json:"proxy_cap"`
	ProxyCount  int                     `json:"proxy_count"`
	TrialEndsAt *string                 `json:"trial_ends_at"` // ISO instant the free trial ends or ended; null = not on a trial
	TrialEnded  bool                    `json:"trial_ended"`   // true: the trial is over; sign-in works, sessions need a plan
	Teams       []AccountTeamMembership `json:"teams"`         // teams this account belongs to
	// Note: the rich /me response intentionally doesn't include
	// any IP / user-agent fingerprint of the caller.
	_ struct{} // force keyed-struct construction for forward-compat
}

// Me — GET /v1/account/me. Read the calling account's full self-visible state.
// Bearer-authenticated; never honors the X-Driftstack-Account header
// (always returns the caller's own account, even when the caller is
// on a team).
//
// Deprecated: GET /v1/account/me is the dashboard's profile read and is not
// part of the public API. Use Whoami to check which account, key, tier and
// scopes an API key has. Me keeps working until a future minor release.
func (r *AccountResource) Me(ctx context.Context) (*AccountSelfProfile, error) {
	var out AccountSelfProfile
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/account/me",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// RateLimitBucket — per-bucket effective rate-limit config.
type RateLimitBucket struct {
	// "global" | "sessions:create" | "agent_sessions:message" | "agent_sessions:input_event" | "agent_sessions:live" | "agent_sessions:steps"
	BucketKey         string  `json:"bucket_key"`
	Capacity          int     `json:"capacity"`
	RefillPerSecond   float64 `json:"refill_per_second"`
	Source            string  `json:"source"` // "tier_default" | "override"
	OverrideExpiresAt *string `json:"override_expires_at"`
}

type GetAccountRateLimitsResponse struct {
	Tier    string            `json:"tier"`
	Buckets []RateLimitBucket `json:"buckets"`
}

// RateLimits — read effective rate-limit config.
func (r *AccountResource) RateLimits(ctx context.Context) (*GetAccountRateLimitsResponse, error) {
	var out GetAccountRateLimitsResponse
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/account/rate-limits",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// LimitFigure — a token bucket's burst (capacity) and refill a second.
type LimitFigure struct {
	Capacity        int     `json:"capacity"`
	RefillPerSecond float64 `json:"refill_per_second"`
}

// AccountLimitsOverride — a limit agreed for the account that sets a bucket now.
type AccountLimitsOverride struct {
	Capacity        int     `json:"capacity"`
	RefillPerSecond float64 `json:"refill_per_second"`
	ExpiresAt       string  `json:"expires_at"`
	SetOnBucket     string  `json:"set_on_bucket"`
}

// AccountLimitsBucket — one rate-limit bucket: the plan figure, the agreed
// limit (nil when none applies), the figure enforced now and where it comes
// from ("plan" | "override" | "session_cap").
type AccountLimitsBucket struct {
	BucketKey string                 `json:"bucket_key"`
	Plan      LimitFigure            `json:"plan"`
	Override  *AccountLimitsOverride `json:"override"`
	Effective LimitFigure            `json:"effective"`
	Source    string                 `json:"source"`
}

// AccountLimitsCapOverride — a number agreed for the account above the plan's.
type AccountLimitsCapOverride struct {
	Value     int     `json:"value"`
	ExpiresAt *string `json:"expires_at"`
}

// AccountLimitsCap — one counted limit ("concurrent_sessions" |
// "saved_profiles" | "profile_restores_30d" | "saved_proxies"). A nil number
// is none (set by contract) or, for Usage and Remaining, not readable now.
type AccountLimitsCap struct {
	Key       string                    `json:"key"`
	Plan      *int                      `json:"plan"`
	Override  *AccountLimitsCapOverride `json:"override"`
	Effective *int                      `json:"effective"`
	Usage     *int                      `json:"usage"`
	Remaining *int                      `json:"remaining"`
}

type GetAccountLimitsResponse struct {
	Tier       string                `json:"tier"`
	RateLimits []AccountLimitsBucket `json:"rate_limits"`
	Caps       []AccountLimitsCap    `json:"caps"`
}

// Limits — the rate limits and counted limits of the account, with what is
// in use. Not listed: the per-address limit, the tasks and lists of steps
// running at once, AI turns running at once, live transcript streams, a
// session's token budget, profile storage, the longest a session may run
// and, on the Trial, its total session time and whether it has ended.
// Always the key's own account.
func (r *AccountResource) Limits(ctx context.Context) (*GetAccountLimitsResponse, error) {
	var out GetAccountLimitsResponse
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/account/limits",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// BundledLlmStatus — consent + cap + month-to-date spend +
// remaining headroom, for the "you've used $X of $Y" dashboard/GUI display.
type BundledLlmStatus struct {
	Consent               bool   `json:"consent"`
	CapCents              int    `json:"cap_cents"`
	UsedThisMonthCents    int    `json:"used_this_month_cents"`
	RemainingCents        int    `json:"remaining_cents"`
	RefusedCountThisMonth int    `json:"refused_count_this_month"`
	MonthStartedAt        string `json:"month_started_at"`
}

// GetBundledLlmStatus — read consent + cap + month-to-date
// spend + remaining headroom.
func (r *AccountResource) GetBundledLlmStatus(ctx context.Context) (*BundledLlmStatus, error) {
	var out BundledLlmStatus
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/account/me/bundled-llm-status",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// AccountCredential — one saved credential: the handle a task refers to it by,
// the label it was saved under, and the websites it may be typed on. There is
// no field for the value: no endpoint returns a saved value.
//
// Sites is the credential's lock, like a password manager entry: a step types
// it only on an https:// page whose host is one of Sites (or a subdomain of
// one when IncludeSubdomains is true), and is refused anywhere else.
type AccountCredential struct {
	Handle            string   `json:"handle"`             // cred_<32 hex>
	Label             string   `json:"label"`              // the customer's own words for it
	Sites             []string `json:"sites"`              // host names, e.g. "shop.example.com"
	IncludeSubdomains bool     `json:"include_subdomains"` // subdomains of Sites match too
	CreatedAt         string   `json:"created_at"`         // ISO instant
}

// ListAccountCredentialsResponse — GET /v1/account/me/credentials.
type ListAccountCredentialsResponse struct {
	Data []AccountCredential `json:"data"`
}

// ListCredentials — GET /v1/account/me/credentials. The account's saved
// credentials: handles, labels and websites, never the values. Write a handle into an
// agent task as {{credential:<handle>}}, as the text of a type step.
// Credentials are saved and deleted in the Driftstack dashboard, by the account
// owner. With WithEffectiveAccount (a team workspace), a member of either role
// lists the owner's saved credentials — the ones agent tasks in that workspace
// use — as the same fields, never a value.
func (r *AccountResource) ListCredentials(ctx context.Context) (*ListAccountCredentialsResponse, error) {
	var out ListAccountCredentialsResponse
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/account/me/credentials",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}
