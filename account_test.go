package driftstack

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAccount_Me(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"id":                        "acc_00000000-0000-4000-8000-000000000001",
		"email":                     "alice@driftstack.local",
		"name":                      "Alice",
		"tier":                      "api_builder",
		"status":                    "active",
		"timezone":                  "Europe/Amsterdam",
		"slug":                      "alice-co",
		"region":                    "eu",
		"avatar_url":                "https://r2.example/avatars/alice.png?sig=...",
		"avatar_source":             "user",
		"mfa_enrolled":              true,
		"concurrent_session_cap":    8,
		"concurrent_session_active": 2,
		"profile_cap":               50,
		"profile_count":             7,
		"profile_trash_count":       2,
		"profile_restores_used":     1,
		"profile_restores_limit":    10,
		"teams": []map[string]any{
			{
				"owner_account_id": "acc_00000000-0000-4000-8000-000000000099",
				"role":             "admin",
				"membership_id":    "mem_00000000-0000-4000-8000-000000000003",
			},
		},
	}
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/account/me" || r.Method != "GET" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	})
	got, err := client.Account.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "acc_00000000-0000-4000-8000-000000000001" {
		t.Errorf("id=%q", got.ID)
	}
	if got.Slug == nil || *got.Slug != "alice-co" {
		t.Errorf("slug=%v", got.Slug)
	}
	if got.Region == nil || *got.Region != "eu" {
		t.Errorf("region=%v", got.Region)
	}
	if got.AvatarSource != "user" {
		t.Errorf("avatar_source=%q", got.AvatarSource)
	}
	if !got.MfaEnrolled {
		t.Errorf("mfa_enrolled should be true")
	}
	if got.ProfileCap == nil || *got.ProfileCap != 50 {
		t.Errorf("profile_cap=%v", got.ProfileCap)
	}
	// 2026-10-08 — restores from the Trash used in the rolling 30 days, and
	// the number the plan allows.
	if got.ProfileTrashCount != 2 || got.ProfileRestoresUsed != 1 || got.ProfileRestoresLimit != 10 {
		t.Errorf("trash=%d restores used=%d limit=%d", got.ProfileTrashCount, got.ProfileRestoresUsed, got.ProfileRestoresLimit)
	}
	if len(got.Teams) != 1 || got.Teams[0].Role != "admin" {
		t.Errorf("teams=%v", got.Teams)
	}
}

func TestAccount_Me_NullableFields(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"id":                        "acc_00000000-0000-4000-8000-000000000001",
		"email":                     "x@y.z",
		"name":                      nil,
		"tier":                      "free",
		"status":                    "active",
		"timezone":                  nil,
		"slug":                      nil,
		"region":                    nil,
		"avatar_url":                nil,
		"avatar_source":             "none",
		"mfa_enrolled":              false,
		"concurrent_session_cap":    1,
		"concurrent_session_active": 0,
		"profile_cap":               nil, // null = enterprise / unmetered
		"profile_count":             0,
		"teams":                     []map[string]any{},
	}
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	})
	got, err := client.Account.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != nil {
		t.Errorf("name should be nil; got %v", got.Name)
	}
	if got.Slug != nil || got.Region != nil || got.AvatarURL != nil {
		t.Errorf("nullable fields should all be nil")
	}
	// A server older than the restore fields sends neither: both read 0.
	if got.ProfileRestoresUsed != 0 || got.ProfileRestoresLimit != 0 {
		t.Errorf("absent restore fields read %d/%d, want 0/0", got.ProfileRestoresUsed, got.ProfileRestoresLimit)
	}
	if got.ProfileCap != nil {
		t.Errorf("profile_cap should be nil for enterprise/unmetered")
	}
	if len(got.Teams) != 0 {
		t.Errorf("teams should be empty")
	}
}

// The six AccountResource methods Go's tests did not reach (V-1985). The TS SDK
// pins all of them; Go pinned Me, the four BYOK calls and the bulk web-session
// revoke, leaving the profile, avatar, session-listing and rate-limit surface
// unasserted.

func TestAccount_Whoami(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/whoami" || r.Method != "GET" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account_id": "acc_00000000-0000-4000-8000-000000000001",
			"api_key_id": "key_00000000-0000-4000-8000-000000000001",
			"tier":       "api_builder",
			"scopes":     []string{"read", "write"},
		})
	})
	got, err := client.Account.Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if got.AccountID != "acc_00000000-0000-4000-8000-000000000001" || got.APIKeyID != "key_00000000-0000-4000-8000-000000000001" {
		t.Errorf("ids: %+v", got)
	}
	if got.Tier != "api_builder" || strings.Join(got.Scopes, ",") != "read,write" {
		t.Errorf("tier/scopes: %+v", got)
	}
}

func TestAccount_RateLimits(t *testing.T) {
	t.Parallel()
	var method, path string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tier": "solo_manual",
			"buckets": []map[string]any{{
				"bucket_key": "global", "capacity": 60, "refill_per_second": 1.0,
				"source": "tier_default", "override_expires_at": nil,
			}},
		})
	})
	out, err := client.Account.RateLimits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if method != "GET" || path != "/v1/account/rate-limits" {
		t.Errorf("got %s %s, want GET /v1/account/rate-limits", method, path)
	}
	if out.Tier != "solo_manual" || len(out.Buckets) != 1 {
		t.Fatalf("out = %+v", out)
	}
	// refill_per_second is a float; decoding it as an int would silently floor
	// a sub-1/s bucket to zero and read as "never refills".
	if out.Buckets[0].RefillPerSecond != 1.0 || out.Buckets[0].Capacity != 60 {
		t.Errorf("bucket = %+v", out.Buckets[0])
	}
	if out.Buckets[0].OverrideExpiresAt != nil {
		t.Errorf("override_expires_at should decode as nil, got %v", *out.Buckets[0].OverrideExpiresAt)
	}
}

func TestAccount_ListCredentials(t *testing.T) {
	t.Parallel()
	var method, path string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"handle":             "cred_9f2c41ab7d6e4850bc13f07a5e9d2c88",
				"label":              "Shop password",
				"sites":              []string{"shop.example.com"},
				"include_subdomains": true,
				"created_at":         "2026-10-01T09:14:22.000Z",
			}},
		})
	})
	out, err := client.Account.ListCredentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if method != "GET" || path != "/v1/account/me/credentials" {
		t.Errorf("got %s %s, want GET /v1/account/me/credentials", method, path)
	}
	if len(out.Data) != 1 || out.Data[0].Handle != "cred_9f2c41ab7d6e4850bc13f07a5e9d2c88" ||
		out.Data[0].Label != "Shop password" || out.Data[0].CreatedAt != "2026-10-01T09:14:22.000Z" ||
		len(out.Data[0].Sites) != 1 || out.Data[0].Sites[0] != "shop.example.com" ||
		!out.Data[0].IncludeSubdomains {
		t.Fatalf("out = %+v", out)
	}
}
