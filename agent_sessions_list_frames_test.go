package driftstack

import (
	"context"
	"net/http"
	"testing"
)

// ListFrames: the request it puts on the wire and the answer it decodes.
func TestAgentSessions_ListFrames_GetsTheFramesPathAndDecodesTheList(t *testing.T) {
	t.Parallel()
	var method, path string
	_, client := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"frames":[{"path":[0],"address":"pay.example.com/embed/card","url":"https://pay.example.com/embed/card?componentName=cardNumber&client_secret=REDACTED","url_masked":true,"url_truncated":false,"name":null,"displayed":true,"zero_size":false},{"path":[0,1],"address":null,"url":null,"url_masked":false,"url_truncated":false,"name":"inner","displayed":false,"zero_size":true}],"truncated":false}`))
	})
	out, err := client.AgentSessions.ListFrames(context.Background(), "agt 1")
	if err != nil {
		t.Fatal(err)
	}
	if method != "GET" || path != "/v1/agent-sessions/agt%201/frames" {
		t.Errorf("request = %s %s, want GET /v1/agent-sessions/agt%%201/frames", method, path)
	}
	if len(out.Frames) != 2 || out.Truncated {
		t.Fatalf("unexpected list: %+v", out)
	}
	first, second := out.Frames[0], out.Frames[1]
	if len(first.Path) != 1 || first.Path[0] != 0 || first.Address == nil ||
		*first.Address != "pay.example.com/embed/card" || first.Name != nil || !first.Displayed {
		t.Errorf("first frame = %+v", first)
	}
	if first.URL == nil ||
		*first.URL != "https://pay.example.com/embed/card?componentName=cardNumber&client_secret=REDACTED" ||
		!first.URLMasked || first.URLTruncated {
		t.Errorf("first frame url = %v masked=%v truncated=%v", first.URL, first.URLMasked, first.URLTruncated)
	}
	if second.URL != nil || second.URLMasked || second.URLTruncated {
		t.Errorf("second frame url = %v", second.URL)
	}
	if len(second.Path) != 2 || second.Path[1] != 1 || second.Address != nil ||
		second.Name == nil || *second.Name != "inner" || !second.ZeroSize {
		t.Errorf("second frame = %+v", second)
	}
}
