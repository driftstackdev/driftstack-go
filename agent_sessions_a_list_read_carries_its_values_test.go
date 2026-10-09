package driftstack

// An "extract" step with All reads every element its selector matches, and
// the result carries them in Values: each match's text, or its Attribute (nil
// for an element without it). A replay of an Idempotency-Key carries none,
// since the values are never stored.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAListReadCarriesItsValuesAndAMissingAttributeIsNil(t *testing.T) {
	t.Parallel()
	raw := `{"kind":"plan-executed","session":{},"ok":true,
	  "intents":[],
	  "results":[
	    {"kind":"success","intent":{"kind":"extract","selector":"li.result","all":true},"summary":"extracted text from 2 elements matching li.result, returned in this step’s values, never stored","values":["Alpha","Beta"],"truncated":false},
	    {"kind":"success","intent":{"kind":"extract","selector":"li.result a","attribute":"href","all":true},"summary":"extracted attribute href from 2 elements matching li.result a, returned in this step’s values, never stored","values":["https://shop.example.com/a",null],"truncated":false},
	    {"kind":"success","intent":{"kind":"extract","selector":"li.result","all":true},"summary":"extracted text from 2 elements matching li.result, returned in this step’s values, never stored"}
	  ]}`
	var resp AgentMessageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	results, err := resp.ParsedResults()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("len=%d", len(results))
	}
	if !results[0].Intent.All {
		t.Errorf("intent=%+v", results[0].Intent)
	}
	if len(results[0].Values) != 2 || *results[0].Values[0] != "Alpha" || *results[0].Values[1] != "Beta" {
		t.Errorf("values=%v", results[0].Values)
	}
	if results[0].Truncated == nil || *results[0].Truncated {
		t.Errorf("truncated=%v", results[0].Truncated)
	}
	if results[1].Values[1] != nil {
		t.Errorf("an element without the attribute is not nil: %v", results[1].Values[1])
	}
	// A replay carries no values at all.
	if results[2].Values != nil || results[2].Truncated != nil {
		t.Errorf("replay values=%v truncated=%v", results[2].Values, results[2].Truncated)
	}
	// The step is written back as it came: all, and no empty values.
	buf, err := json.Marshal(results[2])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(buf), `"values"`) {
		t.Errorf("a result without values marshals them: %s", buf)
	}
	if !strings.Contains(string(buf), `"all":true`) {
		t.Errorf("intent written back without all: %s", buf)
	}
	// A step that reads one match sends no all.
	one, err := json.Marshal(AgentIntent{Kind: "extract", Selector: "li.result"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(one), `"all"`) {
		t.Errorf("a one-match read sends all: %s", one)
	}
}

// An "extract" step with Index reads one match by its position; an Index of 0
// is sent (it is a position, not an absence), and nil sends none.
func TestAnIndexReadSendsItsPositionAndZeroIsAPosition(t *testing.T) {
	t.Parallel()
	zero := 0
	first, err := json.Marshal(AgentIntent{Kind: "extract", Selector: "li.result", Index: &zero})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), `"index":0`) {
		t.Errorf("index 0 not sent: %s", first)
	}
	none, err := json.Marshal(AgentIntent{Kind: "extract", Selector: "li.result"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(none), `"index"`) {
		t.Errorf("a read with no index sends one: %s", none)
	}
	var back AgentIntent
	if err := json.Unmarshal([]byte(`{"kind":"extract","selector":"li","index":2}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.Index == nil || *back.Index != 2 {
		t.Errorf("index=%v", back.Index)
	}
}
