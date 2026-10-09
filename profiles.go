package driftstack

import (
	"context"
	"net/url"
	"strconv"
)

// ProfilesResource handles /v1/profiles endpoints.
type ProfilesResource struct {
	client *Client
}

// Create makes a new profile. Tier-limit enforced server-side; returns a
// *QuotaExceededError when the cap is hit — the `tier-limit` problem type maps
// to QuotaExceededError in this SDK. (It said "throws a TierLimitError", which
// named a type this SDK does not define: TierLimitError exists only in the
// TypeScript SDK, and Go returns errors rather than throwing.)
func (r *ProfilesResource) Create(ctx context.Context, body *CreateProfileRequest) (*Profile, error) {
	var out Profile
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/profiles",
		body:   body,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns a page of profiles, newest first. Pass nil for defaults.
//
// Live profiles by default. Status: ProfileListTrashed lists the trash instead
// (the same rows as ListTrash): profiles there count toward no limit, and
// each carries PurgesAt, the earliest time the profile is permanently deleted
// (at least 7 days after the delete; removal follows at the next daily
// clean-up). The trash comes back whole, in one page.
func (r *ProfilesResource) List(ctx context.Context, query *ListProfilesQuery) (*ProfilesListPage, error) {
	var out ProfilesListPage
	q := url.Values{}
	if query != nil {
		if query.Limit > 0 {
			q.Set("limit", strconv.Itoa(query.Limit))
		}
		if query.Cursor != "" {
			q.Set("cursor", query.Cursor)
		}
		if query.Status != "" {
			q.Set("status", string(query.Status))
		}
	}
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/profiles",
		query:  q,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// Iterate yields every profile across cursor pages. The callback
// returns false to stop early; an error from the callback is
// propagated back to the caller.
func (r *ProfilesResource) Iterate(ctx context.Context, query *ListProfilesQuery, fn func(*Profile) (bool, error)) error {
	cursor := ""
	limit := 0
	var status ProfileListStatus
	if query != nil {
		limit = query.Limit
		cursor = query.Cursor
		status = query.Status
	}
	for {
		q := &ListProfilesQuery{Limit: limit, Cursor: cursor, Status: status}
		page, err := r.List(ctx, q)
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

// Get fetches a single profile by id.
func (r *ProfilesResource) Get(ctx context.Context, profileID string) (*Profile, error) {
	var out Profile
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/profiles/" + url.PathEscape(profileID),
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update applies a partial change. Fields left as zero / nil are
// untouched server-side. A Clear flag (ClearNotes, ClearDefaultProxyID, ...)
// removes a stored value; a field set beside its own Clear flag is an error,
// IsRetryable false, and nothing is sent. With ExpectedRevision set, a
// profile another computer changed first is refused with a 409
// *ConflictError (Problem["code"] "stale_revision") and nothing changes.
func (r *ProfilesResource) Update(ctx context.Context, profileID string, body *UpdateProfileRequest) (*Profile, error) {
	// Checked here, not left to MarshalJSON: a body that fails to encode
	// reaches the caller as a *TransportError, which IsRetryable reports true
	// for, and this request can never succeed.
	if body != nil {
		if err := body.contradiction(); err != nil {
			return nil, err
		}
	}
	var out Profile
	if err := r.client.do(ctx, requestOptions{
		method: "PATCH",
		path:   "/v1/profiles/" + url.PathEscape(profileID),
		body:   body,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteProfileOptions are the optional settings of ProfilesResource.Delete.
type DeleteProfileOptions struct {
	// Permanent deletes the profile permanently in the same call, instead of
	// keeping it in the trash for 7 days (sent as ?permanent=true). This
	// cannot be undone.
	Permanent bool
}

// Delete removes a profile. Idempotent — calling on a missing id is
// not an error (returns nil).
//
// By default this moves the profile to the trash and frees its slot at once:
// a profile in the trash counts toward no limit. It is kept there for at
// least 7 days (until its PurgesAt), then removed for good at the next daily
// clean-up; until it is removed Restore brings it back.
//
// With &DeleteProfileOptions{Permanent: true} it is deleted permanently in
// the same call (a live profile is moved to the trash and purged; a trashed
// one is purged). This cannot be undone. A profile with a live session
// is refused with a 409 and nothing is deleted. For a profile already in the
// trash, Purge does the same.
func (r *ProfilesResource) Delete(ctx context.Context, profileID string, opts ...*DeleteProfileOptions) error {
	req := requestOptions{
		method: "DELETE",
		path:   "/v1/profiles/" + url.PathEscape(profileID),
	}
	if len(opts) > 0 && opts[0] != nil && opts[0].Permanent {
		req.query = url.Values{"permanent": {"true"}}
	}
	return r.client.do(ctx, req)
}

// ListTrash returns the account's trashed (soft-deleted) profiles, most-
// recently trashed first. Each carries DeletedAt and PurgesAt, the earliest
// time the profile is permanently deleted (at least 7 days after the delete;
// removal follows at the next daily clean-up). L4b recycle bin.
func (r *ProfilesResource) ListTrash(ctx context.Context) (*ProfilesTrashList, error) {
	var out ProfilesTrashList
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/profiles/trash",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// Restore un-trashes a profile (clears DeletedAt). Returns a 404 error if
// there's no trashed profile with that id, or 409 if a live profile already
// holds the name (rename it first). L4b recycle bin.
//
// A restore needs a free slot, and each account may restore a limited number
// of profiles in any rolling 30 days: 10% of the plan's profiles, and never
// fewer than 3; Enterprise is a flat 100.
// Both refusals are a *QuotaExceededError (429); read its Code:
// QuotaCodeRestoreNeedsSlot (the account's live profiles are at the plan's
// number) or QuotaCodeRestoreCap (the plan's restores are used up;
// NextRestoreAt says when the next one is available). A refused restore
// changes nothing and uses none of the plan's restores.
func (r *ProfilesResource) Restore(ctx context.Context, profileID string) (*Profile, error) {
	var out Profile
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/profiles/" + url.PathEscape(profileID) + "/restore",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// Purge permanently deletes a trashed profile now, instead of at the end of
// its 7 days in the trash. It frees no slot: a profile in the trash already
// counts toward no limit. Returns a 404 error if there's no trashed profile
// with that id. Irreversible. L4b recycle bin.
func (r *ProfilesResource) Purge(ctx context.Context, profileID string) error {
	return r.client.do(ctx, requestOptions{
		method: "DELETE",
		path:   "/v1/profiles/" + url.PathEscape(profileID) + "/purge",
	})
}

// LaunchProfileRequest — 2026-05-20 antidetect-browser-style one-shot
// launch. Label is an optional override; everything else flows from
// the profile (archetype + metadata + last_used_at bumped
// server-side).
//
// ProxyID (2026-10-02) is the id of one of your saved proxies, the same one
// CreateSessionRequest takes; empty, the launch uses the proxy the profile is
// bound to. A deployment that requires a proxy of your own refuses a launch
// with neither with a 422 *ProxyRequiredError. This struct once carried a raw
// Proxy field that silently did nothing; it stays removed, so setting one is a
// compile error instead of a no-op. On a deployment that cannot carry a proxy
// here, use AgentSessionsResource.Create, setting ProfileID and ProxyID -- that
// resource starts an iPhone Safari session and routes its traffic through the
// saved proxy you name.
type LaunchProfileRequest struct {
	Label   string `json:"label,omitempty"`
	ProxyID string `json:"proxy_id,omitempty"`
}

// Launch creates a session bound to this profile. Equivalent to
// POST /v1/sessions {profile_id, archetype: <profile.archetype>}
// but one round-trip + the server inherits the profile's archetype.
func (r *ProfilesResource) Launch(
	ctx context.Context,
	profileID string,
	body *LaunchProfileRequest,
) (*Session, error) {
	var out Session
	req := requestOptions{
		method: "POST",
		path:   "/v1/profiles/" + url.PathEscape(profileID) + "/launch",
		out:    &out,
	}
	if body != nil {
		req.body = body
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// CloneProfileRequest — the Clone body. Pass an empty struct to let the server
// auto-derive a "(copy)" / "(copy 2)" / ... name.
type CloneProfileRequest struct {
	Name string `json:"name,omitempty"`
}

// Clone duplicates a profile. Tier-cap + name-conflict are checked
// the same way as Create.
func (r *ProfilesResource) Clone(
	ctx context.Context,
	profileID string,
	body *CloneProfileRequest,
) (*Profile, error) {
	if body == nil {
		body = &CloneProfileRequest{}
	}
	var out Profile
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/profiles/" + url.PathEscape(profileID) + "/clone",
		body:   body,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProfileExportPayload is the metadata-only body inside an export envelope.
type ProfileExportPayload struct {
	Name        string  `json:"name"`
	Archetype   string  `json:"archetype"`
	Description *string `json:"description"`
	// Notes — the free-text notes kept with the profile. Always present on an
	// export; an envelope written before notes existed has none and imports
	// as null.
	Notes *string `json:"notes,omitempty"`
}

// ProfileExportEnvelope — a versioned, metadata-only export. Per-profile
// browser state lives driver-side and is out of scope for the v1 envelope; the
// Version field lets a future v2 stay back-compat. The Source* fields are
// informational — Import always mints a fresh id, into any account.
type ProfileExportEnvelope struct {
	Version         int                  `json:"version"`
	ExportedAt      string               `json:"exported_at"`
	SourceProfileID string               `json:"source_profile_id"`
	SourceAccountID string               `json:"source_account_id"`
	Profile         ProfileExportPayload `json:"profile"`
}

// Export returns this profile as a versioned, metadata-only JSON envelope.
// Feed the result to Import (in any account) to mint a fresh profile from it.
func (r *ProfilesResource) Export(ctx context.Context, profileID string) (*ProfileExportEnvelope, error) {
	var out ProfileExportEnvelope
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/profiles/" + url.PathEscape(profileID) + "/export",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// ImportProfileRequest — a v1 export envelope plus an optional rename.
type ImportProfileRequest struct {
	Envelope ProfileExportEnvelope `json:"envelope"`
	// NameOverride renames on import without editing the file; omit to use
	// the envelope's profile name.
	NameOverride string `json:"name_override,omitempty"`
}

// Import mints a fresh profile in the EFFECTIVE account — the caller's own,
// or the owner they are acting as via X-Driftstack-Account — from a v1 export
// envelope. Tier-cap + name-conflict semantics match Create; importing an
// envelope from a different account is permitted (file-based transfer).
func (r *ProfilesResource) Import(ctx context.Context, body *ImportProfileRequest) (*Profile, error) {
	var out Profile
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/profiles/import",
		body:   body,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// TrimProfileResponse — the discriminated body for Trim. The server
// ALWAYS returns HTTP 200; branch on Status, never the HTTP code:
//   - "ok"          → caches cleared; BytesReclaimed freed, SizeBytes is the new
//     (smaller) sealed-store size persisted server-side.
//   - "unavailable" → nothing to trim (fresh profile, or no machine free to run
//     the trim). Reason is human-readable. Not an error.
//   - "timeout"     → the machine running the trim did not respond in time. Safe
//     to retry.
//   - "error"       → that machine reported a failure; the stored blob is
//     untouched.
//
// SizeBytes / BytesReclaimed are present only on "ok"; Reason only on
// "unavailable" / "error" — hence omitempty on all three.
type TrimProfileResponse struct {
	Status         string `json:"status"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
	BytesReclaimed int64  `json:"bytes_reclaimed,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// Trim — "Clear cache, keep logins". Reclaims a profile's
// re-fetchable caches (HTTP/media/DOMCache/service-workers) WITHOUT touching
// logins, localStorage, IndexedDB or open tabs — the headline reclaim action
// when an account is over its storage cap. The server always responds 200 with
// a DISCRIMINATED body; branch on Status (see TrimProfileResponse), not the HTTP
// code. On "ok" the profile's SizeBytes is updated server-side.
func (r *ProfilesResource) Trim(ctx context.Context, profileID string) (*TrimProfileResponse, error) {
	var out TrimProfileResponse
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/profiles/" + url.PathEscape(profileID) + "/trim",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProfileActivityEntry is one planned navigation from an agent session.
type ProfileActivityEntry struct {
	At             string `json:"at"`
	URL            string `json:"url"`
	AgentSessionID string `json:"agent_session_id"`
}

// ProfileActivityResponse is a profile's recent navigation, most recent first.
// Truncated is true when older activity exists beyond the server's bounds.
// PagesWithheld is true when the pages were withheld from the caller: they come
// from AI session records, which need the read:sessions scope and, in a
// teammate's workspace, the admin role. Data is then empty.
type ProfileActivityResponse struct {
	Data            []ProfileActivityEntry `json:"data"`
	SessionsScanned int                    `json:"sessions_scanned"`
	Truncated       bool                   `json:"truncated"`
	PagesWithheld   bool                   `json:"pages_withheld"`
}

// Activity returns the profile's recent navigation, projected from the
// account's agent session transcripts. This is ACCOUNT ACTIVITY, not browsing
// history: Trim with scope "history" clears the profile's open tabs in the
// session and does not remove these rows.
func (r *ProfilesResource) Activity(ctx context.Context, profileID string) (*ProfileActivityResponse, error) {
	var out ProfileActivityResponse
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/profiles/" + url.PathEscape(profileID) + "/activity",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}
