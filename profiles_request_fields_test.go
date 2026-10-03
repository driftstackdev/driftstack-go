package driftstack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// The bodies Profiles.Create and Profiles.Update put on the wire, compared
// byte for byte: a field left alone must be absent (never null), a Clear flag
// must be null (tags: []), and a set value must be the value.

const testProxyID = "a1b2c3d4-0000-4000-8000-000000000001"

func intPtr(n int) *int    { return &n }
func boolPtr(b bool) *bool { return &b }

// captureProfileBody runs one call against a stub that records the request
// body and answers a profile, and returns the body.
func captureProfileBody(t *testing.T, wantMethod string, call func(*Client) error) string {
	t.Helper()
	var body string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != wantMethod {
			t.Errorf("method=%q, want %q", r.Method, wantMethod)
		}
		body = decodeBody(t, r)
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(profileFixture("prf_x"))
	})
	if err := call(client); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestProfiles_Create_SendsTheLaunchSettings(t *testing.T) {
	t.Parallel()
	body := captureProfileBody(t, "POST", func(c *Client) error {
		_, err := c.Profiles.Create(context.Background(), &CreateProfileRequest{
			Name:               "fresh",
			DefaultProxyID:     testProxyID,
			Geolocation:        &ProfileGeolocation{Latitude: 48.8566, Longitude: 2.3522, Accuracy: 20},
			StopOnExitIPChange: true,
		})
		return err
	})
	want := `{"name":"fresh","default_proxy_id":"` + testProxyID + `",` +
		`"geolocation":{"latitude":48.8566,"longitude":2.3522,"accuracy":20},"stop_on_exit_ip_change":true}`
	if body != want {
		t.Errorf("body\n got %s\nwant %s", body, want)
	}
}

func TestProfiles_Create_LeavesTheLaunchSettingsOutWhenUnset(t *testing.T) {
	t.Parallel()
	body := captureProfileBody(t, "POST", func(c *Client) error {
		_, err := c.Profiles.Create(context.Background(), &CreateProfileRequest{Name: "fresh"})
		return err
	})
	if want := `{"name":"fresh"}`; body != want {
		t.Errorf("body\n got %s\nwant %s", body, want)
	}
}

func TestProfiles_Update_SendsEachNewField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		req  UpdateProfileRequest
		want string
	}{
		{"default proxy", UpdateProfileRequest{DefaultProxyID: stringPtr(testProxyID)},
			`{"default_proxy_id":"` + testProxyID + `"}`},
		{"geolocation", UpdateProfileRequest{Geolocation: &ProfileGeolocation{Latitude: -33.8688, Longitude: 151.2093}},
			`{"geolocation":{"latitude":-33.8688,"longitude":151.2093}}`},
		{"stop on exit IP change, true", UpdateProfileRequest{StopOnExitIPChange: boolPtr(true)},
			`{"stop_on_exit_ip_change":true}`},
		// false must reach the server: it turns the setting off.
		{"stop on exit IP change, false", UpdateProfileRequest{StopOnExitIPChange: boolPtr(false)},
			`{"stop_on_exit_ip_change":false}`},
		{"expected revision", UpdateProfileRequest{Name: stringPtr("renamed"), ExpectedRevision: intPtr(7)},
			`{"name":"renamed","expected_revision":7}`},
		{"notes", UpdateProfileRequest{Notes: stringPtr("door code 4411")},
			`{"notes":"door code 4411"}`},
		{"detached", UpdateProfileRequest{ProxyChoice: stringPtr("detached")},
			`{"proxy_choice":"detached"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := tc.req
			body := captureProfileBody(t, "PATCH", func(c *Client) error {
				_, err := c.Profiles.Update(context.Background(), "prf_x", &req)
				return err
			})
			if body != tc.want {
				t.Errorf("body\n got %s\nwant %s", body, tc.want)
			}
		})
	}
}

func TestProfiles_Update_EachClearFlagSendsNull(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		req  UpdateProfileRequest
		want string
	}{
		{"description", UpdateProfileRequest{ClearDescription: true}, `{"description":null}`},
		{"folder", UpdateProfileRequest{ClearFolder: true}, `{"folder":null}`},
		// tags is not nullable on the server: an empty list removes every tag.
		{"tags", UpdateProfileRequest{ClearTags: true}, `{"tags":[]}`},
		{"icon", UpdateProfileRequest{ClearIcon: true}, `{"icon":null}`},
		{"note", UpdateProfileRequest{ClearNote: true}, `{"note":null}`},
		{"notes", UpdateProfileRequest{ClearNotes: true}, `{"notes":null}`},
		{"default proxy", UpdateProfileRequest{ClearDefaultProxyID: true}, `{"default_proxy_id":null}`},
		{"geolocation", UpdateProfileRequest{ClearGeolocation: true}, `{"geolocation":null}`},
		{"a clear beside a set", UpdateProfileRequest{
			Name: stringPtr("renamed"), ClearNotes: true, ClearGeolocation: true, StopOnExitIPChange: boolPtr(false),
		}, `{"name":"renamed","notes":null,"geolocation":null,"stop_on_exit_ip_change":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := tc.req
			body := captureProfileBody(t, "PATCH", func(c *Client) error {
				_, err := c.Profiles.Update(context.Background(), "prf_x", &req)
				return err
			})
			if body != tc.want {
				t.Errorf("body\n got %s\nwant %s", body, tc.want)
			}
		})
	}
}

// updateProfileRequestV060 is UpdateProfileRequest exactly as v0.6.0 published
// it, with no MarshalJSON: what a program that sets none of the new fields
// sent before them.
type updateProfileRequestV060 struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Folder      *string  `json:"folder,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Icon        *string  `json:"icon,omitempty"`
	Note        *string  `json:"note,omitempty"`
	Notes       *string  `json:"notes,omitempty"`
	ProxyChoice *string  `json:"proxy_choice,omitempty"`
}

func TestProfiles_Update_ABodyWithoutTheNewFieldsIsUnchangedFromV060(t *testing.T) {
	t.Parallel()
	cases := []updateProfileRequestV060{
		{},
		{Name: stringPtr("renamed")},
		// An empty Tags is left out, as it was; ClearTags is how to send [].
		{Tags: []string{}},
		{Tags: []string{"eu", "warm"}},
		// "" is a value, not a clear.
		{Description: stringPtr(""), Notes: stringPtr("")},
		{
			Name: stringPtr("renamed"), Description: stringPtr("d"), Folder: stringPtr("Clients"),
			Tags: []string{"eu"}, Icon: stringPtr("🍏"), Note: stringPtr("n"), Notes: stringPtr("long notes"),
			ProxyChoice: stringPtr("detached"),
		},
	}
	for i, old := range cases {
		wantBytes, err := json.Marshal(old)
		if err != nil {
			t.Fatal(err)
		}
		want := string(wantBytes)
		req := UpdateProfileRequest{
			Name: old.Name, Description: old.Description, Folder: old.Folder, Tags: old.Tags,
			Icon: old.Icon, Note: old.Note, Notes: old.Notes, ProxyChoice: old.ProxyChoice,
		}
		body := captureProfileBody(t, "PATCH", func(c *Client) error {
			_, err := c.Profiles.Update(context.Background(), "prf_x", &req)
			return err
		})
		if body != want {
			t.Errorf("case %d: body\n got %s\nwant %s (v0.6.0)", i, body, want)
		}
		if strings.Contains(body, "null") {
			t.Errorf("case %d: a request that clears nothing sent a null: %s", i, body)
		}
	}
}

// The error is the caller's and resending cannot fix it, so it must not read
// as retryable. Before Update checked it, the encode failure came back as a
// *TransportError ("failed to marshal request body"), IsRetryable true, and
// the loop documented on IsRetryable resent it up to five times.
func TestProfiles_Update_AFieldAndItsClearFlagTogetherSendNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		field string
		req   UpdateProfileRequest
	}{
		{"Description", UpdateProfileRequest{Description: stringPtr("x"), ClearDescription: true}},
		{"Folder", UpdateProfileRequest{Folder: stringPtr("x"), ClearFolder: true}},
		{"Tags", UpdateProfileRequest{Tags: []string{"eu"}, ClearTags: true}},
		{"Icon", UpdateProfileRequest{Icon: stringPtr("x"), ClearIcon: true}},
		{"Note", UpdateProfileRequest{Note: stringPtr("x"), ClearNote: true}},
		{"Notes", UpdateProfileRequest{Notes: stringPtr("x"), ClearNotes: true}},
		{"DefaultProxyID", UpdateProfileRequest{DefaultProxyID: stringPtr(testProxyID), ClearDefaultProxyID: true}},
		{"Geolocation", UpdateProfileRequest{Geolocation: &ProfileGeolocation{Latitude: 1, Longitude: 2}, ClearGeolocation: true}},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			var hits atomic.Int32
			_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.WriteHeader(200)
			})
			req := tc.req
			_, err := client.Profiles.Update(context.Background(), "prf_x", &req)
			if err == nil {
				t.Fatal("Update returned no error for a field set beside its Clear flag")
			}
			if want := "sets both " + tc.field + " and Clear" + tc.field; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not say %q", err, want)
			}
			if n := hits.Load(); n != 0 {
				t.Errorf("the server was called %d times; a contradictory request must send nothing", n)
			}
			if IsRetryable(err) {
				t.Errorf("IsRetryable(%T %v) = true; resending a contradictory request cannot succeed", err, err)
			}
			if errors.Is(err, ErrTransport) {
				t.Errorf("err = %T %v reads as a transport failure; nothing reached the network", err, err)
			}
			// A caller who encodes the request itself is refused the same way.
			if _, err := json.Marshal(req); err == nil || !strings.Contains(err.Error(), "sets both "+tc.field) {
				t.Errorf("json.Marshal err = %v, want it to say %q", err, "sets both "+tc.field)
			}
		})
	}
}

func TestProfiles_Update_AStaleExpectedRevisionIsAConflictError(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	var body string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body = decodeBody(t, r)
		// What the server answers when the profile moved on since revision 7.
		w.Header().Set("content-type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"https://errors.driftstack.dev/conflict","title":"Conflict","status":409,` +
			`"detail":"This profile changed on another computer since you last loaded it. Reload it and try again.",` +
			`"resource":"profile","code":"stale_revision","expected_revision":7,"current_revision":8}`))
	})
	_, err := client.Profiles.Update(context.Background(), "prf_x", &UpdateProfileRequest{
		Notes:            stringPtr("mine"),
		ExpectedRevision: intPtr(7),
	})
	if want := `{"notes":"mine","expected_revision":7}`; body != want {
		t.Errorf("body\n got %s\nwant %s", body, want)
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %T %v, want *ConflictError", err, err)
	}
	if !errors.Is(err, ErrConflict) {
		t.Error("errors.Is(err, ErrConflict) is false")
	}
	if conflict.Status != http.StatusConflict {
		t.Errorf("status=%d, want 409", conflict.Status)
	}
	if code, _ := conflict.Problem["code"].(string); code != "stale_revision" {
		t.Errorf(`Problem["code"]=%v, want "stale_revision"`, conflict.Problem["code"])
	}
	if current, _ := conflict.Problem["current_revision"].(float64); current != 8 {
		t.Errorf(`Problem["current_revision"]=%v, want 8`, conflict.Problem["current_revision"])
	}
	// PATCH is not retried: the second computer's change must not be overwritten
	// by a resend, and the caller must reload first.
	if n := hits.Load(); n != 1 {
		t.Errorf("the server was called %d times, want 1", n)
	}
}
