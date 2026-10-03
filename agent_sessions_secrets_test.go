package driftstack

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// Session secrets: the requests RegisterSecret, ListSecrets and DeleteSecret
// put on the wire, and the answers they decode.
func TestAgentSessions_RegisterSecret_PostsOnlyTheFieldsGiven(t *testing.T) {
	t.Parallel()
	var method, path, body string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.EscapedPath()
		body = decodeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(AgentSessionSecret{
			Handle: "sec_4be1c09d2f7a4e6b8c35d1a0f9e27b64", Label: "Card",
			Sites: []string{"shop.example.com"}, CreatedAt: "2026-10-02T12:00:00.000Z",
			ExpiresAt: "2026-10-02T13:00:00.000Z",
		})
	})
	out, err := client.AgentSessions.RegisterSecret(context.Background(), "agt 1", RegisterAgentSessionSecretRequest{
		Label: "Card", Secret: "4242424242424242", Sites: []string{"shop.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if method != "POST" || path != "/v1/agent-sessions/agt%201/secrets" {
		t.Errorf("request = %s %s, want POST /v1/agent-sessions/agt%%201/secrets", method, path)
	}
	// Unset optional fields stay off the wire, so the server's defaults apply.
	if body != `{"label":"Card","secret":"4242424242424242","sites":["shop.example.com"]}` {
		t.Errorf("body = %s", body)
	}
	if out.Handle != "sec_4be1c09d2f7a4e6b8c35d1a0f9e27b64" || out.ExpiresAt != "2026-10-02T13:00:00.000Z" {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestAgentSessions_RegisterSecret_SendsTheOptionalFieldsWhenSet(t *testing.T) {
	t.Parallel()
	var body string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"handle":"sec_4be1c09d2f7a4e6b8c35d1a0f9e27b64"}`))
	})
	yes, ttl := true, 600
	if _, err := client.AgentSessions.RegisterSecret(context.Background(), "agt_1", RegisterAgentSessionSecretRequest{
		Label: "Card", Secret: "v", Sites: []string{"shop.example.com"},
		IncludeSubdomains: &yes, TTLSeconds: &ttl,
	}); err != nil {
		t.Fatal(err)
	}
	if body != `{"label":"Card","secret":"v","sites":["shop.example.com"],"include_subdomains":true,"ttl_seconds":600}` {
		t.Errorf("body = %s", body)
	}
}

func TestAgentSessions_ListAndDeleteSecret(t *testing.T) {
	t.Parallel()
	var seen []string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.EscapedPath())
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"handle":"sec_4be1c09d2f7a4e6b8c35d1a0f9e27b64","label":"Card","sites":["shop.example.com"],"include_subdomains":false,"created_at":"2026-10-02T12:00:00.000Z","expires_at":"2026-10-02T13:00:00.000Z"}]}`))
	})
	list, err := client.AgentSessions.ListSecrets(context.Background(), "agt_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Label != "Card" {
		t.Errorf("unexpected list: %+v", list)
	}
	if err := client.AgentSessions.DeleteSecret(context.Background(), "agt_1", "sec_a/b"); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /v1/agent-sessions/agt_1/secrets", "DELETE /v1/agent-sessions/agt_1/secrets/sec_a%2Fb"}
	if len(seen) != 2 || seen[0] != want[0] || seen[1] != want[1] {
		t.Errorf("requests = %v, want %v", seen, want)
	}
}
