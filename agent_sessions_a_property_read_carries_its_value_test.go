package driftstack

// An "extract" step with a Property reads what an element holds NOW (a field's
// value, whether a checkbox is ticked) and the result carries it in Value: a
// string, a number, true or false, or null when the element has no such
// property — told apart from a step that carries no Value at all (every other
// step, and a replay of an Idempotency-Key, since the value is never stored).

import (
	"encoding/json"
	"testing"
)

func TestAPropertyReadCarriesItsValueAndANullIsNotAnAbsence(t *testing.T) {
	t.Parallel()
	raw := `{"kind":"plan-executed","session":{},"ok":true,
	  "intents":[],
	  "results":[
	    {"kind":"success","intent":{"kind":"extract","selector":"input[name=cardnumber]","property":"value","frame":[0]},"summary":"read property value of input[name=cardnumber] in embedded frame [0]: a string of 19 characters, returned in this step’s value, never stored","value":"4242 4242 4242 4242","value_truncated":false},
	    {"kind":"success","intent":{"kind":"extract","selector":"#terms","property":"checked"},"summary":"read property checked of #terms: a true/false value, returned in this step’s value, never stored","value":true,"value_truncated":false},
	    {"kind":"success","intent":{"kind":"extract","selector":"#x","property":"selectedIndex"},"summary":"read property selectedIndex of #x: the element has no selectedIndex property (value: null)","value":null,"value_truncated":false},
	    {"kind":"success","intent":{"kind":"extract","selector":"#x","property":"value"},"summary":"read property value of #x: a string of 4 characters, returned in this step’s value, never stored"}
	  ]}`
	var resp AgentMessageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	results, err := resp.ParsedResults()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("len=%d", len(results))
	}
	if results[0].Intent.Property != "value" || len(results[0].Intent.Frame) != 1 {
		t.Errorf("intent=%+v", results[0].Intent)
	}
	var card string
	if err := json.Unmarshal(results[0].Value, &card); err != nil || card != "4242 4242 4242 4242" {
		t.Errorf("card=%q err=%v", card, err)
	}
	if results[0].ValueTruncated == nil || *results[0].ValueTruncated {
		t.Errorf("truncated=%v", results[0].ValueTruncated)
	}
	var ticked bool
	if err := json.Unmarshal(results[1].Value, &ticked); err != nil || !ticked {
		t.Errorf("ticked=%v err=%v", ticked, err)
	}
	// An explicit null is the element answering "no such property"…
	if string(results[2].Value) != "null" {
		t.Errorf("null value=%q", results[2].Value)
	}
	// …and an absent one is a result that carries none (a replay).
	if results[3].Value != nil || results[3].ValueTruncated != nil {
		t.Errorf("absent value=%q truncated=%v", results[3].Value, results[3].ValueTruncated)
	}
	// The step is written back as it came: the property, and no empty value.
	buf, err := json.Marshal(results[3])
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatal(err)
	}
	if _, ok := back["value"]; ok {
		t.Errorf("a result without a value marshals one: %s", buf)
	}
	if intent, _ := back["intent"].(map[string]any); intent["property"] != "value" {
		t.Errorf("intent written back without its property: %s", buf)
	}
}

// A read of a field Driftstack typed a saved credential into reads back the
// placeholder, and says so in ShowsPlaceholder; every other result has none.
func TestAReadThatShowsAPlaceholderSaysSo(t *testing.T) {
	t.Parallel()
	raw := `{"kind":"plan-executed","session":{},"ok":true,
	  "intents":[],
	  "results":[
	    {"kind":"success","intent":{"kind":"extract","selector":"input[name=cardnumber]","property":"value","frame":[6]},"summary":"read property value of input[name=cardnumber] in embedded frame [6]: a string of 52 characters, returned in this step’s value, never stored","value":"{{credential:cred_0123456789abcdef0123456789abcdef}}","value_truncated":false,"shows_placeholder":true},
	    {"kind":"success","intent":{"kind":"extract","selector":"#name","property":"value"},"summary":"read property value of #name: a string of 3 characters, returned in this step’s value, never stored","value":"Ada","value_truncated":false}
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
	if results[0].ShowsPlaceholder == nil || !*results[0].ShowsPlaceholder {
		t.Errorf("shows_placeholder=%v", results[0].ShowsPlaceholder)
	}
	if results[1].ShowsPlaceholder != nil {
		t.Errorf("a read of a typed value has shows_placeholder=%v", *results[1].ShowsPlaceholder)
	}
	buf, err := json.Marshal(results[1])
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatal(err)
	}
	if _, ok := back["shows_placeholder"]; ok {
		t.Errorf("a result without the flag marshals one: %s", buf)
	}
}

// B-147 — a read whose text could not be checked in time says so in Withheld,
// and its Value is JSON null, not a note; every other result has no Withheld.
func TestAReadThatWasWithheldSaysSoAndHasANullValue(t *testing.T) {
	t.Parallel()
	raw := `{"kind":"plan-executed","session":{},"ok":true,
	  "intents":[],
	  "results":[
	    {"kind":"success","intent":{"kind":"extract","selector":"#name","property":"value"},"summary":"(not shown: this text could not be checked in time)","value":null,"value_truncated":false,"withheld":true},
	    {"kind":"success","intent":{"kind":"extract","selector":"#name","property":"value"},"summary":"read property value of #name: a string of 3 characters, returned in this step’s value, never stored","value":"Ada","value_truncated":false}
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
	if results[0].Withheld == nil || !*results[0].Withheld {
		t.Errorf("withheld=%v", results[0].Withheld)
	}
	if string(results[0].Value) != "null" {
		t.Errorf("a withheld read's value=%q, want null", string(results[0].Value))
	}
	if results[1].Withheld != nil {
		t.Errorf("a read that was checked has withheld=%v", *results[1].Withheld)
	}
	buf, err := json.Marshal(results[1])
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatal(err)
	}
	if _, ok := back["withheld"]; ok {
		t.Errorf("a result without the flag marshals one: %s", buf)
	}
}
