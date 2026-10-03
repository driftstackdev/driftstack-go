# Changelog

All notable changes to the Driftstack Go SDK. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning
follows [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

One change below alters, with no compile error and no runtime error, how a
struct of yours that embeds `UpdateProfileRequest` is encoded to JSON. If your
program has one, read **Changed** below first.

### Added (2026-10-03)

- **On a session whose device supports it, a tap or a wait can name a frame.**
  Additive, and nothing changes on any other session or in the types. On a
  session whose device can act inside an embedded frame (an iframe), `Frame`
  can be set on an `"interact"` step with `Action` `"tap"`, and on a `"wait"`
  with `Condition` `"selector_visible"` — the same path of positions as a
  read's. The tap's selector is matched, and the element is waited for, inside
  that frame. A `"scroll"` or `"press"` never carries `Frame`: both act on the
  page itself. Such a step succeeded only when the session confirmed it was
  done inside exactly that frame; one it did not confirm fails and is not tried
  again (`Diagnosis.Category` `"unknown"` for a tap, `"condition_not_met"` for
  a wait). A tap inside a frame that pays, buys or deletes an account asks for
  approval as one on the page does.

- **Session secrets: `AgentSessions.RegisterSecret(ctx, id, body)`,
  `ListSecrets(ctx, id)` and `DeleteSecret(ctx, id, handle)`.** Register a
  password, a one-time code or a payment card number for one agent session and
  get back a handle (`sec_…`); write `{{credential:<handle>}}` in a `Message` as
  the whole value of the step that types it. The plan, the transcript and every
  response carry the handle, and the value is typed only on an `https://` page
  on one of `Sites`. Once typed it is on the page; where the page shows it
  again it is replaced by its placeholder in what the model is shown, the step
  results and the transcript, as for a saved credential (a card number however
  its digits are grouped, but not part of it). `Label` appears in refusals the
  model reads, so put nothing secret in it. It is held in server memory only and
  dropped when the session ends, when deleted, when the service restarts, or
  when `TTLSeconds` runs out (default 3600); one a step has typed, or tried to,
  is kept past `TTLSeconds`, never typed again, only to keep hiding it from the
  model. New types `AgentSessionSecret` and
  `RegisterAgentSessionSecretRequest`. `RegisterSecret` is not retried on a
  network error, since a retry could register the value twice.

- **`CaptureRequest.FrameMatch` (`*FrameMatch`).** With `CaptureDOMSnapshot`,
  reads one embedded frame (an iframe) named by its address instead of the
  page: `Host` (required; exact, letter case aside, port not compared),
  `PathPrefix` (whole path segments) and `Query` (each parameter with exactly
  that value; at most 10). Exactly one frame must match. With none or several,
  nothing is read, the session stays ready, and `Capture` returns a 409
  `*ConflictError` whose `Problem["code"]` is `"frame_not_found"`,
  `"frame_match_ambiguous"` (`Problem["matched_frames"]` is the count) or
  `"frame_match_unconfirmed"`. A `Query` parameter whose value the browser
  removes before it reaches Driftstack (`client_secret`, `token`,
  `code_verifier`, ...; the full list is in the Sessions reference), or
  `FrameMatch` on a screenshot or a PDF, returns a 400 `*BadRequestError`. A
  session started with a `ProxyID` cannot run it yet, like every step-by-step
  operation.

- **A profile's proxy and launch settings can be set from Go:
  `DefaultProxyID`, `Geolocation` and `StopOnExitIPChange` on
  `CreateProfileRequest` and `UpdateProfileRequest`.** `Profile` already
  returned them. `DefaultProxyID` is one of your saved proxies for the profile
  to launch through (`Profile.ProxyChoice` then reads `"bound"`); a proxy that
  is not one of yours is a 404 `*NotFoundError`, and an `http` proxy, which
  cannot carry a session, is a 400 `*BadRequestError`. `Geolocation` and
  `StopOnExitIPChange` apply to a session created with the profile unless its
  create sets its own. On `UpdateProfileRequest`, `StopOnExitIPChange` is a
  `*bool`, so `false` can be sent.

- **`UpdateProfileRequest.ExpectedRevision` (`*int`).** Send the
  `Profile.Revision` you last read, and an update to a profile that another
  computer changed first is refused with a 409 `*ConflictError` whose
  `Problem["code"]` is `"stale_revision"` (`Problem["current_revision"]` is
  the revision now stored), and nothing changes. Leave it nil to write
  unconditionally, as before.

- **Clearing a profile setting: `ClearDescription`, `ClearFolder`,
  `ClearTags`, `ClearIcon`, `ClearNote`, `ClearNotes`, `ClearDefaultProxyID`
  and `ClearGeolocation` on `UpdateProfileRequest`.** A nil field leaves a
  setting as it is, so until now an update could not remove one. Each flag
  sends its field as `null` and removes what is stored; `ClearTags` sends an
  empty list and removes every tag, and `ClearDefaultProxyID` leaves the
  profile `"unset"`. Setting a field and its own Clear flag together is an
  error, and nothing is sent; `IsRetryable` returns false for it, since
  sending it again cannot succeed. An update that uses none of the new fields
  is sent exactly as before. `UpdateProfileRequest` now has a `MarshalJSON`
  method, which changes how a struct that embeds it is encoded: see
  **Changed** below.

### Changed (2026-10-03)

- **A struct of yours that embeds `UpdateProfileRequest` is now encoded as
  `UpdateProfileRequest` alone, and your own fields are left out.**
  `UpdateProfileRequest` now has a `MarshalJSON` method (it writes the Clear
  flags), and `encoding/json` uses a method promoted from an embedded field to
  encode the whole struct. Nothing stops compiling and no error is returned:
  ``struct { UpdateProfileRequest; Extra string `json:"extra"` }`` with `Name`
  `"x"` encodes as `{"name":"x"}`, where v0.6.0 also wrote `"extra"`. This
  reaches you only if you encode such a struct yourself (to store, queue or
  log it, say); `Profiles.Update` takes an `*UpdateProfileRequest`, so the
  body it sends is unchanged. To keep your fields, give the field a name
  (`Update UpdateProfileRequest`), which nests the request under that name, or
  write a `MarshalJSON` for your struct that encodes the two parts and joins
  them.

## [0.6.0] - 2026-10-03

v0.6.0 adds to v0.5.0 and removes nothing. One addition can stop a v0.5.0
program compiling: `AgentIntent`, `AgentIntentResult` and `AgentStepEvent` can
no longer be compared with `==` or used as map keys. If your program does
either, read **Migration** below first. Upgrade with
`go get github.com/driftstackdev/driftstack-go@v0.6.0`.

Hosted Driftstack now runs every session through a proxy of yours, and
`CreateSessionRequest.ProxyID` and `LaunchProfileRequest.ProxyID`, new here,
are how `Sessions.Create` and `Profiles.Launch` name one.

### Added

- **`CreateSessionRequest.ProxyID` and `LaunchProfileRequest.ProxyID`.** The
  id of one of your saved proxies; the session's traffic goes out through it.
  Hosted Driftstack runs every session through a proxy of yours, so a create
  without one is refused with a 422 `*ProxyRequiredError` (from a profile
  bound to a proxy, that proxy is used when `ProxyID` is empty). A session
  started with a `ProxyID` cannot run the step-by-step operations (Navigate,
  Interact, Capture, ...) yet: they answer 503 and leave the session ready.
  It ends, and reads `destroyed`, when its agent session is closed or it stops
  on its own (30 minutes without activity, or its time limit).
- **`AgentSession.Ready` (`*bool`), `ReadyAt` and `AgentSession.IsReady()`.**
  `Status` reads `"active"` from the moment a session is created; `Ready` turns
  true once the session reports that its browser has finished
  starting, and `ReadyAt` says when. Wait for `IsReady()` before sending the
  first message. A session on a VPN can take longer to become ready. A session
  that closes while `Ready` is false failed to start; `ClosedReason` says why.
  `Ready` is nil from an older server, which does not hold messages either;
  `IsReady()` reads nil as ready.
- **`SessionNotReadyError`** and **`ErrSessionNotReady`** (HTTP 409, type
  `https://errors.driftstack.dev/session-not-ready`; `errors.Is` also matches
  `ErrConflict`). A message sent before the session is ready waits for it, for
  up to 45 seconds on the streamed response (20 seconds on a JSON one); if it is
  still starting then, nothing ran and the message is
  refused with this error. `IsRetryable` returns true; `RetryAfterSeconds` says
  how long to wait, and the same idempotency key is safe to reuse.
- **Two more agent-session models: `"claude-fable-5-1"` (Claude Fable 5.1) and
  `"claude-opus-5-5"` (Claude Opus 5.5).** Both run only on your own Anthropic
  key, like every Opus model: when a session would run on Driftstack's included
  AI, `Create` (and every message) returns a 403 `*ForbiddenError` whose
  `RequiresOwnKey()` is true. The default model is unchanged
  (`"claude-sonnet-5"`).
- **`AgentIntent.Frame` (`[]int`) on a read step.** A step with `Kind`
  `"capture"` and `Capture` `"dom_snapshot"`, and one with `Kind` `"extract"`
  and `Body` true, can now carry `Frame`: the embedded document (an iframe) the
  step read instead of the page, as a path of positions — the frame's position
  among the page's frames, then its position inside that frame for a nested
  one, outermost first (`[0]`, `[0, 1]`). Each position is 0 to 512, and a path
  is 1 to 8 levels deep. Frames are numbered by the session's own frame list, in
  the order the browser created them, which is not always the order of the
  iframe tags in the markup. `Frame` is also set on an `"interact"` with
  `Action` `"type"`: the step typed into an element inside that frame. It is
  never set on a screenshot, a PDF, an `"extract"` by selector or a step that
  taps, scrolls, presses or waits, so no step can name a frame to tap into. A
  `"tap_at"` step taps a point on the screen, and that point can be inside a
  frame. Empty means the step read the page itself. You see it in the steps a turn
  reports (`ParsedIntents()`, and each result's `Intent`), in a transcript
  entry's `Intents` and in a recipe's `IntentLog`. `Frame` is a slice, so
  `AgentIntent` and the two types that hold one can no longer be compared with
  `==`: see **Migration** below.
- **`AgentIntent.Body` (`bool`), `X` and `Y` (`*int`).** An `"extract"` step
  reads the text of one element (`Selector`) or of the whole page (`Body`
  true); a `"tap_at"` step taps the point `X`, `Y`, in viewport pixels from the
  top-left corner. The API already returned these steps; their fields are now
  decoded instead of dropped. Additive.
- **`AccountResource.ListCredentials`** — `GET /v1/account/me/credentials`:
  the `Handle`, `Label`, `Sites`, `IncludeSubdomains` and `CreatedAt` of each
  saved credential on the account, never the value. Write a handle into an
  agent task as `{{credential:<handle>}}`, as the text of a type step. Each
  credential is locked to its `Sites`: a step types it only on an `https://`
  page on one of them, and is refused elsewhere with
  `Diagnosis.Category` `"credential_site_not_allowed"`. Saving and deleting a
  credential are done in the dashboard. With `WithEffectiveAccount`, a team
  member of either role lists the account owner's saved credentials — the
  ones agent tasks in that workspace use — as the same fields, never a value;
  a workspace you are not a member of answers 403.
- **`"session_unresponsive"`** joins the `Diagnosis.Category` values and the
  `NoticeReason` values. The session stopped answering automated steps (the live view may still show the page): it had already failed to answer one, and a quick check sent just before the next step got no answer either, so that step was not sent. The step fails
  with `Diagnosis.Category` `"session_unresponsive"` and `Retryable` false, and
  the turn stops there with `NoticeReason` `"session_unresponsive"`; a turn that
  hit it before planning anything answers `Kind` `"refuse"` with the same
  `NoticeReason` `"session_unresponsive"`. **What to do:** end
  the session and launch a new one — sending "continue" will not help. If the
  session answers the check again, steps run as normal. Both fields are plain
  strings, so nothing changes in the types.

### Changed

- **A tap that changed nothing is a failed step, and a turn that stopped
  because nothing was changing is not `OK`.** A tap the browser made and then
  saw change nothing on the page now comes back as a `"failure"` with
  `Diagnosis.Category == "no_effect"`, not as a `"success"`; the agent looks at
  the page again and tries something else. A tap where whether it changed
  anything could not be checked is still a `"success"`, now with
  `Warning.Kind == "effect_unknown"` and a summary that says so. An
  `AgentMessageResponse` with `NoticeReason == "no_progress"` now always has
  `OK` false. A `"type"` step the browser could type only part of is a
  `"failure"`.

### Deprecated

- **`Egress.AttachToSession` and `Egress.GetSessionProxy`.** The server retired
  `/v1/sessions/:id/proxy`: both answer 410 (`*FeatureUnavailableError`, code
  `endpoint_retired`) on every deployment. Set `ProxyID` when you create the
  session.

### Migration

`AgentIntent` gained `Frame`, a slice, and a Go struct that holds a slice
cannot be compared with `==` or `!=` or used as a map key. Three types lost
that:

- **`AgentIntent`**, which holds `Frame` (`[]int`).
- **`AgentIntentResult`**, which holds an `AgentIntent` in `Intent`.
- **`AgentStepEvent`**, which holds an `AgentIntentResult` in `Result`.

A program that compares two of these with `==` or `!=`, uses one as a map key,
or passes one as a `comparable` type argument no longer compiles, and the
compiler names each such line ("cannot be compared", "invalid map key type",
"does not satisfy comparable"). Nothing changes at run time, and a program that
does none of these needs no change. To fix one, compare the fields you need
(`a.Kind == b.Kind && a.Selector == b.Selector`), compare two frames with
`slices.Equal(a.Frame, b.Frame)`, or compare whole values with
`reflect.DeepEqual(a, b)`; key a map by a field, or by a string you build from
the fields, instead of by the struct.

## [0.5.0] - 2026-09-30

**Breaking: the module now covers what a program needs to run Driftstack, and
nothing a person manages in the dashboard.** Sign-up and sign-in, two-factor,
API keys, team members and invites, billing, checkout and crypto orders,
account and notification settings, the audit log and legal acceptance are
done in the [Driftstack dashboard](https://app.driftstack.io). Their methods
are removed, and their endpoints are no longer in the API reference. Sessions,
agent sessions, profiles and snapshots, saved proxies, archetypes, recipes,
webhooks, usage and rate limits are unchanged.

v0.5.0 is also the first version published at the module's own path,
`github.com/driftstackdev/driftstack-go`, and agent sessions now require a
proxy of yours.

### Upgrading from v0.4.0

Three things a v0.4.0 program needs changed. Step 1 is how you get v0.5.0 at
all. Step 2 compiles as it is and is refused at run time, so make it before you
deploy. Step 3 matters only if your program calls something that was removed,
and the compiler points out each call.

1. **Change the module path to `github.com/driftstackdev/driftstack-go`.** The
   SDK is published from its own repository now, so install and import it from
   there. The old path, `github.com/driftstackdev/driftstack-api/packages/sdk-go`,
   keeps building for existing programs at the versions the module proxy
   already serves (v0.4.0 and earlier) and receives no new versions: there is
   no v0.5.0 at it. Rewrite the import path in your `*.go` files, then fetch
   the module and tidy:

   ```sh
   grep -rl --include='*.go' 'driftstack-api/packages/sdk-go' . |
     xargs sed -i.bak 's#github\.com/driftstackdev/driftstack-api/packages/sdk-go#github.com/driftstackdev/driftstack-go#g'
   find . -name '*.go.bak' -delete
   go get github.com/driftstackdev/driftstack-go@latest
   go mod tidy
   ```

   Your imports then read `driftstack "github.com/driftstackdev/driftstack-go"`.
   The path is all this step changes: the package name is still `driftstack`.

2. **Set `ProxyID` on every `AgentSessions.Create`.** Every agent session now
   runs through a proxy you chose, so a create with an empty `ProxyID` is
   refused with the new **`*ProxyRequiredError`** (HTTP 422, type
   `https://errors.driftstack.dev/proxy-required`, `Code` `"proxy_required"`,
   `errors.Is(err, driftstack.ErrProxyRequired)`) before anything is created —
   no session, no charge, nothing recorded under your idempotency key. The
   field keeps its type, so this is a behaviour change your compiler will not
   catch. Pass the id of one of your saved proxies:
   `&driftstack.CreateAgentSessionRequest{Mode: "ai", ProxyID: "b1d7…"}`.
   `client.Egress.ListProxies` lists the ids you have and
   `client.Egress.CreateProxy` saves a new one; both need a key with the
   `account_owner` scope. A proxy you add in the desktop app is saved to your
   account the first time the app tests it or launches through it.

   `Sessions.Create` and `Profiles.Launch` (`POST /v1/sessions`,
   `POST /v1/profiles/{id}/launch`) are unchanged: they do not take a proxy, and
   only agent sessions require one. A deployment configured to require a
   customer proxy for every session refuses them with the same
   `*ProxyRequiredError`. To run a session through one of your saved proxies,
   use `AgentSessions.Create` with `ProxyID` (and `ProfileID` to launch a
   profile).

3. **Remove calls to what moved to the dashboard.** Everything under
   **Removed** below is gone from the module, so a program that still calls it
   fails to compile. The paragraph above says where that work is done now.

### Removed

- **`PurposeCumulativeRigValidation` and `PurposeTestDomainProbe`.** They
  are for Driftstack's own validation runs and are not part of the customer API.
  `PurposeProductionCustomer`, the default, is the one session purpose.
- **Resources:** `Client.APIKeys`, `Auth`, `Mfa`, `Team`, `Billing`,
  `CryptoOrders`, `AuditLog`, `EmailPreferences` and `Legal`, with their
  request and response types.
- **`AccountResource`:** `UpdateMe`, `UploadAvatar`, `ClearAvatar`,
  `ListWebSessions`, `RevokeWebSession`, `RevokeAllOtherWebSessions`,
  `GetBundledLlmSettings`, `UpdateBundledLlmSettings` and the four
  `…ByokAnthropicKey` methods. The monthly AI cap and a stored Anthropic key
  are set in dashboard Settings; a key for one agent session is still passed
  to `AgentSessions.Create`.
- **`AgentSessionsResource`:** `SetMode`, `Takeover`, `Handback` and
  `SendInputEvent`. They control a session a person drives in the Driftstack
  desktop app, which is not part of this API. `LivekitToken` stays, with the
  same signature and return type as in v0.4.0: live video is part of the
  public API, and it mints a fresh `*LiveKitInfo` for a running session's
  video room when the `LiveKit` field on the created session is nil or its
  24-hour token has expired.
- **`ProfilesResource.Transfer`:** giving a profile to another account is not
  part of the public API. To hand one over, `Profiles.Export` it and have the
  other account `Profiles.Import` the file.
- **`examples/billing_flow` and `examples/crypto_checkout`.**
- **Types and constants the removed methods used:** `APIKeyScope` and its
  `Scope…` constants, `APIKey…`, `Team…`, `Subscription`,
  `SubscriptionStatus` and the checkout and portal types, the sign-in types
  (`Signup…`, `VerifyEmail…`, `Login…`, `MagicLink…`, `PasswordReset…`,
  `RefreshSession…`, `Logout…`, `WebSession`, `Mfa…`) and `CliAuthorize…`.
  `Whoami` returns a key's scopes as strings.
- **`CanonicalModifierNames`**: the modifier vocabulary of the desktop app's
  live-input channel, which is not part of this API.
- **`PublicArchetype.CanvasFamily`**, which the API reference leaves out.

### Added

- **`AccountResource.Whoami`** — `GET /v1/whoami`: the `AccountID`,
  `APIKeyID`, `Tier` and `Scopes` behind the key. It needs no scope, so a
  read-only key can call it.
- **`ErrDashboardOnly`** (403, `https://errors.driftstack.dev/dashboard-only`):
  returned when a request made with an API key asks for something that is
  done in the dashboard — checkout and the billing portal, creating, editing
  or cancelling a crypto order, creating an API key, accepting a team invite
  or the legal terms, and signing dashboard sessions out.
- **`DeviceUnavailableError`** (409, problem type `device-unavailable`) and
  the sentinel **`ErrDeviceUnavailable`**: a create on a profile whose device
  is not offered right now — on hold until its evidence exists, or still in
  development — is refused before any session exists, so nothing starts and no
  concurrent-session slot is used. `Archetype` is the profile's device id and
  `HeldReason` the reason for the hold in one plain sentence (`""` when there
  is none).
  `errors.Is` matches both `ErrDeviceUnavailable` and `ErrConflict`.
- **`AiCreditsExhaustedError` / `ErrAiCreditsExhausted`** (HTTP 402, type
  `https://errors.driftstack.dev/ai-credits-exhausted`): an AI turn could not
  run on your account's AI credits, and nothing ran. `Reason` is `"balance"`
  (the credits your plan or trial includes are used up; `AvailableCredits`,
  `RequiredCredits` and, when known, `ResetsAt`), `"debt"` (AI is paused on
  the account; `DebtReason`) or `"task_too_large"` (shorten the request or
  start a new chat). A key of your own still runs the turn. Not retryable
  as-is.
- **`TrialEndedError` / `ErrTrialEnded`** (HTTP 402, type
  `https://errors.driftstack.dev/trial-ended`): your free trial has ended, so a
  new session is refused until you choose a plan. Every other call keeps
  working. Not retryable as-is.
- **`ProxyRequiredError` / `ErrProxyRequired`** — see step 2 of
  **Upgrading from v0.4.0** above. `Code` is `"proxy_required"`; the error's
  message is a sentence you can show your user. Not retryable as-is.
- **`AgentSession.ProxyID`** — `*string`, the id of the account proxy
  (`GET /v1/account/me/proxies`) the session's traffic goes out through.
  That is the `proxy_id` the create named, or the one a successful
  `POST /v1/agent-sessions/{id}/egress` moved it to. It is set from the
  create's response onwards and does not follow a profile's proxy setting.
  Every agent session created since proxy_id became required has one; `nil` only on an older
  session created without a proxy of yours, and when an older server did not
  send the field.
- **The 2026-09 plans, available from 2026-09-29.** A new account starts on
  the 7-day free trial (`TierTrial`, `"trial"`), and `TierStarterV3`,
  `TierProV3` and `TierTeamV3` can be bought in the dashboard; Scale
  (`TierScaleV3`) is sold by talking to us. The earlier plans' constants
  stay: an account that holds one keeps it. No new id reuses an old one.
- **`AccountSelfProfile.TrialEndsAt` and `.TrialEnded`.** When the account's
  free trial ends (or ended), `nil` when it is not on a trial; and whether it
  has ended. An account whose trial has ended can still sign in and read
  everything, but starting a session is refused until it subscribes.
- **`AgentIntentResult.Warning`** — `*AgentStepWarning`, something worth
  knowing about a step that succeeded; `nil` means there is nothing to report.
  Its one kind today is `"http_error_status"`: a navigation reached the site
  and the site answered with an HTTP status of 400 or above, carried in
  `Status`. The step still succeeded — the page that loaded may be an error
  page, a page asking to sign in or to complete a verification step, or the
  whole page served under that status — and its `Summary` says what the site
  answered. `Kind` is an open set: treat a value you do not recognise as a
  note and read `Summary`.
- **`AgentSessionCapabilityReport.EgressState` can read
  `"default_connection_down"`** — only on a session created before proxy_id became required,
  without a proxy of its own: the connection Driftstack provided then stopped
  carrying traffic while the session ran. It was on our side. No session
  created since reports it, because every one runs through a proxy of yours. `"dead_proxy"` keeps its meaning — your own proxy stopped. On
  `GET /v1/sessions/{id}` the same fact arrives as the
  `default_connection_down` code in `EgressCapabilities.Warnings`, and for
  that session a connection without UDP reads `quic_unavailable` and a failed
  check that its traffic left the right way reads `safeguard_failed` — never
  `udp_unsupported_by_proxy` or `safeguard_failed:proxy_egress_verification`,
  which name a proxy it does not have.
- **Error code `"default_egress_unavailable"`** in `AgentSessionErrorEvent.Code`,
  and as `AgentSession.ClosedReason` — only on a session created before
  proxy_id became required, without a proxy of its own, whose connection (the one Driftstack
  provided then) failed. It arrives with `CustomerActionable` `false` — ours to
  fix. No session created since can report it. `Code` stays a `string`, so nothing to change
  unless you branch on it.

### Deprecated

- **`EmailAlreadyRegisteredError`, `InvalidCredentialsError`,
  `InvalidAuthTokenError`, `EmailNotVerifiedError`, `MfaStepUpRequiredError`
  and `LegalAcceptanceRequiredError`.** Only dashboard sign-in and dashboard
  actions raise them, so a program using an API key does not receive them.
  They stay so code that matches them keeps compiling.
- **`AccountResource.Me`** — the dashboard's profile read, not part of the
  public API. Use `Whoami`. It keeps working until a future minor release.
- **`AgentSession.PairModeState`** — the desktop app's takeover state; a
  program's own sessions are `"ai"`. It is now `omitempty` and may be
  removed in a later release.

### Changed

- `Sessions.Search`, `Sessions.Login`, `Egress.AttachToSession`,
  `Egress.GetSessionProxy` and `AgentSessions.SetEgress` stay, marked
  **not available yet** in their doc comments: the server does not serve
  them on any deployment today, and they are not in the API reference until
  it does.

### Security

- **`EgressResource.UpdateProxy` needs `"password"` when the address
  changes.** A body that changes `"host"`, `"port"` or `"scheme"` (`socks5` ↔
  `http`) of a saved proxy that stores a password must now carry
  `"password"` too — the password, or `nil` to clear it. Without it the
  server answers `400` and nothing changes: _To change this proxy’s address,
  send its password again (or null to remove it). A saved password is never
  sent to a new server on its own._ Before, a `"password"` left out of the
  map was always kept, so anyone able to edit a proxy could send its password
  to a server of their choosing. Leaving `"password"` out of an update that
  keeps the address — a `"label"` or `"username"` change, or a resend of the
  same address — keeps the stored password as before. On an `openvpn` or
  `wireguard` proxy, change `"host"` or `"port"` by sending `"scheme"` with
  the full VPN configuration. The refused attempt is recorded in the account's
  audit log (read it in the dashboard) as `"proxy.move_refused"`. Shipped without a deprecation window under
  the API's [security-fix policy](https://docs.driftstack.io/api/versioning/#security-fixes).

## [0.4.0] - 2026-09-22

**Nothing was removed.** Every method, field and error type v0.3.0 published
still means the same thing, so upgrading takes no code change on its own —
except where **Migration** below says a string comparison needs updating.

### Added

- **`EgressCapabilities.Safeguards`** — `*string`, one of `"passed"`,
  `"failed"` or `"unverified"`, summarising whether every egress safeguard
  held for a session. `"failed"` wins whenever any check did not pass;
  `"passed"` only when the device declared the full set of checks a healthy
  session reports and every one of them reported back; `"unverified"`
  otherwise. `nil` on a session reported before this field existed — never
  read a `nil` `Safeguards` as `"unverified"` or `"passed"`. Rides everywhere
  `EgressCapabilities` already does: `client.Sessions.Get` / `.List` /
  `.Create`, `client.Profiles.Launch`, and the
  `session.egress_capability_changed` webhook payload.
- **`AccountProxyTestResult.MeasuredBy`** on a `?check=full` proxy test
  result (`client.Egress.TestProxy`) — `"phone"` when a real phone session
  took the measurement, `"driftstack"` when Driftstack itself did because no
  phone could be reached in time. `MeasuredFrom`, the field this replaces, is
  still sent beside it with its original values for existing integrations
  (now doc-commented `Deprecated`), but is no longer documented; read
  `MeasuredBy` from here on.
- **`AccountProxyOsFingerprint.DirectReading` and `.WebsiteLikeReading`** —
  the same two facts `SingleHostVantage` and `WebPortVantage` already carry,
  under plainer names, added beside the originals rather than replacing
  them. Present on the proxy test result **and** on each saved
  proxy returned by `client.Egress.ListProxies` / `.UpdateProxy`.
- **`"page_unreadable"`** joins the `NoticeReason` values a `plan-executed`
  turn can carry: the page could not be read to plan the next step, so the
  task stopped rather than guess. Send `continue` to try again.

### Changed

- **Two dead `EgressCapabilities.Warnings` codes are retired from the
  documentation**: `quic_disabled_fallback_http2` and
  `dns_remote_resolve_unsupported_by_proxy`. Neither has ever been sent, so
  this is a documentation correction, not a behavioural change. `Warnings`
  stays `[]string`.

### Migration

Two closed-string fields were narrowed — values removed, not added — on
`AccountProxyTestResult`, the result of `client.Egress.TestProxy`. Both
fields are plain `string`, not a Go enum type, so neither change fails to
compile: code comparing a value against one of the old strings simply stops
matching, silently. The server has sent the new values only since
2026-09-21.

- **`NotRun`** — `"node_busy"`, `"node_error"` and `"no_node"` merged into
  `"check_unavailable"` (you can do exactly one thing about any of the
  three: try again shortly, or contact support if it persists);
  `"unresolvable"` is now `"config_unresolvable"`, matching the word the
  "why a launch is refused" vocabulary already used for the identical fact.
  `"live_session"` is unchanged.
- **`OsFingerprintUnavailable`** — `"vpn_tunnel"` is now
  `"not_available_for_vpn"`, `"not_observed"` is now `"not_captured"`, and
  `"observer_off"` is now `"not_offered_here"`.

Update any code that compares `NotRun` or `OsFingerprintUnavailable` against
one of the old strings to compare against its replacement instead.

## [0.3.0] - 2026-09-20

The release the guide [Run AI tasks from your
code](https://docs.driftstack.io/guides/run-ai-tasks-from-code/) is written
against — the last tag, `packages/sdk-go/v0.1.6`, cannot run any of its
examples, because the AI agent was not reachable from it at all.

**Read this if you are upgrading from v0.1.6**, which almost everyone is:
`v0.2.0` below was written but the tag was never pushed, so nothing in it
ever reached a customer. Going from v0.1.6 to v0.3.0 therefore brings BOTH
entries, and everything a v0.1.6 program may need changed is collected under
**Migrating from v0.1.6** here rather than left in the older entry. Of the
three removals `v0.2.0` made, two are back as deprecated constants for the
removal window the versioning policy asks for, so exactly one thing still
fails to compile. Everything else is additive: the client went from 4
resources to 19, and no method or error type present in v0.1.6 was removed
or renamed.

### Migrating from v0.1.6

Four things a v0.1.6 program may need changed. Two fail at compile time; the
other two compile as they are, so read them rather than waiting for the
compiler to raise them.

1. **The `AccountTier` constants are the current plan names.** The single
   pricing ladder became two. `TierFree` and `TierEnterprise` are unchanged,
   and the ladder is now:

   ```go
   TierFree         // "free"
   TierSoloManual   // "solo_manual"
   TierTeamManual   // "team_manual"
   TierAgencyManual // "agency_manual"
   TierAPIStarter   // "api_starter"
   TierAPIBuilder   // "api_builder"
   TierAPIScale     // "api_scale"
   TierEnterprise   // "enterprise"
   ```

   `TierStarter`, `TierSolo`, `TierBuilder` and `TierScale` still compile.
   They are **deprecated** and will be removed in a later MINOR release, and
   they keep the wire values they had in v0.1.6 — values the server no longer
   returns, so a comparison against one is now always false. That, not the
   compile, is what to fix. `TierStarter` → `TierAPIStarter`, `TierBuilder` →
   `TierAPIBuilder` and `TierScale` → `TierAPIScale` are the closest matches;
   `TierSolo` has no single successor — decide between `TierSoloManual` and
   `TierTeamManual` by what the account actually pays for. A `switch` over
   `AccountTier` should gain a `default`: the ladder has changed once and may
   again.

2. **The two quota webhook events are no longer sent.**
   `EventQuotaWarning80Pct` (`quota.warning_80pct`) and `EventQuotaExceeded`
   (`quota.exceeded`) still compile — also **deprecated**, also to be removed
   in a later MINOR release — but nothing dispatches them and the create and
   update schemas reject them, so drop them from any `Events` slice you
   build. No event replaced them: read `Quotas` from
   `client.Usage.CurrentPeriod(ctx)` to watch headroom instead. The
   constants that remain, plus the ones added since, are
   `EventSessionCompleted`, `EventSessionFailed`,
   `EventSessionChallengeDetected`, `EventSessionProfileSaveFailed`,
   `EventSessionEgressCapabilityChanged`, `EventAPIKeyRevoked`,
   `EventCryptoOrderPaid`, `EventCryptoOrderFailed` and `EventTestPing`.

3. **`CreateWebhookRequest.Description` is a `*string`.** It was a `string`
   with `omitempty`, which cannot tell "no description" from "the empty
   string". Pass a pointer:

   ```go
   // v0.1.6
   &driftstack.CreateWebhookRequest{URL: u, Events: ev, Description: "billing"}
   // v0.3.0
   desc := "billing"
   &driftstack.CreateWebhookRequest{URL: u, Events: ev, Description: &desc}
   ```

   Leave it `nil` to send no description at all. This is the removal that
   could not become a deprecated alias: a field cannot carry both types at
   once.

4. **Seven structs that existed in v0.1.6 gained fields**, so an unkeyed
   (positional) composite literal over any of them no longer compiles — name
   the fields instead, and it stays compiling the next time one gains a
   field. They are `Session`, `SessionState`, `CreateSessionRequest`,
   `InteractAction`, `WebhookEndpoint`, `VerifyWebhookOptions` and `Client`.

   ```go
   // v0.1.6 — no longer compiles, the struct has four more fields now:
   driftstack.CreateSessionRequest{"run-42", nil}
   // Keyed, and it stays compiling the next time a field is added:
   driftstack.CreateSessionRequest{Label: "run-42"}
   ```

### Added

#### Run an AI task end to end

`client.AgentSessions` is new, and is the whole AI surface.

- **Start it, send the task, close it** — `Create(ctx, body, opts...)` opens a
  session, on a saved profile if you pass one; `Message(ctx, id, text, opts)`
  sends the task in plain words and returns what happened;
  `Get` / `List` / `Iterate` read sessions back; `Close(ctx, id)` ends one and
  saves the profile's sign-in. A new session is `provisioning` until its
  browser is ready, then `active`.
- **Every way a turn can end is a named result kind** — `plan-executed` (the
  steps ran, with `Answer` when you asked a question), `clarify` (the agent is
  asking you something), `refuse` (it will not do this), `stopped`, and a step
  held for your approval. `AnswerUnavailable` says why there is no `Answer`
  when you asked for one.
- **Live progress while it runs** — `MessageOptions.OnStep` receives each
  `AgentStepEvent` as the step finishes and `MessageOptions.OnEvent` every
  other progress event (`phase`, `plan`, `step_start`, `answer`, `notice`, and
  any added later), under the same byte ceiling and deadline as before.
- **Approve a step, or don't** — a step with real-world consequences (a
  payment, a message sent, something deleted) pauses the turn and comes back
  as a `confirmation_required` result. `ApprovalFor(result)` turns that result
  into the approval that releases it; send it on the next `Message` with the
  same task. An unattended job simply never does.
- **Stop a task that runs too long** — `Stop(ctx, id)` asks the running turn
  to stop; the waiting `Message` then returns kind `stopped` rather than an
  error.
- **Screenshots** — `GetCapture(ctx, id, captureID)` returns the image behind
  a `capture` step's `captureId` as `*AgentCapture` (`ContentType`
  `image/png` or `image/jpeg`, plus `Bytes`). Screenshots are kept only
  briefly, so fetch one as soon as its turn ends; one that is no longer kept
  is a `*NotFoundError`.
- **Transcripts** — `Transcript(ctx, id, opts, fn)` calls `fn` with every
  entry of the session's conversation, oldest first, and then with each new
  one as it is written. Return `false` from `fn` to stop, or cancel the
  context. `TranscriptOptions.LastEventID` resumes after the last `Index` you
  saw. Adds `AgentTranscriptEntry`, `AgentTranscriptEvent` and
  `TranscriptOptions`.
- **Why a turn handed back, in one word** — `AgentMessageResponse.NoticeReason`
  sits beside `Notice`: `"step_limit"`, `"time_limit"`, `"budget_low"`,
  `"no_progress"`, `"repeated_step"`, `"ai_unavailable"`, `"question"` or
  `"declined"`. The set is OPEN — a `switch` over it needs a `default` that
  shows `Notice` — and the field is empty against an older server. The
  streamed `notice` event carries the same pair.
- **When it is safe to retry a message** — a refusal that did no work leaves
  its `Idempotency-Key` free: after a 409 whose `TurnInProgress()` is true, a
  429, a 402, a 403 about the plan's AI or the model, or a 502 whose
  `KeyRejected()` is false, send the same request again with the **same**
  `MessageOptions.IdempotencyKey`. Any other failure gets a new one. One
  message is one key, always.
- **Typed refusals you can act on** — `(*ForbiddenError).RequiresOwnKey()` /
  `.Model()` (this model needs your own Anthropic key);
  `(*ConflictError).TurnInProgress()`, `.SessionStatus()`,
  `.IdempotencyStatus()`, `.AIControlUnavailable()`, `.Phase()`,
  `.TokensConsumed()`, `.Usage()`, `.PartialResults()` and `.ClosedReason()`
  (why a closed session ended, without a second call);
  `(*FeatureUnavailableError).StopUnconfirmed()` (the one `Stop` 503 worth
  calling again); and `(*ByokAnthropicRequiredError).KeyRejected()` /
  `.KeySource()` / `.KeyRejectedReason()` (your own key was refused, which
  key, and why).
- **Send your own Anthropic key** — `CreateOptions.ByokAPIKey` at create, so a
  session can run on a model that requires one without storing anything.
  `CreateAgentSessionRequest` also reaches `SkipProxyProbe` and
  `ContinueFromAgentSessionID`.
- **Typed steps** — `AgentIntent`, `AgentIntentResult`,
  `AgentFailureDiagnosis`, and `ParsedResults()` / `ParsedIntents()` to read
  them. `Results` and `Intents` stay `[]json.RawMessage`, so a shape this SDK
  has not seen still arrives intact.
- **`examples/agent_chat`** is the complete flow end to end: create, wait
  until ready, send a task with a fresh idempotency key and live progress,
  handle every result kind, close with `defer`.

#### Watch one live, or take the wheel

- **`LivekitToken(ctx, id)`** — a `*LiveKitInfo` for the live video view of a
  running session, so a person can watch it work.
- **`SetMode(ctx, id, body)`**, **`SendInputEvent(ctx, id, body)`** and
  **`Resume(ctx, id)`** drive a session a person is holding, and pick one back
  up. **`SetEgress(ctx, id, body)`** changes which of your proxies a running
  session goes out through.
- **`Takeover(ctx, id, clientID)`** and **`Handback(ctx, id)`** hand control
  of a running session between the agent and a person, and hand it back. Both
  return a `*PairModeStateEnvelope` whose `PairModeState` is a
  `map[string]any`, so you can branch on `["kind"]` without a second call. An
  invalid transition, or a session that is not in pair mode, is a typed 409.

#### The rest of the API

`client.Sessions`, `client.APIKeys`, `client.Usage` and `client.Webhooks`
were the whole client in v0.1.6, and each of the four gained methods:
`Webhooks` gained `Update` (partial update — pointer fields tell "leave as
is" from "set", and it does NOT rotate the signing secret), `RotateSecret`
(fresh secret shown once, previous one valid for 24h, both signatures sent
during the window), `SendTest` (a synthetic `test.ping` delivery so you can
check your handler before depending on it), `ReplayDelivery` and
`IterateDeliveries`; `APIKeys` gained `Rotate`.

And fourteen resources are new:

- **`client.Profiles`** — `Create`, `List`, `Iterate`, `Get`, `Update`,
  `Delete`, plus `Clone(ctx, profileID, nil)` to let the server name the copy
  `(copy)` / `(copy 2)` / … and `Trim`.
- **`client.ProfileSnapshots`** — immutable point-in-time copies of a
  profile: `Capture`, `ListForProfile`, `List`, `Iterate`, `Get`, `Restore`,
  `Delete`. `Restore` creates a NEW profile; the original is never modified.
- **`client.Account`** — `Me(ctx)` returns `*AccountSelfProfile` with the
  full account: slug, region, avatar, whether MFA is enrolled, team
  memberships. Plus `UpdateMe`, `UploadAvatar` / `ClearAvatar`,
  `ListWebSessions` / `RevokeWebSession` / `RevokeAllOtherWebSessions`, and
  `RateLimits` for the limits actually in force on your account.
- **`client.Auth`** — sign-up, e-mail verification, log in, magic links,
  password reset, refresh, log out, and the three-call activation flow a CLI
  or desktop app uses instead of asking for a pasted key:
  `CliAuthorizeInitiate` returns a `Code` and a `BrowserURL`, the person signs
  in and authorises, and `CliAuthorizeExchange` reports `pending`, then
  `bound` once (with `APIKey` + `AccountID`), then `expired`.
- **`client.Mfa`** — `Status`, `Enroll`, `Verify`, `Disable`,
  `RegenerateRecoveryCodes`; plus `Auth.MfaChallenge` to exchange a login
  challenge for a session (the response's `Via` is `"totp"` or `"recovery"`)
  and `Auth.MfaStepUp` to refresh the freshness window an operation asked
  for. Pair it with `*MfaStepUpRequiredError`: catch, step up, retry.
- **`client.Team`** — members, invites, roles, and `ListOwners(ctx)` for the
  workspaces your account has joined.
- **`client.AuditLog`** — `List` / `Iterate`, and `Export(ctx)`: a
  single-call JSON export of your account's audit log, up to 10,000 rows,
  with `Truncated` set when there were more.
- **`client.Billing`** — `GetState`, `CreateCheckoutSession`,
  `CreatePortalSession`.
- **`client.CryptoOrders`** — `Quote`, `CreateCheckout` (takes
  `*CreateCheckoutOptions{IdempotencyKey}` so a retry cannot mint a second
  order), `List`, `Iterate` (the visit callback walks every page — return
  `false` to stop early), `Get`, `UpdateNote`, `Cancel`, `Receipt`. Returned
  envelopes are forward-compatible `map[string]any` envelopes, so a field
  added server-side arrives without an SDK release. Crypto payments are not
  refundable, and cancelling only works while an order is pending.
- **`client.Egress`** and account proxies — manage saved proxies, and route a
  session's traffic through one with `ProxyID` on agent-session create. An
  unknown or not-owned proxy id is a 404.
- **`client.Archetypes`** — the device archetypes your plan can use.
- **`client.Recipes`** — `Create(ctx, CreateRecipeRequest{...})` snapshots a
  finished agent session's steps and transcript into a recipe you can replay.
  `Label` is 1–120 characters after trimming and `Description` is optional
  and omitted from the wire when empty. The returned `Recipe.AgentSessionID`
  is a `*string`, so a recipe that outlived its source session decodes
  cleanly as `nil`.
- **`client.EmailPreferences`** — `List`, `Set`, `OptIn`, `OptOut`.
- **`client.Legal`** — record acceptance of a document version.

#### Errors

- **New typed errors, each with an `errors.Is` sentinel** —
  `BadRequestError`, `InternalError`, `FeatureUnavailableError`,
  `MfaStepUpRequiredError`, `EmailAlreadyRegisteredError`,
  `InvalidCredentialsError`, `InvalidAuthTokenError`,
  `EmailNotVerifiedError`, `ByokAnthropicRequiredError`,
  `ProxyValidationFailedError`, `StorageQuotaExceededError`,
  `PairModeConflictError` and `PairModeStateInvalidTransitionError`. You can
  `errors.As(err, &InvalidCredentialsError{})` on login instead of falling
  through to the unknown case.
- **`VerifyWebhookSignature` accepts a previous-secret signature.** You
  rarely need it: during a rotation grace window both signatures arrive
  inside the one `x-driftstack-signature` header, which the verifier already
  checks.

### Changed

- **Too many AI turns is a `*RateLimitError` now.** This refusal answers as
  `rate-limited` with `RetryAfterSeconds` 5, so `Message` returns
  `*RateLimitError` (and `IsRetryable` is true) where it returned
  `*ConcurrencyLimitError`. `*ConcurrencyLimitError` still means exactly what
  it always meant on `Create`: your plan's limit on sessions running at once.
- **A generic 400 is a `*BadRequestError` now**, not a `*ValidationError`. A
  400 that carries field-level issues is still a `*ValidationError`. Callers
  matching `errors.As(err, &ValidationError{})` or
  `errors.Is(err, ErrValidation)` on a generic 400 should switch to
  `*BadRequestError` / `ErrBadRequest`. `IsRetryable` is unaffected — both
  400 types stay non-retryable.
- **`Intents` on a `plan-executed` result** covers every plan the turn made,
  not only the first.
- **The agent-session documentation describes what the API does** — result
  kinds, how an approval resumes paused steps, when a key may be reused, the
  progress event names — and no longer describes how the service is built.

### Fixed

- **`AgentMessageResponse.Answer` is readable.** It was dropped when the
  response was decoded, so a Go caller could not read the answer to the
  question their own task asked, at all.
- **The docs on `AgentSession.Model` and `CreateAgentSessionRequest.Model`
  named the wrong default.** The default is `"claude-sonnet-5"`.
- **Auth responses match what the API sends.** `LoginResponse`,
  `VerifyEmailResponse`, `MagicLinkConsumeResponse`,
  `PasswordResetConfirmResponse` and `RefreshSessionResponse` were flat
  `{AccountID, SessionToken, ExpiresAt}` structs; the API returns a nested
  `{session: …}`, so every one of them failed to decode. They are nested now,
  with a new `WebSession` struct, and `LoginResponse` carries the
  MFA-required branch (`MfaRequired`, `ChallengeToken`,
  `ChallengeExpiresAt`) to branch on.
- **Auth request fields match what the API accepts.**
  `RefreshSessionRequest` and `LogoutRequest` sent `session_token`; the API
  expects `token`. `SignupResponse` described fields the API never returned.
- **`Profile`, `CreateProfileRequest` and `UpdateProfileRequest` match the
  API.** They carried fields the API does not return and silently dropped,
  and were missing `Archetype` — so you could not pin a non-default archetype
  on create.
- **`WebhookEndpoint`** was missing the rotation fields and the delivery
  counts.
- **`Subscription`** described 5 fields where the API returns 8, and typed
  `StripeSubscriptionID` as nullable when it is always present.
- **`SessionPurpose` values the API actually accepts.** Three of the four
  constants matched no server value and would have been rejected with a 400.

## [0.2.0] - 2026-05-05

> ⚠️ **Never tagged.** `packages/sdk-go/v0.2.0` was written but the tag was
> never pushed, so this release never reached a customer; everything in it
> shipped for the first time in v0.3.0. Kept here as history — if you are
> upgrading from v0.1.6, the **Migrating from v0.1.6** section of v0.3.0 is
> the one to read, because it accounts for later changes to the same symbols.

### Added

- **`AuthResource`** — new `client.Auth` for `/v1/auth/*` flows:
  `Signup`, `VerifyEmail`, `Login`, `RequestMagicLink`,
  `ConsumeMagicLink`, `RequestPasswordReset`, `ConfirmPasswordReset`,
  `Refresh`, `Logout`. Mirrors the TypeScript + Python SDK shape.
- **`BillingResource`** — new `client.Billing` for `/v1/billing`:
  `GetState`, `CreateCheckoutSession`, `StartTrialPack`,
  `CreatePortalSession`. Subscription + trial-pack state shapes
  added (`Subscription`, `TrialPackState`, `GetBillingStateResponse`).
- **`ProfilesResource`** — new `client.Profiles` for `/v1/profiles`:
  `Create`, `List`, `Iterate`, `Get`, `Update`, `Delete`. Iterator
  walks cursor pages; callback returns `(continue, error)`.
- **`SessionPurpose`** type + constants
  (`PurposeProductionCustomer`, `PurposeRecaptureRun`,
  `PurposeFingerprintProbe`, `PurposeBehaviouralCapture`) +
  `DefaultSessionPurpose`. `CreateSessionRequest.Purpose` and
  `Session.Purpose` fields exposed.
- **`examples/billing_flow/main.go`** — server-side billing self-
  serve example.

### Changed

- **BREAKING — `AccountTier` enum** — replaced legacy values
  (`free`, `starter`, `solo`, `builder`, `scale`, `enterprise`) with
  the two-ladder restructure (`trial_pack`, `solo_manual`,
  `team_manual`, `agency_manual`, `api_starter`, `api_builder`,
  `api_scale`, `enterprise`). Old constants removed; consumers
  must update. Pre-1.0 SemVer permits the breakage.
- **`APIKeyScope` enum** — added `ScopeAccountOwner`,
  `ScopeDriftstackInternalAdmin`, `ScopeGUIControl`. The legacy
  `ScopeAdmin` token remains a compat alias.
- **`CreateSessionRequest.Archetype`** field exposed (server defaults
  to the locked archetype if empty, matching schema).

## [0.1.6] - 2026-05-03

Written on 2026-09-20 from the tagged tree: `packages/sdk-go/v0.1.6` was
tagged and published without a CHANGELOG heading of its own.

### Added

- **`LegalAcceptanceRequiredError`** — a 409 that asks you to accept a
  document version is its own error type now, carrying the pending
  acceptances (`PendingAcceptance.DocumentKey` +
  `.CurrentVersion`) as `PendingAcceptances`. Sentinel
  `ErrLegalAcceptanceRequired` for `errors.Is`.

## [0.1.5] - 2026-05-03

### Added

- **`SessionTimeoutError`** — new typed error struct mapping the
  `https://errors.driftstack.dev/session-timeout` problem type
  (status 504). Distinguished from `DriverError` so callers can
  react specifically to "the operation didn't finish within the
  per-call timeout I supplied" without conflating with downstream
  driver failures. Carries `TimeoutMs int` from the problem
  extension. Sentinel: `ErrSessionTimeout` for `errors.Is`
  matching.

  ```go
  err := client.Sessions.Interact(ctx, sid, body)
  if errors.Is(err, ErrSessionTimeout) {
      var ste *SessionTimeoutError
      if errors.As(err, &ste) {
          log.Printf("Op timed out after %d ms", ste.TimeoutMs)
      }
  }
  ```

- Test coverage at `errors_test.go::TestSessionTimeoutExtractsTimeoutMs`.

## [0.1.4] - 2026-05-03

### Removed

- `Offset` struct removed; `InteractAction.Offset` field dropped
  from the public surface. Same reason as `tap_at`: a coordinate
  primitive on the customer-facing schema lets the customer bypass
  the behavioral simulation layer for the offset portion of the
  interaction. Bounded coordinates are still coordinates.

### Migration

If your code constructs `InteractAction{Kind: "tap", Selector: ...,
Offset: &Offset{...}}` directly, drop the `Offset` field. The
`NewTapAction` constructor signature is unchanged (was already
selector-only). Re-express the intent through selector specificity:

```go
// Before (0.1.x):
action := InteractAction{
    Kind:     "tap",
    Selector: "button.cta",
    Offset:   &Offset{X: 0, Y: 50},
}

// After (0.1.4+):
action := NewTapAction("button.cta .icon-arrow")
```

Coordinate-level addressing for screenshot-driven workflows is not
part of the public API and is not exposed in this SDK.

## [0.1.3] - 2026-05-03

### Fixed

- `NewTimeCondition(ms)` now emits `kind: "time"` on the wire (was
  `"time_ms"`, which the server's discriminated-union parser
  rejected with 400). Every Go customer call to
  `client.Wait(ctx, sid, NewTimeCondition(...))` was silently
  failing in 0.1.0–0.1.2.
- `NavigateRequest` gained the `TimeoutMS` field
  (`json:"timeout_ms,omitempty"`). The Zod schema accepts an
  optional `timeout_ms` in 1000–120000 ms range; TS/Python SDKs
  both expose it. Go customers can now set per-call navigate
  timeout overrides. Range validation happens server-side.

### Added

- `TestWaitConditionConstructors` and `TestNavigateRequestMarshalling`
  in `types_test.go`. Wire-shape regression coverage now matches
  the InteractAction tests added in 0.1.1.

## [0.1.2] - 2026-05-03

### Changed

- Re-cut: `Offset` struct kept for backwards-source-compat but
  `tap_at` / `type_focused` constructors removed (`NewTapAtAction`,
  `NewTypeFocusedAction`). Customer-facing schemas stay
  intent-only.
- The desktop app's control input moved to a server-side surface
  that is not part of the public API and doesn't appear in this SDK.

## [0.1.1] - 2026-05-02

### Fixed

- `NewScrollAction(x, y)` was emitting `{"x", "y"}` on the wire
  instead of `{"delta_x", "delta_y"}` — silently no-op'd by the
  server's `delta_x: 0, delta_y: 0` defaults. Renamed struct fields
  to `DeltaX`/`DeltaY` with proper JSON tags. Constructor signature
  is parameter-name-only — calls still type-check.

### Added

- `tap_at` and `type_focused` constructors briefly added (subsequently
  removed in 0.1.2, to keep customer-facing schemas intent-only).
- `types_test.go` with marshalling round-trip tests for all
  `InteractAction` constructors. Catches the silent-noop class of
  bug locally before customer prod.

## [0.1.0] - 2026-05-02

### Added

- `Client` with sync-only API (Go's idiomatic concurrency is
  goroutines, not async/await; one client serves both shapes).
- Resource accessors mounted on the client: `Sessions` (9 methods),
  `APIKeys` (3), `Usage` (1), `Webhooks` (5). All take
  `context.Context` first.
- Error type hierarchy with sentinel errors (`ErrAuth`,
  `ErrRateLimit`, etc.) for `errors.Is` matching plus typed structs
  (`*RateLimitError`, `*ConcurrencyLimitError`, etc.) for `errors.As`
  payload extraction.
- `RetryConfig` + automatic retry on `*TransportError` and
  `*RateLimitError`. Honours `Retry-After`. `context.Cancel` aborts
  the retry loop between attempts.
- `VerifyWebhookSignature` helper (Stripe-style HMAC-SHA256,
  constant-time via `hmac.Equal`).
- Discriminated-union builders for `Interact` (`NewTapAction`,
  `NewTypeAction`, `NewScrollAction`, `NewPressAction`) and `Wait`
  (`NewSelectorCondition`, `NewSelectorHiddenCondition`,
  `NewURLMatchesCondition`, `NewTimeCondition`).
- 33 tests covering errors, retry, webhook signatures, and every
  resource method via `httptest.Server` mocks.
- 5 examples: `quickstart`, `error_handling`, `webhook_receiver`,
  `goroutine_pool`, `scraping_pipeline`.

### Build

- Module path:
  `github.com/driftstackdev/driftstack-api/packages/sdk-go`.
- Zero non-stdlib runtime dependencies.
- CI: `go vet` + `go test` on Ubuntu / Go 1.22.

### Notes

- `0.1.0` is the inaugural alpha release, tagged as
  `packages/sdk-go/v0.1.0` (Go modules sub-directory tagging
  convention). This note sat under `[Unreleased]` until 2026-09-20;
  it describes 0.1.0 and belongs here.

### Notes

- Types in `types.go` are hand-maintained, not codegen output —
  `oapi-codegen` doesn't yet support OpenAPI 3.1 nullable shorthand
  (`type: [string, null]`). Hand-writing is tractable at the current
  schema size and produces cleaner output.
- Same hand-written-over-codegen call as the TypeScript SDK.
