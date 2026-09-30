package driftstack

// V-436 — wire-shape regression tests. These tests deserialize
// hand-crafted JSON that mirrors the actual server response shapes
// (per `apps/server/src/routes/*` `publicX()` serializers) into Go
// SDK types. They catch regressions in the SDK struct definitions
// against the live wire format — separate from the `*_test.go`
// fixture-based tests that round-trip the SDK's own struct.
//
// Add a new case here when:
// - A new field is added on the server-side public shape.
// - A field's nullability changes.
// - A new endpoint's response shape lands in the SDK.

import (
	"encoding/json"
	"testing"
	"time"
)

// V-426 — Profile shape per server's CreateProfileResponseSchema
// (= ProfileSchema): { id, name, archetype, description?,
// last_used_at?, created_at, updated_at }.
func TestWireShape_Profile_RealServerShape(t *testing.T) {
	t.Parallel()
	raw := `{
		"id": "prof_00000000-0000-4000-8000-000000000001",
		"name": "shopper-eu",
		"archetype": "iphone16pro_ios18_7_safari26_4",
		"description": "EU shopping flow",
		"last_used_at": "2026-05-08T12:00:00.000Z",
		"size_bytes": 9000000000,
		"last_saved_at": "2026-05-08T12:00:00.000Z",
		"notes": "Signed in as the EU test shopper. Cart has two items.",
		"created_at": "2026-04-15T08:00:00.000Z",
		"updated_at": "2026-05-08T12:00:00.000Z"
	}`
	var got Profile
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "prof_00000000-0000-4000-8000-000000000001" {
		t.Errorf("ID=%q", got.ID)
	}
	if got.Archetype != "iphone16pro_ios18_7_safari26_4" {
		t.Errorf("Archetype=%q (expected to populate; was the V-426 missing field)", got.Archetype)
	}
	if got.Description == nil || *got.Description != "EU shopping flow" {
		t.Errorf("Description=%v", got.Description)
	}
	if got.LastUsedAt == nil {
		t.Errorf("LastUsedAt should be populated")
	}
	// doc-150 item 5 — size_bytes is *int64 (sealed stores can exceed 2^31).
	if got.SizeBytes == nil || *got.SizeBytes != 9000000000 {
		t.Errorf("SizeBytes=%v (expected 9000000000)", got.SizeBytes)
	}
	if got.LastSavedAt == nil {
		t.Errorf("LastSavedAt should be populated")
	}
	// Profile notes — free text on the resource; nil when the server sends null.
	if got.Notes == nil || *got.Notes != "Signed in as the EU test shopper. Cart has two items." {
		t.Errorf("Notes=%v", got.Notes)
	}
}

// V-427 — WebhookEndpoint includes V-185 delivery_counts +
// V-359 prev_secret_prefix + rotation_grace_expires_at.
func TestWireShape_WebhookEndpoint_RotationGraceState(t *testing.T) {
	t.Parallel()
	graceUntil := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	raw := `{
		"id": "whk_00000000-0000-4000-8000-000000000001",
		"url": "https://example.test/hook",
		"secret_prefix": "whsec_aabbccdd",
		"prev_secret_prefix": "whsec_old11122",
		"rotation_grace_expires_at": "` + graceUntil.Format(time.RFC3339) + `",
		"events": ["session.completed", "session.failed"],
		"description": null,
		"active": true,
		"consecutive_failures": 0,
		"last_success_at": null,
		"last_failure_at": null,
		"disabled_at": null,
		"delivery_counts": {"delivered": 42, "failed": 3, "dlq": 1},
		"created_at": "2026-04-01T00:00:00.000Z"
	}`
	var got WebhookEndpoint
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.PrevSecretPrefix == nil || *got.PrevSecretPrefix != "whsec_old11122" {
		t.Errorf("PrevSecretPrefix=%v (V-359 grace state field)", got.PrevSecretPrefix)
	}
	if got.RotationGraceExpiresAt == nil {
		t.Errorf("RotationGraceExpiresAt should be populated")
	}
	if got.DeliveryCounts.Delivered != 42 || got.DeliveryCounts.DLQ != 1 {
		t.Errorf("DeliveryCounts mismatch: %+v", got.DeliveryCounts)
	}
}

// V-433 — SessionPurpose enum constants must match server values.
// Sanity check against the canonical set.
func TestWireShape_SessionPurpose_CanonicalValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		serverWire string
		goConst    SessionPurpose
	}{
		{"production_customer", PurposeProductionCustomer},
	}
	for _, c := range cases {
		if string(c.goConst) != c.serverWire {
			t.Errorf("SessionPurpose %s: Go=%q, server=%q", c.serverWire, c.goConst, c.serverWire)
		}
	}
}
