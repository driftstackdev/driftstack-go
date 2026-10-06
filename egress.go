package driftstack

import (
	"context"
	"net/url"
)

// EgressResource handles the customer egress surface:
//  1. Per-session proxy attach (/v1/sessions/{id}/proxy).
//  2. Saved proxy library — CRUD + a reachability test over the
//     account's reusable proxy configs (/v1/account/me/proxies). This
//     is the LIVE account-proxies API (shipped) — the same backend the
//     desktop app + dashboard use.
//
// Mirrors the TypeScript + Python SDK egress resources.
//
// SECURITY: the secret-bearing fields (SOCKS5 password, OpenVPN
// config_blob, WireGuard private_key and preshared_key) are write-only — wrapped
// server-side under the account key, never echoed back. List/get return
// metadata only (+ HasPassword / HasSecret).
type EgressResource struct {
	client *Client
}

// SessionEgressConfig is the body shape for AttachToSession.
// Use map[string]any for nested shapes — keeping the type loose
// matches the existing SDK pattern for non-billing surfaces.
type SessionEgressConfig struct {
	SessionID       string          `json:"session_id"`
	Proxy           map[string]any  `json:"proxy"`
	EgressSafeguard map[string]bool `json:"egress_safeguard"`
}

// SessionProxyAttachResponse is the public-safe envelope returned
// by AttachToSession + GetSessionProxy. Carries only the proxy type
// + safeguard flags — never raw secret material.
type SessionProxyAttachResponse struct {
	Type       string          `json:"type"`
	Safeguards map[string]bool `json:"safeguards"`
}

// AccountProxyInput is the flat create body for CreateProxy. Password +
// the VPN blocks carry write-only secret material (wrapped server-side).
type AccountProxyInput struct {
	Label     string         `json:"label"`
	Scheme    string         `json:"scheme,omitempty"` // empty omitted → server default (socks5); "" would fail the enum
	Host      string         `json:"host"`
	Port      int            `json:"port"`
	Username  *string        `json:"username,omitempty"`
	Password  *string        `json:"password,omitempty"`
	OpenVPN   map[string]any `json:"openvpn,omitempty"`
	WireGuard map[string]any `json:"wireguard,omitempty"`
	// ExpectedRevision — PUT only: the Revision last read; a proxy that moved
	// on is refused (409, code "stale_revision") and nothing changes.
	ExpectedRevision *int `json:"expected_revision,omitempty"`
}

// AccountProxyMetadata is the public-safe envelope returned by
// CreateProxy / UpdateProxy + ListProxies items. The secret itself is
// never returned — HasPassword / HasSecret signal whether one is stored.
type AccountProxyMetadata struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Scheme      string  `json:"scheme"`
	Host        string  `json:"host"`
	Port        int     `json:"port"`
	Username    *string `json:"username"`
	HasPassword bool    `json:"has_password"`
	HasSecret   bool    `json:"has_secret"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	// HasPresharedKey / Config / Revision (2026-09-28): whether a WireGuard
	// pre-shared key is stored; the NON-SECRET VPN fields (nil on socks5/http);
	// and the revision every PUT moves — send it back as "expected_revision"
	// in UpdateProxy's body to be told (409, code "stale_revision") when
	// another computer changed the proxy first.
	HasPresharedKey bool                   `json:"has_preshared_key"`
	Config          *AccountProxyVpnConfig `json:"config"`
	Revision        int                    `json:"revision"`
	// MemberView / Country / Status — set on the narrow rows a team MEMBER
	// gets from ListProxies in a team workspace (owner decision 3): name, type,
	// country and status only; Host, Port, Username and HasPassword are then
	// absent. Owners and admins get full rows with MemberView false.
	MemberView bool    `json:"member_view,omitempty"`
	Country    *string `json:"country,omitempty"`
	Status     string  `json:"status,omitempty"`
	// QuicMeasured is 'h3' or 'h2-only' once a live session through this proxy
	// reported its transport, or null when never measured. QuicMeasuredAt is
	// when that measurement was taken (RFC 3339), or null.
	QuicMeasured   *string `json:"quic_measured"`
	QuicMeasuredAt *string `json:"quic_measured_at"`
	// QuicProbe and UDPProbe are what a proxy test last measured through this
	// proxy: whether QUIC relays, and whether the proxy carries UDP. Separate
	// from QuicMeasured, which is what a live session negotiated. Three states,
	// and a caller must keep them apart: true = measured working, false =
	// measured NOT working, nil = never measured. QuicProbeAt / UDPProbeAt are
	// when each was measured (RFC 3339), or nil.
	QuicProbe   *bool   `json:"quic_probe"`
	QuicProbeAt *string `json:"quic_probe_at"`
	UDPProbe    *bool   `json:"udp_probe"`
	UDPProbeAt  *string `json:"udp_probe_at"`
	// ExitObserved is the last exit identity observed THROUGH this proxy — by a
	// live session (observed_via "session") or by a proxy test
	// ("probe"), latest wins — or nil when never observed. For an OpenVPN /
	// WireGuard proxy this is the only source of its location and timezone
	// short of running a test. Nil is "not observed", never a placeholder.
	ExitObserved *AccountProxyExitObserved `json:"exit_observed"`
	// ExitSupersededAt is when a proxy test found the tunnel DOWN while
	// ExitObserved was set (RFC 3339), or nil when never contradicted. The
	// stored exit is the last thing SEEN; this is when it was CONTRADICTED, so
	// a caller adopting ExitObserved should refuse an observation dated at or
	// before it. Cleared by the next exit observation (session or probe).
	// Driftstack's background reachability check, run from its own servers,
	// can set it too, so it does not say whether the proxy works where sessions
	// run: read FullCheckOk for that.
	ExitSupersededAt *string `json:"exit_superseded_at"`
	// FullCheckOk is the result of the last ?check=full test measured where
	// sessions run (measured_by "phone"): true = the proxy was usable, false = the check
	// finished and the proxy was not usable, nil = no such check since the proxy's
	// address, scheme or credentials last changed. FullCheckAt is when it was
	// measured (RFC 3339), or nil. Nothing else sets them: not a quick check,
	// not a full check that fell back to Driftstack's servers, not a check
	// that could not run, and not the background check.
	FullCheckOk *bool   `json:"full_check_ok"`
	FullCheckAt *string `json:"full_check_at"`
	// OsFingerprint is the LAST OS fingerprint Driftstack recorded for
	// this proxy's own TCP stack (POST :id/test takes it; this list is how it
	// reaches a machine that never ran that test), or nil when never measured.
	// Nil is "not measured", never "no OS" and never a placeholder.
	OsFingerprint *AccountProxyOsFingerprint `json:"os_fingerprint"`
	// OsFingerprintAt is when that reading was taken (RFC 3339), or nil. AGE the
	// reading by this: it is a stored measurement, not one your request made, and
	// a reading you cannot date should be treated as stale rather than current.
	OsFingerprintAt *string `json:"os_fingerprint_at"`
}

// AccountProxyExitObserved is the stored exit identity on AccountProxyMetadata.
// Country and Timezone are nil when they could not be resolved;
// ObservedAt is when the observation was recorded (RFC 3339), or nil.
type AccountProxyExitObserved struct {
	IP          string  `json:"ip"`
	Country     *string `json:"country"`
	Timezone    *string `json:"timezone"`
	ObservedVia string  `json:"observed_via"`
	ObservedAt  *string `json:"observed_at"`
}

// AccountProxyList is the GET /v1/account/me/proxies envelope.
type AccountProxyList struct {
	Data []AccountProxyMetadata `json:"data"`
	// Cap / Count (2026-09-28): the workspace's saved-proxy allowance under
	// the owner's plan (nil = unmetered) and how many it holds.
	Cap   *int `json:"cap"`
	Count int  `json:"count"`
}

// AccountProxyTestResult is the POST :id/test result. Ok=true carries
// LatencyMs; Ok=false carries Reason. 200 either way.
//
// NotRun is set when NOTHING RAN, so an Ok=false result is not a judgement
// about the proxy: "live_session" (a test of a VPN proxy was refused because
// a live session holds the tunnel), "config_unresolvable" (the stored
// configuration could not be turned into anything runnable — the same word
// the launch-refusal reason table uses for this fact), or "check_unavailable"
// (the full check could not be completed on our side right now — none of the
// machines that run full checks was free, the dispatch timed out, or this
// deployment does not run them; the Reason says which). Branch on it — never
// on the Reason prose — before treating Ok=false as a failed proxy.
//
// Renamed 2026-09-21 from "node_busy" / "node_error" / "no_node" (merged into
// "check_unavailable") and "unresolvable" (renamed "config_unresolvable").
type AccountProxyTestResult struct {
	Ok        bool   `json:"ok"`
	LatencyMs int    `json:"latency_ms,omitempty"`
	Reason    string `json:"reason,omitempty"`
	NotRun    string `json:"not_run,omitempty"`
	// MeasuredFrom is the older name for MeasuredBy below.
	//
	// Deprecated: superseded by MeasuredBy (2026-09-21). Still sent with its
	// original values, unchanged, for an integration that already reads it;
	// MeasuredBy is documented from here on.
	MeasuredFrom *string `json:"measured_from,omitempty"`
	// MeasuredBy is the customer-worded name for MeasuredFrom above: "phone"
	// (one of the machines that run sessions took the measurement — what
	// `?check=full` asks for; the value keeps its original name) or
	// "driftstack" (Driftstack itself measured it — the same path
	// `?check=quick` always takes, and the honest fallback when a
	// `?check=full` request could not reach that machine in time). Present only
	// on a full-check result (including a quick check of a VPN proxy — see
	// CheckRan); a quick check of a SOCKS5/HTTP proxy is always "driftstack"
	// and carries no field to say so.
	MeasuredBy *string `json:"measured_by,omitempty"`
	// CheckRan is set when Driftstack ran a different check from the one asked
	// for, and names it. Today: "full" on a quick check of an OpenVPN or
	// WireGuard proxy, which can only be checked by bringing its tunnel up
	// (2026-10-06). Nil otherwise.
	CheckRan *string `json:"check_ran,omitempty"`
	// OsFingerprint is the proxy's own TCP-stack fingerprint, present only when
	// Driftstack actually observed it. Absent is "not observed", never
	// a placeholder OS.
	OsFingerprint *AccountProxyOsFingerprint `json:"os_fingerprint,omitempty"`
	// OsFingerprintAt is set when OsFingerprint is a STORED reading the server
	// attached because THIS test observed none — it is when that reading was
	// taken (RFC 3339). Nil means the fingerprint above was measured by this
	// test and is dated by the reply. A stored reading keeps
	// OsFingerprintUnavailable beside it: that names why this test found none.
	OsFingerprintAt *string `json:"os_fingerprint_at,omitempty"`
	// OsFingerprintUnavailable names WHY OsFingerprint is absent when the server
	// knows: "not_available_for_vpn" (an openvpn/wireguard proxy has no single
	// address of its own to read a stack from), "not_captured" (the reading
	// was attempted and produced nothing this time) or "not_offered_here"
	// (this deployment does not take this reading at all). Nil when a
	// fingerprint is present or the server predates the field.
	//
	// Renamed 2026-09-21 from "vpn_tunnel" / "not_observed" / "observer_off".
	OsFingerprintUnavailable *string `json:"os_fingerprint_unavailable,omitempty"`
	// ExitObserved is the exit seen behind a VPN row, when one was
	// observed; nil otherwise.
	ExitObserved *AccountProxyExitObserved `json:"exit_observed,omitempty"`
}

// AccountProxyOsFingerprint is Driftstack's passive TCP-stack
// fingerprint of the proxy host (or its exit IP), as the server reports it.
type AccountProxyOsFingerprint struct {
	OS          string `json:"os"`
	Confidence  string `json:"confidence"`
	Reason      string `json:"reason"`
	ObservedIP  string `json:"observed_ip"`
	ObservedVia string `json:"observed_via"`
	// SingleHostVantage is true only when the dialled host, the SYN source and
	// the exit are one machine, so the reading describes the path a website
	// gets. When false, do not draw a match/mismatch conclusion from OS.
	// Same fact as DirectReading below, under its original name.
	SingleHostVantage bool `json:"single_host_vantage"`
	// WebPortVantage is true when the reading was taken on port 443 at an IP
	// literal, the port a website connects on. With ObservedVia "proxy_host"
	// it still names a stack rather than giving a conclusion.
	// Same fact as WebsiteLikeReading below, under its original name.
	WebPortVantage bool `json:"web_port_vantage"`
	// DirectReading is the customer-worded name for SingleHostVantage above —
	// added 2026-09-21, same value, alongside the original field.
	DirectReading bool `json:"direct_reading,omitempty"`
	// WebsiteLikeReading is the customer-worded name for WebPortVantage above —
	// added 2026-09-21, same value, alongside the original field.
	WebsiteLikeReading bool `json:"website_like_reading,omitempty"`
}

// AttachToSession sets the proxy config for a session.
//
// Deprecated: RETIRED. Every deployment answers 410 (*FeatureUnavailableError,
// code "endpoint_retired") and reads nothing you send: a proxy cannot be set
// on an existing session. Set ProxyID when you create the session, with
// Sessions.Create or AgentSessions.Create.
func (r *EgressResource) AttachToSession(ctx context.Context, sessionID string, config *SessionEgressConfig) (*SessionProxyAttachResponse, error) {
	var out SessionProxyAttachResponse
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/sessions/" + url.PathEscape(sessionID) + "/proxy",
		body:   config,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSessionProxy reads the session's current proxy summary.
//
// Deprecated: RETIRED. Every deployment answers 410, like AttachToSession. A
// session's proxy is the ProxyID it was created with.
func (r *EgressResource) GetSessionProxy(ctx context.Context, sessionID string) (*SessionProxyAttachResponse, error) {
	var out SessionProxyAttachResponse
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/sessions/" + url.PathEscape(sessionID) + "/proxy",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// AccountProxyVpnConfig — the non-secret half of a VPN proxy: the WireGuard
// peer public key, endpoint, allowed IPs, address, DNS, MTU and keepalive, or
// the OpenVPN username. Never a key, a pre-shared key or a config blob.
type AccountProxyVpnConfig struct {
	PeerPublicKey string `json:"peer_public_key,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	AllowedIPs    string `json:"allowed_ips,omitempty"`
	Address       string `json:"address,omitempty"`
	DNS           string `json:"dns,omitempty"`
	Username      string `json:"username,omitempty"`
	// MTU is the WireGuard MTU (1280-1500) when the saved configuration sets
	// one; nil otherwise. Sent to the session's machine, which does not apply
	// it yet (2026-10-06).
	MTU *int `json:"mtu,omitempty"`
	// PersistentKeepalive is the WireGuard keepalive in seconds (0-65535) when
	// the saved configuration sets one; nil otherwise.
	PersistentKeepalive *int `json:"persistent_keepalive,omitempty"`
}

// CreateProxyOptions carries optional per-call overrides for CreateProxy.
// IdempotencyKey is forwarded as the Idempotency-Key request header: a retry
// with the same key returns the proxy already created under it (same id, one
// row) instead of a second one.
type CreateProxyOptions struct {
	IdempotencyKey string
}

// ListProxies returns the workspace's saved proxies (metadata only). Without
// an X-Driftstack-Account header these are the calling account's; in a team
// workspace an owner or admin gets full rows and a member the narrow view
// (MemberView true). Cap and Count describe the workspace under the owner's
// plan.
func (r *EgressResource) ListProxies(ctx context.Context) (*AccountProxyList, error) {
	var out AccountProxyList
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/account/me/proxies",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateProxy stores a reusable proxy config and returns its metadata.
// password / the VPN config_blob / private_key are write-only — wrapped
// server-side, never echoed back.
//
// An optional *CreateProxyOptions carries an IdempotencyKey, forwarded as the
// Idempotency-Key header: a retry with the same key returns the proxy already
// created under it (same id, one row) instead of a second one. In a team
// workspace (X-Driftstack-Account) the proxy is created on the owner's account
// under the owner's plan; the admin role is needed.
func (r *EgressResource) CreateProxy(ctx context.Context, body *AccountProxyInput, opts ...*CreateProxyOptions) (*AccountProxyMetadata, error) {
	var out AccountProxyMetadata
	req := requestOptions{
		method: "POST",
		path:   "/v1/account/me/proxies",
		body:   body,
		out:    &out,
	}
	if len(opts) > 0 && opts[0] != nil && opts[0].IdempotencyKey != "" {
		req.headers = map[string]string{"Idempotency-Key": opts[0].IdempotencyKey}
	}
	if err := r.client.do(ctx, req); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateProxy patches a saved proxy. The body is a loose map so callers
// preserve the omit-vs-null secret semantics (omit a field to keep it, a
// null password to clear it, a string to (re)wrap it).
//
// Changing "host", "port" or "scheme" on a proxy that stores a password
// needs "password" in the same body (the password, or nil to clear it);
// without it the server answers 400 and nothing changes, so a saved
// password is never sent to a new address. On an OpenVPN or WireGuard
// proxy, send the full VPN configuration to change its address.
func (r *EgressResource) UpdateProxy(ctx context.Context, proxyID string, body map[string]any) (*AccountProxyMetadata, error) {
	var out AccountProxyMetadata
	if err := r.client.do(ctx, requestOptions{
		method: "PUT",
		path:   "/v1/account/me/proxies/" + url.PathEscape(proxyID),
		body:   body,
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteProxy removes a saved proxy by id.
func (r *EgressResource) DeleteProxy(ctx context.Context, proxyID string) error {
	return r.client.do(ctx, requestOptions{
		method: "DELETE",
		path:   "/v1/account/me/proxies/" + url.PathEscape(proxyID),
	})
}

// TestProxy runs a server-side reachability probe (SSRF-guarded) of a
// saved proxy's host:port. 200 either way: Ok=true+LatencyMs when
// reachable, Ok=false+Reason when not.
func (r *EgressResource) TestProxy(ctx context.Context, proxyID string) (*AccountProxyTestResult, error) {
	var out AccountProxyTestResult
	if err := r.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/v1/account/me/proxies/" + url.PathEscape(proxyID) + "/test",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}
