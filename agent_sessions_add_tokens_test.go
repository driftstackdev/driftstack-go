package driftstack

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// POST /v1/agent-sessions/:id/budget (B-043) — the request the SDK puts on the
// wire, the session it answers, and the over-the-cap refusal.
func TestAgentSessions_AddTokens_PostsExactlyAddTokens(t *testing.T) {
	t.Parallel()
	var method, path, body string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.EscapedPath()
		body = decodeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"agt_1","status":"active","token_budget_total":750000,"token_budget_remaining":653000,"transcript_length":4}`))
	})
	out, err := client.AgentSessions.AddTokens(context.Background(), "agt_1", 650000)
	if err != nil {
		t.Fatal(err)
	}
	if method != "POST" || path != "/v1/agent-sessions/agt_1/budget" {
		t.Errorf("request = %s %s, want POST /v1/agent-sessions/agt_1/budget", method, path)
	}
	// The server's body schema is strict: exactly add_tokens.
	if body != `{"add_tokens":650000}` {
		t.Errorf("body = %q, want {\"add_tokens\":650000}", body)
	}
	if out.TokenBudgetTotal != 750000 || out.TokenBudgetRemaining != 653000 {
		t.Errorf("unexpected session: %+v", out)
	}
}

func TestAgentSessions_AddTokens_PastTheCapIsAConflictWithItsCode(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"https://errors.driftstack.dev/conflict","title":"Conflict","status":409,"code":"token_budget_cap_exceeded","max_add_tokens":10000}`))
	})
	_, err := client.AgentSessions.AddTokens(context.Background(), "agt_1", 20000)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want *ConflictError", err)
	}
	if conflict.Problem["code"] != "token_budget_cap_exceeded" {
		t.Errorf("code = %v, want token_budget_cap_exceeded", conflict.Problem["code"])
	}
}
