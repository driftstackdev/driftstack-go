package driftstack

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// POST /steps — RunSteps puts the steps on the wire as written, on the same
// stream a message is answered on, and an empty type step still carries its
// value (an empty value is a clear, an absent one is not a step at all).
func TestAgentSessions_RunSteps_PostsTheStepsAndSendsAnEmptyTypeValue(t *testing.T) {
	t.Parallel()
	var method, path, accept, key string
	var body map[string]any
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.EscapedPath()
		accept, key = r.Header.Get("Accept"), r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		writeAgentMessageSSE(t, w, 200, map[string]any{
			"kind":    "plan-executed",
			"session": map[string]any{"id": "agt 1", "status": "active"},
			"intents": []any{},
			"results": []any{},
			"ok":      true,
		})
	})
	steps := []AgentIntent{
		{Kind: "navigate", URL: "https://example.com/"},
		{Kind: "interact", Action: "type", Selector: "#q", Value: ""},
		{Kind: "interact", Action: "type", Selector: "#q", Value: "shoes"},
		{Kind: "capture", Capture: "screenshot"},
	}
	out, err := client.AgentSessions.RunSteps(context.Background(), "agt 1", steps, &RunStepsOptions{
		IdempotencyKey:              "steps-1",
		ApproveConsequentialActions: []ConsequentialActionApproval{{Category: "purchase", MatchedText: "Buy now"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Kind != "plan-executed" || !out.OK {
		t.Errorf("kind/ok = %q/%v", out.Kind, out.OK)
	}
	if method != "POST" || path != "/v1/agent-sessions/agt%201/steps" {
		t.Errorf("request = %s %s", method, path)
	}
	if accept != "text/event-stream" || key != "steps-1" {
		t.Errorf("accept/key = %q/%q", accept, key)
	}
	wire, _ := body["steps"].([]any)
	if len(wire) != 4 {
		t.Fatalf("steps = %v", body["steps"])
	}
	clear, _ := wire[1].(map[string]any)
	if v, ok := clear["value"]; !ok || v != "" {
		t.Errorf("an empty type step must carry value \"\": %v", clear)
	}
	nav, _ := wire[0].(map[string]any)
	if _, ok := nav["value"]; ok {
		t.Errorf("a navigate step carries no value: %v", nav)
	}
	approvals, _ := body["approve_consequential_actions"].([]any)
	if len(approvals) != 1 {
		t.Errorf("approvals = %v", body["approve_consequential_actions"])
	}
}

func TestAgentSessions_RunSteps_NilOptionsSendsOnlyTheSteps(t *testing.T) {
	t.Parallel()
	var body map[string]any
	var key string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeAgentMessageSSE(t, w, 200, map[string]any{"kind": "plan-executed", "ok": false})
	})
	if _, err := client.AgentSessions.RunSteps(context.Background(), "agt_1", []AgentIntent{{Kind: "back"}}, nil); err != nil {
		t.Fatal(err)
	}
	if key != "" {
		t.Errorf("no idempotency key was given, got %q", key)
	}
	if _, ok := body["approve_consequential_actions"]; ok || len(body) != 1 {
		t.Errorf("body = %v", body)
	}
}

// An extract that reads an attribute is a step a list can send, and one the
// answer can describe: Attribute goes on the wire and comes back decoded.
func TestAgentSessions_RunSteps_AnExtractCarriesItsAttribute(t *testing.T) {
	t.Parallel()
	var body map[string]any
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeAgentMessageSSE(t, w, 200, map[string]any{
			"kind": "plan-executed",
			"intents": []any{
				map[string]any{"kind": "extract", "selector": "a.next", "attribute": "href"},
			},
			"results": []any{},
			"ok":      true,
		})
	})
	step := AgentIntent{Kind: "extract", Selector: "a.next", Attribute: "href"}
	out, err := client.AgentSessions.RunSteps(context.Background(), "agt_1", []AgentIntent{step}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := body["steps"].([]any)
	sent, _ := wire[0].(map[string]any)
	if sent["attribute"] != "href" {
		t.Errorf("the step on the wire = %v", sent)
	}
	intents, err := out.ParsedIntents()
	if err != nil {
		t.Fatal(err)
	}
	if len(intents) != 1 || intents[0].Attribute != "href" {
		t.Errorf("the step in the answer = %+v", intents)
	}
}
