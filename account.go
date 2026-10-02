package driftstack

import (
	"context"
)

// AccountResource holds the account reads a program needs: Whoami (which
// account, key, tier and scopes an API key has), RateLimits,
// GetBundledLlmStatus and ListCredentials (the handles of the account's saved
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
	// ProfileTrashCount — profiles in the recycle bin. They count toward
	// ProfileCap (owner decision 2, 2026-09-26), so ProfileCount +
	// ProfileTrashCount is what the cap refuses on.
	ProfileTrashCount int `json:"profile_trash_count"`
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
	// "global" | "sessions:create" | "agent_sessions:message" | "agent_sessions:input_event"
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
