package driftstack

// A "dialog" step answers the page's open JavaScript dialog: its result says
// whether one was answered, which kind and how, and never echoes the text typed
// into a prompt. A step with no dialog open succeeds with Handled false. Text
// is a pointer, so an empty answer to a prompt is sent and no text is not.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestADialogStepIsReadWithWhatItAnsweredAndNeverItsText(t *testing.T) {
	t.Parallel()
	raw := `{"kind":"plan-executed","session":{},"ok":true,
	  "intents":[],
	  "results":[
	    {"kind":"success","intent":{"kind":"dialog","action":"accept","text":"Ada"},"summary":"answered the prompt dialog: OK","dialog":{"handled":true,"kind":"prompt","action":"accept"}},
	    {"kind":"success","intent":{"kind":"dialog","action":"dismiss"},"summary":"no dialog was open, so nothing was answered","dialog":{"handled":false,"action":"dismiss"}}
	  ]}`
	var resp AgentMessageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	results, err := resp.ParsedResults()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("len=%d", len(results))
	}
	first := results[0]
	if first.Intent.Kind != "dialog" || first.Intent.Action != "accept" || first.Intent.Text == nil || *first.Intent.Text != "Ada" {
		t.Errorf("intent=%+v", first.Intent)
	}
	if first.Dialog == nil || !first.Dialog.Handled || first.Dialog.Kind != "prompt" || first.Dialog.Action != "accept" {
		t.Errorf("dialog=%+v", first.Dialog)
	}
	second := results[1]
	if second.Dialog == nil || second.Dialog.Handled || second.Dialog.Kind != "" || second.Dialog.Action != "dismiss" {
		t.Errorf("dialog=%+v", second.Dialog)
	}
	if second.Intent.Text != nil {
		t.Errorf("text=%v", *second.Intent.Text)
	}
	// Written back, a dialog result without a kind carries none, and the
	// answer is never re-encoded as text.
	buf, err := json.Marshal(second.Dialog)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf); got != `{"handled":false,"action":"dismiss"}` {
		t.Errorf("dialog=%s", got)
	}
}

func TestADialogStepSendsAnEmptyAnswerAndOmitsNoAnswer(t *testing.T) {
	t.Parallel()
	empty := ""
	buf, err := json.Marshal(AgentIntent{Kind: "dialog", Action: "accept", Text: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf); got != `{"kind":"dialog","action":"accept","text":""}` {
		t.Errorf("empty answer=%s", got)
	}
	buf, err = json.Marshal(AgentIntent{Kind: "dialog", Action: "dismiss"})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf); strings.Contains(got, "text") || got != `{"kind":"dialog","action":"dismiss"}` {
		t.Errorf("no answer=%s", got)
	}
}
