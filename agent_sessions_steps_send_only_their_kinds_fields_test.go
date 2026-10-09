package driftstack

// AgentIntent is one flat struct for every kind of step. The server drops a
// field a step's kind does not take today, and a later release will refuse it
// (a 400 naming steps.<i>.<field>). Every field but Kind is omitted when it is not set, so a
// step sends only what the program set: each kind's step below, built the way
// a program builds it, encodes no field outside that kind's list. The lists
// mirror the server's (agent-steps-contract.ts CUSTOMER_STEP_FIELDS, pinned
// by a-step-field-its-kind-does-not-take-is-a-400-only-with-the-refusal-switch-on.test.ts).

import (
	"encoding/json"
	"testing"
)

var stepFieldsByKind = map[string][]string{
	"navigate":         {"kind", "url"},
	"interact":         {"kind", "action", "selector", "value", "value_omitted", "sensitive", "frame"},
	"wait":             {"kind", "condition", "selector", "timeoutMs", "frame"},
	"capture":          {"kind", "capture", "frame"},
	"scroll":           {"kind", "direction", "amount_px"},
	"behavioral_pause": {"kind", "duration_ms", "reading_word_count"},
	"back":             {"kind"},
	"extract":          {"kind", "selector", "body", "attribute", "property", "all", "index", "frame"},
	"tap_at":           {"kind", "x", "y"},
	"dialog":           {"kind", "action", "text"},
}

func TestEveryKindOfStepSendsOnlyItsOwnFields(t *testing.T) {
	t.Parallel()
	n := func(v int) *int { return &v }
	text := "yes"
	steps := []AgentIntent{
		{Kind: "navigate", URL: "https://shop.example.com/"},
		{Kind: "interact", Action: "type", Selector: "#q", Value: "lamp", Sensitive: true, Frame: []int{0}},
		{Kind: "interact", Action: "tap", Selector: "#go", FramePath: []FrameLevel{FrameBySrc("pay.example")}},
		{Kind: "wait", Condition: "selector_visible", Selector: "#r", TimeoutMs: n(5000), Frame: []int{0}},
		{Kind: "capture", Capture: "dom_snapshot", Frame: []int{1}},
		{Kind: "scroll", Direction: "down", AmountPx: n(400)},
		{Kind: "behavioral_pause", DurationMs: n(500), ReadingWordCount: n(20)},
		{Kind: "back"},
		{Kind: "extract", Selector: "li a", Attribute: "href", All: true, Frame: []int{0}},
		{Kind: "extract", Selector: "li", Index: n(0)},
		{Kind: "extract", Selector: "#pw", Property: "value"},
		{Kind: "extract", Body: true},
		{Kind: "tap_at", X: n(10), Y: n(0)},
		{Kind: "dialog", Action: "accept", Text: &text},
	}
	for _, step := range steps {
		buf, err := json.Marshal(step)
		if err != nil {
			t.Fatalf("%s: %v", step.Kind, err)
		}
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(buf, &keys); err != nil {
			t.Fatal(err)
		}
		allowed := map[string]bool{}
		for _, f := range stepFieldsByKind[step.Kind] {
			allowed[f] = true
		}
		for key := range keys {
			if !allowed[key] {
				t.Errorf("a %s step sends %q, which its kind does not take: %s", step.Kind, key, buf)
			}
		}
	}
}
