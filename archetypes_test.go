package driftstack

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestArchetypesList(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/archetypes" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(ListArchetypesResponse{
			DefaultArchetypeID: "iphone17_ios18_7_safari26_4",
			Data: []PublicArchetype{{
				ID: "iphone17_ios18_7_safari26_4", DisplayLabel: "iPhone 17 / iOS 18.7 / Safari 26.4",
				Device: "iPhone 17", IOSVersion: "18.7", SafariVersion: "26.4",
				Status: "launch", IsDefault: true,
			}},
		})
	})

	got, err := client.Archetypes.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultArchetypeID != "iphone17_ios18_7_safari26_4" || len(got.Data) != 1 {
		t.Fatalf("unexpected catalog: %+v", got)
	}
	if !got.Data[0].IsDefault || got.Data[0].Status != "launch" {
		t.Fatalf("unexpected default archetype: %+v", got.Data[0])
	}
}

// GET /v1/archetypes names each entry's browser. An entry from an older server
// carries neither field; a value this SDK has never seen is kept as sent, and
// one such entry does not cost the caller the list.
func TestArchetypesListNamesEachEntrysBrowser(t *testing.T) {
	t.Parallel()
	const body = `{"default_archetype_id":"iphone17_ios18_7_safari26_4","data":[
		{"id":"iphone17_ios18_7_safari26_4","display_label":"iPhone 17 / iOS 18.7 / Safari 26.4","device":"iPhone 17","ios_version":"18.7","safari_version":"26.4","browser":"safari","browser_version":"26.4","canvas_family":"B","status":"launch","is_default":true},
		{"id":"iphone17_ios18_7_chrome149","display_label":"iPhone 17 / iOS 18.7 / Chrome 149","device":"iPhone 17","ios_version":"18.7","safari_version":"26.6","browser":"chrome","browser_version":"149","status":"available","is_default":false},
		{"id":"iphone13_ios18_7_safari26_5","display_label":"iPhone 13 / iOS 18.7 / Safari 26.5","device":"iPhone 13","ios_version":"18.7","safari_version":"26.5","status":"available","is_default":false},
		{"id":"iphone17_ios18_7_firefox140","display_label":"iPhone 17 / iOS 18.7 / Firefox 140","device":"iPhone 17","ios_version":"18.7","safari_version":"26.6","browser":"firefox","browser_version":"140","status":"available","is_default":false},
		{"id":"iphone17_ios18_7_x1","display_label":"iPhone 17 / iOS 18.7 / X 1","device":"iPhone 17","ios_version":"18.7","safari_version":"26.6","browser":"other","status":"available","is_default":false}
	]}`
	_, client := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	got, err := client.Archetypes.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ browser, version, safari string }{
		{"safari", "26.4", "26.4"},
		{"chrome", "149", "26.6"}, // the Safari release it is built on, never 149
		{"", "", "26.5"},          // an older server: no browser fields
		{"firefox", "140", "26.6"},
		{"other", "", "26.6"},
	}
	if len(got.Data) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got.Data), len(want))
	}
	for i, w := range want {
		e := got.Data[i]
		if e.Browser != w.browser || e.BrowserVersion != w.version || e.SafariVersion != w.safari {
			t.Errorf("entry %d (%s): browser=%q version=%q safari=%q, want %q %q %q",
				i, e.ID, e.Browser, e.BrowserVersion, e.SafariVersion, w.browser, w.version, w.safari)
		}
	}

	// An entry without the fields is written without them (omitempty).
	out, err := json.Marshal(got.Data[2])
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(out, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"browser", "browser_version"} {
		if _, ok := keys[k]; ok {
			t.Errorf("an entry with no %s was written with one: %s", k, out)
		}
	}
}
