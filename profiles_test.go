package driftstack

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func profileFixture(id string) Profile {
	return Profile{
		ID:        id,
		Name:      "test profile " + id,
		Archetype: "iphone16pro_ios18_7_safari26_4",
	}
}

func TestProfiles_Create(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/profiles" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body CreateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Name != "fresh" {
			t.Errorf("name=%q", body.Name)
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(profileFixture("prf_x"))
	})
	got, err := client.Profiles.Create(context.Background(), &CreateProfileRequest{Name: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "prf_x" {
		t.Errorf("id=%q", got.ID)
	}
}

func TestProfiles_List(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/profiles" || r.Method != "GET" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "25" {
			t.Errorf("limit=%q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(ProfilesListPage{
			Data:    []Profile{profileFixture("prf_1"), profileFixture("prf_2")},
			HasMore: false,
		})
	})
	got, err := client.Profiles.List(context.Background(), &ListProfilesQuery{Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 2 {
		t.Errorf("got %d profiles", len(got.Data))
	}
}

// 2026-10-08 — ListProfilesQuery.Status sends ?status=: ProfileListTrashed
// lists the trash, ProfileListActive the live profiles, and the zero value
// sends no status at all (the live profiles, unchanged).
func TestProfiles_List_Status(t *testing.T) {
	t.Parallel()
	if ProfileListActive != "active" || ProfileListTrashed != "trashed" {
		t.Fatalf("status values: %q %q", ProfileListActive, ProfileListTrashed)
	}
	cases := []struct {
		name  string
		query *ListProfilesQuery
		want  string
	}{
		{"trashed", &ListProfilesQuery{Status: ProfileListTrashed}, "status=trashed"},
		{"active", &ListProfilesQuery{Status: ProfileListActive}, "status=active"},
		{"with limit and cursor", &ListProfilesQuery{Limit: 25, Cursor: "prf_c", Status: ProfileListActive}, "cursor=prf_c&limit=25&status=active"},
		{"no status", &ListProfilesQuery{Limit: 10}, "limit=10"},
		{"nil query", nil, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/profiles" || r.Method != "GET" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.URL.RawQuery != tc.want {
					t.Errorf("query=%q, want %q", r.URL.RawQuery, tc.want)
				}
				w.Header().Set("content-type", "application/json")
				_ = json.NewEncoder(w).Encode(ProfilesListPage{
					Data:    []Profile{profileFixture("prf_1")},
					HasMore: false,
				})
			})
			got, err := client.Profiles.List(context.Background(), tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Data) != 1 || got.HasMore || got.NextCursor != nil {
				t.Errorf("page=%+v", got)
			}
		})
	}
}

// Iterate takes the same query struct, so it carries Status to every page:
// dropping it would walk the live profiles for a caller who asked for the trash.
func TestProfiles_Iterate_CarriesStatus(t *testing.T) {
	t.Parallel()
	requests := 0
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.URL.Query().Get("status"); got != "trashed" {
			t.Errorf("status=%q", got)
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(ProfilesListPage{
			Data:    []Profile{profileFixture("prf_1"), profileFixture("prf_2")},
			HasMore: false,
		})
	})
	seen := 0
	err := client.Profiles.Iterate(context.Background(), &ListProfilesQuery{Status: ProfileListTrashed},
		func(*Profile) (bool, error) {
			seen++
			return true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 2 || requests != 1 {
		t.Errorf("seen=%d requests=%d", seen, requests)
	}
}

func TestProfiles_Iterate_WalksCursorPages(t *testing.T) {
	t.Parallel()
	pageOne := ProfilesListPage{
		Data:       []Profile{profileFixture("prf_1"), profileFixture("prf_2")},
		HasMore:    true,
		NextCursor: stringPtr("prf_2"),
	}
	pageTwo := ProfilesListPage{
		Data:    []Profile{profileFixture("prf_3")},
		HasMore: false,
	}

	requestN := 0
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestN++
		w.Header().Set("content-type", "application/json")
		if r.URL.Query().Get("cursor") == "" {
			_ = json.NewEncoder(w).Encode(pageOne)
		} else {
			_ = json.NewEncoder(w).Encode(pageTwo)
		}
	})

	var seen []string
	err := client.Profiles.Iterate(context.Background(), nil, func(p *Profile) (bool, error) {
		seen = append(seen, p.ID)
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 {
		t.Errorf("seen %d profiles, expected 3 (got %v)", len(seen), seen)
	}
	if requestN != 2 {
		t.Errorf("requests %d, expected 2 (one per page)", requestN)
	}
}

func TestProfiles_Update_PartialPatch(t *testing.T) {
	t.Parallel()
	newName := "renamed"
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method=%q", r.Method)
		}
		var body UpdateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Name == nil || *body.Name != newName {
			t.Errorf("body.name=%v", body.Name)
		}
		updated := profileFixture("prf_x")
		updated.Name = newName
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(updated)
	})
	got, err := client.Profiles.Update(context.Background(), "prf_x", &UpdateProfileRequest{Name: &newName})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != newName {
		t.Errorf("name=%q", got.Name)
	}
}

func TestProfiles_Delete_Idempotent(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method=%q", r.Method)
		}
		w.WriteHeader(204)
	})
	if err := client.Profiles.Delete(context.Background(), "prf_x"); err != nil {
		t.Errorf("err=%v", err)
	}
}

// 2026-10-05 — Delete with &DeleteProfileOptions{Permanent: true} sends
// ?permanent=true; a plain Delete (and Permanent: false, and a nil option)
// sends no query, so it keeps moving the profile to the trash.
func TestProfiles_Delete_Permanent(t *testing.T) {
	t.Parallel()
	var queries []string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/v1/profiles/prof_x" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		queries = append(queries, r.URL.RawQuery)
		w.WriteHeader(204)
	})
	ctx := context.Background()
	if err := client.Profiles.Delete(ctx, "prof_x", &DeleteProfileOptions{Permanent: true}); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := client.Profiles.Delete(ctx, "prof_x"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := client.Profiles.Delete(ctx, "prof_x", &DeleteProfileOptions{Permanent: false}); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := client.Profiles.Delete(ctx, "prof_x", nil); err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{"permanent=true", "", "", ""}
	if len(queries) != len(want) {
		t.Fatalf("queries=%q, want %q", queries, want)
	}
	for i := range want {
		if queries[i] != want[i] {
			t.Errorf("call %d query=%q, want %q", i, queries[i], want[i])
		}
	}
}

func TestProfiles_Clone_DefaultBodyEmpty(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/profiles/prof_src/clone" || r.Method != "POST" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		var body CloneProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Name != "" {
			t.Errorf("default name should be empty, got %q", body.Name)
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(profileFixture("prof_copy"))
	})
	got, err := client.Profiles.Clone(context.Background(), "prof_src", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "prof_copy" {
		t.Errorf("id=%q", got.ID)
	}
}

func TestProfiles_Clone_ExplicitName(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body CloneProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Name != "my-explicit-clone" {
			t.Errorf("name=%q", body.Name)
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(profileFixture("prof_x"))
	})
	_, err := client.Profiles.Clone(
		context.Background(),
		"prof_src",
		&CloneProfileRequest{Name: "my-explicit-clone"},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestProfiles_Trim_OkPersistsSize(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/profiles/prof_src/trim" || r.Method != "POST" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(TrimProfileResponse{
			Status:         "ok",
			SizeBytes:      4000,
			BytesReclaimed: 6000,
		})
	})
	got, err := client.Profiles.Trim(context.Background(), "prof_src")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "ok" {
		t.Errorf("status=%q", got.Status)
	}
	if got.SizeBytes != 4000 {
		t.Errorf("size_bytes=%d", got.SizeBytes)
	}
	if got.BytesReclaimed != 6000 {
		t.Errorf("bytes_reclaimed=%d", got.BytesReclaimed)
	}
}

func TestProfiles_Trim_Unavailable(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(TrimProfileResponse{
			Status: "unavailable",
			Reason: "no fleet node is connected",
		})
	})
	got, err := client.Profiles.Trim(context.Background(), "prof_src")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "unavailable" {
		t.Errorf("status=%q", got.Status)
	}
	if got.Reason != "no fleet node is connected" {
		t.Errorf("reason=%q", got.Reason)
	}
	if got.SizeBytes != 0 || got.BytesReclaimed != 0 {
		t.Errorf("size/reclaim should be zero on unavailable, got %d/%d", got.SizeBytes, got.BytesReclaimed)
	}
}

func stringPtr(s string) *string { return &s }
