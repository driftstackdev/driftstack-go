package driftstack

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// POST /v1/agent-sessions/:id/reopen — the request the SDK puts on the wire, the
// new session's link back to the ended one, and the past-the-window refusal.
func TestAgentSessions_Reopen_PostsAnEmptyBodyWithTheKeyHeaders(t *testing.T) {
	t.Parallel()
	var method, path, body, key, byok string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.EscapedPath()
		body = decodeBody(t, r)
		key, byok = r.Header.Get("Idempotency-Key"), r.Header.Get("x-byok-anthropic-api-key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"agt_new","status":"active","resumed_from":"agt_old","transcript_length":0}`))
	})
	out, err := client.AgentSessions.Reopen(context.Background(), "agt_old", &ReopenOptions{IdempotencyKey: "k-1"})
	if err != nil {
		t.Fatal(err)
	}
	if method != "POST" || path != "/v1/agent-sessions/agt_old/reopen" {
		t.Errorf("request = %s %s, want POST /v1/agent-sessions/agt_old/reopen", method, path)
	}
	// The server's body schema is strict: an empty object, never a stray field.
	if body != "{}" {
		t.Errorf("body = %q, want {}", body)
	}
	if key != "k-1" || byok != "" {
		t.Errorf("headers: Idempotency-Key %q, x-byok-anthropic-api-key %q", key, byok)
	}
	if out.ID != "agt_new" || out.ResumedFrom == nil || *out.ResumedFrom != "agt_old" {
		t.Errorf("unexpected session: %+v", out)
	}
}

func TestAgentSessions_Reopen_PastTheWindowIsASessionDestroyedError(t *testing.T) {
	t.Parallel()
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"type":"https://errors.driftstack.dev/session-destroyed","title":"Session destroyed","status":410,"code":"reopen_window_passed","reopen_window_days":7}`))
	})
	_, err := client.AgentSessions.Reopen(context.Background(), "agt_old", nil)
	var gone *SessionDestroyedError
	if !errors.As(err, &gone) {
		t.Fatalf("err = %v, want *SessionDestroyedError", err)
	}
	if gone.Problem["code"] != "reopen_window_passed" {
		t.Errorf("code = %v, want reopen_window_passed", gone.Problem["code"])
	}
}
