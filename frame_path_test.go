package driftstack

import (
	"encoding/json"
	"reflect"
	"testing"
)

// A frame path level is a position or, for a frame placed inside a shadow
// root, the selector of its iframe. Frame ([]int) keeps every program that
// sets or reads positions compiling and encoding exactly as before; FramePath
// carries a path with a selector level.

func TestFramePath_APositionsOnlyStepIsEncodedExactlyAsBefore(t *testing.T) {
	t.Parallel()
	step := AgentIntent{Kind: "extract", Body: true, Frame: []int{0, 2}}
	got, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	// The encoding of the plain fields, with no JSON methods involved.
	want, err := json.Marshal(agentIntentFields(step))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) || string(got) != `{"kind":"extract","frame":[0,2],"body":true}` {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestFramePath_ASelectorLevelIsSentAsAString(t *testing.T) {
	t.Parallel()
	step := AgentIntent{
		Kind:      "interact",
		Action:    "type",
		Selector:  "#card",
		Value:     "4242",
		FramePath: []FrameLevel{FrameAt(0), FrameBySelector("x-pay >>> iframe")},
	}
	got, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(got, &wire); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wire["frame"], []any{float64(0), "x-pay >>> iframe"}) {
		t.Fatalf("frame = %#v (%s)", wire["frame"], got)
	}
	if wire["kind"] != "interact" || wire["selector"] != "#card" || wire["value"] != "4242" {
		t.Fatalf("other fields lost: %s", got)
	}
	// Frame set as well, to another path, is an error: never one of the two
	// chosen in silence.
	step.Frame = []int{9}
	if again, err := json.Marshal(step); err == nil {
		t.Fatalf("got %s", again)
	}
}

func TestFramePath_AnAnswerIsReadLevelByLevel(t *testing.T) {
	t.Parallel()
	var shadow AgentIntent
	if err := json.Unmarshal([]byte(`{"kind":"capture","capture":"dom_snapshot","frame":[0,"x-pay >>> iframe:nth-of-type(2)"]}`), &shadow); err != nil {
		t.Fatal(err)
	}
	want := []FrameLevel{FrameAt(0), FrameBySelector("x-pay >>> iframe:nth-of-type(2)")}
	if !reflect.DeepEqual(shadow.FramePath, want) || shadow.Frame != nil {
		t.Fatalf("FramePath=%#v Frame=%#v", shadow.FramePath, shadow.Frame)
	}
	if shadow.Kind != "capture" || shadow.Capture != "dom_snapshot" {
		t.Fatalf("other fields lost: %#v", shadow)
	}
	if !shadow.FramePath[1].IsSelector() || shadow.FramePath[0].IsSelector() {
		t.Fatal("IsSelector")
	}
	var positions AgentIntent
	if err := json.Unmarshal([]byte(`{"kind":"extract","body":true,"frame":[1,0]}`), &positions); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(positions.Frame, []int{1, 0}) || positions.FramePath != nil {
		t.Fatalf("Frame=%#v FramePath=%#v", positions.Frame, positions.FramePath)
	}
	var none AgentIntent
	if err := json.Unmarshal([]byte(`{"kind":"back"}`), &none); err != nil {
		t.Fatal(err)
	}
	if none.Frame != nil || none.FramePath != nil {
		t.Fatalf("%#v", none)
	}
	// Round trip: what was read is what is sent.
	out, err := json.Marshal(shadow)
	if err != nil {
		t.Fatal(err)
	}
	var back AgentIntent
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, shadow) {
		t.Fatalf("round trip: %#v != %#v", back, shadow)
	}
	// A level that is neither is refused, as it was.
	var bad AgentIntent
	if err := json.Unmarshal([]byte(`{"kind":"extract","frame":[true]}`), &bad); err == nil {
		t.Fatal("a boolean level was accepted")
	}
	if err := json.Unmarshal([]byte(`{"kind":"extract","frame":[""]}`), &bad); err == nil {
		t.Fatal("an empty selector level was accepted")
	}
}

func TestFramePath_AResultHoldingAnIntentReadsIt(t *testing.T) {
	t.Parallel()
	var res AgentIntentResult
	raw := `{"kind":"failure","intent":{"kind":"extract","body":true,"frame":["checkout-form >>> iframe"]},"reason":"not done"}`
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Intent.FramePath, []FrameLevel{FrameBySelector("checkout-form >>> iframe")}) {
		t.Fatalf("%#v", res.Intent)
	}
}

// Review of the shadow-frame path (2026-10-04): a step read from an answer
// and edited is sent as edited; an empty selector is never sent as position
// 0; decoding keeps what the answer does not name, as encoding/json does.

func TestFramePath_AStepReadFromAnAnswerIsSentAsEdited(t *testing.T) {
	t.Parallel()
	var step AgentIntent
	if err := json.Unmarshal([]byte(`{"kind":"extract","body":true,"frame":[1,0]}`), &step); err != nil {
		t.Fatal(err)
	}
	// Read exactly as before FramePath existed.
	if !reflect.DeepEqual(step, AgentIntent{Kind: "extract", Body: true, Frame: []int{1, 0}}) {
		t.Fatalf("%#v", step)
	}
	step.Frame = []int{2}
	got, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"kind":"extract","frame":[2],"body":true}` {
		t.Fatalf("edited Frame: got %s", got)
	}
	step.Frame = nil
	got, err = json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"kind":"extract","body":true}` {
		t.Fatalf("cleared Frame: got %s", got)
	}
	// A step whose path has a selector level is read into FramePath only;
	// setting Frame on it as well, to another path, is an error, never a
	// silent choice of one.
	var shadow AgentIntent
	if err := json.Unmarshal([]byte(`{"kind":"extract","body":true,"frame":["x-pay >>> iframe"]}`), &shadow); err != nil {
		t.Fatal(err)
	}
	if shadow.Frame != nil {
		t.Fatalf("Frame = %#v", shadow.Frame)
	}
	shadow.Frame = []int{2}
	if out, err := json.Marshal(shadow); err == nil {
		t.Fatalf("both set to different paths was sent: %s", out)
	}
	// The same path in both is sent once.
	same := AgentIntent{Kind: "extract", Body: true, Frame: []int{3}, FramePath: []FrameLevel{FrameAt(3)}}
	out, err := json.Marshal(same)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"kind":"extract","body":true,"frame":[3]}` {
		t.Fatalf("got %s", out)
	}
}

func TestFramePath_AnEmptySelectorIsNeverSentAsPositionZero(t *testing.T) {
	t.Parallel()
	for _, path := range [][]FrameLevel{
		{FrameBySelector("")},
		{FrameAt(0), FrameBySelector("")},
		{FrameAt(-1)},
	} {
		step := AgentIntent{Kind: "extract", Body: true, FramePath: path}
		if out, err := json.Marshal(step); err == nil {
			t.Fatalf("%#v was sent as %s", path, out)
		}
	}
	// Positive control.
	out, err := json.Marshal(AgentIntent{Kind: "extract", Body: true, FramePath: []FrameLevel{FrameAt(0), FrameBySelector("a >>> iframe")}})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(out, &wire); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wire["frame"], []any{float64(0), "a >>> iframe"}) {
		t.Fatalf("%s", out)
	}
}

func TestFramePath_DecodingKeepsWhatTheAnswerDoesNotName(t *testing.T) {
	t.Parallel()
	step := AgentIntent{Selector: "#keep", Frame: []int{4}}
	if err := json.Unmarshal([]byte(`{"kind":"extract"}`), &step); err != nil {
		t.Fatal(err)
	}
	if step.Kind != "extract" || step.Selector != "#keep" || !reflect.DeepEqual(step.Frame, []int{4}) {
		t.Fatalf("%#v", step)
	}
	// A frame the answer does name replaces both forms.
	if err := json.Unmarshal([]byte(`{"frame":["a >>> iframe"]}`), &step); err != nil {
		t.Fatal(err)
	}
	if step.Frame != nil || !reflect.DeepEqual(step.FramePath, []FrameLevel{FrameBySelector("a >>> iframe")}) {
		t.Fatalf("%#v", step)
	}
	if err := json.Unmarshal([]byte(`{"frame":[1]}`), &step); err != nil {
		t.Fatal(err)
	}
	if step.FramePath != nil || !reflect.DeepEqual(step.Frame, []int{1}) {
		t.Fatalf("%#v", step)
	}
	if err := json.Unmarshal([]byte(`{"frame":null}`), &step); err != nil {
		t.Fatal(err)
	}
	if step.FramePath != nil || step.Frame != nil {
		t.Fatalf("%#v", step)
	}
}

// 2026-10-04 — a level may find its frame by its address or name when the step
// runs: encoded as {"src": …} / {"name": …}, read back into FramePath.

func TestFramePath_AQueryLevelIsSentAsAnObjectWithOneKey(t *testing.T) {
	t.Parallel()
	step := AgentIntent{
		Kind:      "interact",
		Action:    "type",
		Selector:  "#card",
		Value:     "4242",
		FramePath: []FrameLevel{FrameAt(0), FrameBySrc("js.example-pay.com"), FrameByName("card-number")},
	}
	got, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(got, &wire); err != nil {
		t.Fatal(err)
	}
	want := []any{float64(0), map[string]any{"src": "js.example-pay.com"}, map[string]any{"name": "card-number"}}
	if !reflect.DeepEqual(wire["frame"], want) {
		t.Fatalf("frame = %#v (%s)", wire["frame"], got)
	}
	// Read back level by level, into FramePath only.
	var back AgentIntent
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.FramePath, step.FramePath) || back.Frame != nil {
		t.Fatalf("FramePath=%#v Frame=%#v", back.FramePath, back.Frame)
	}
	if !back.FramePath[1].IsQuery() || !back.FramePath[2].IsQuery() || back.FramePath[0].IsQuery() {
		t.Fatal("IsQuery")
	}
}

func TestFramePath_AQueryLevelThatIsEmptyOrAmbiguousIsRefused(t *testing.T) {
	t.Parallel()
	for _, path := range [][]FrameLevel{
		{FrameBySrc("")},
		{FrameByName("")},
		{{Src: "a", Name: "b"}},
		{{Selector: "a >>> iframe", Src: "b"}},
	} {
		step := AgentIntent{Kind: "extract", Body: true, FramePath: path}
		if out, err := json.Marshal(step); err == nil {
			t.Fatalf("%#v was sent as %s", path, out)
		}
	}
	for _, raw := range []string{
		`{"kind":"extract","frame":[{}]}`,
		`{"kind":"extract","frame":[{"src":""}]}`,
		`{"kind":"extract","frame":[{"src":"a","name":"b"}]}`,
		`{"kind":"extract","frame":[{"href":"a"}]}`,
		`{"kind":"extract","frame":[{"src":1}]}`,
	} {
		var bad AgentIntent
		if err := json.Unmarshal([]byte(raw), &bad); err == nil {
			t.Fatalf("%s was read as %#v", raw, bad.FramePath)
		}
	}
}
