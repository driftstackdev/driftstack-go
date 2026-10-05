package driftstack

import (
	"bytes"
	"encoding/json"
	"errors"
)

// FrameLevel is one level of an embedded frame's path: a position among the
// frames of the document that holds it (Index), or — when Selector is not
// empty — a frame placed inside a shadow root, which has no position and is
// named by the selector of its iframe element in that document (" >>> "
// steps into a shadow root; at most 1024 characters). ListFrames lists
// numbered frames only for now. It is encoded as a JSON number or a JSON
// string.
//
// In a step you send with RunSteps a level may instead find its frame when the
// step runs, by what it is rather than by its number (a number goes stale when
// the page's scripts add or remove frames): Src, the frame whose address or
// whose iframe's src attribute CONTAINS this text (1 to 512 characters), or
// Name, the frame whose iframe's name attribute IS this text (1 to 256
// characters) — see FrameBySrc and FrameByName. Encoded as {"src": …} or
// {"name": …}. Exactly one frame must match, or the step fails with nothing
// done; only a session whose browser can find a frame this way runs it. At
// most one of Selector, Src and Name is set.
type FrameLevel struct {
	Index    int
	Selector string
	Src      string
	Name     string
}

// FrameAt is the level at position index.
func FrameAt(index int) FrameLevel { return FrameLevel{Index: index} }

// FrameBySelector is the level of a frame inside a shadow root, by the
// selector of its iframe element. An empty selector names no frame: a step
// carrying one is an encoding error, never position 0.
func FrameBySelector(selector string) FrameLevel {
	if selector == "" {
		return FrameLevel{Index: -1}
	}
	return FrameLevel{Selector: selector}
}

// FrameBySrc is the level of the frame whose address, or whose iframe's src
// attribute, contains text (letter case included), found when the step runs.
// Empty text names no frame: a step carrying it is an encoding error, never
// position 0.
func FrameBySrc(text string) FrameLevel {
	if text == "" {
		return FrameLevel{Index: -1}
	}
	return FrameLevel{Src: text}
}

// FrameByName is the level of the frame whose iframe's name attribute is
// exactly name, found when the step runs. An empty name names no frame: a step
// carrying it is an encoding error, never position 0.
func FrameByName(name string) FrameLevel {
	if name == "" {
		return FrameLevel{Index: -1}
	}
	return FrameLevel{Name: name}
}

// IsSelector reports whether the level names a frame by its selector.
func (l FrameLevel) IsSelector() bool { return l.Selector != "" }

// IsQuery reports whether the level finds its frame by its address (Src) or
// its name (Name) when the step runs.
func (l FrameLevel) IsQuery() bool { return l.Src != "" || l.Name != "" }

// isPosition reports whether the level is a position: no selector and no query.
func (l FrameLevel) isPosition() bool { return !l.IsSelector() && !l.IsQuery() }

// MarshalJSON encodes the level as a string when it names a selector, as
// {"src": …} or {"name": …} when it is a query, and as a number otherwise. A
// negative position (which is also what FrameBySelector(""), FrameBySrc("")
// and FrameByName("") build) is an error: no frame has one, and so is a level
// setting more than one of Selector, Src and Name.
func (l FrameLevel) MarshalJSON() ([]byte, error) {
	set := 0
	for _, text := range []string{l.Selector, l.Src, l.Name} {
		if text != "" {
			set++
		}
	}
	if set > 1 {
		return nil, errors.New("driftstack: a frame path level sets more than one of Selector, Src and Name")
	}
	switch {
	case l.Selector != "":
		return json.Marshal(l.Selector)
	case l.Src != "":
		return json.Marshal(struct {
			Src string `json:"src"`
		}{l.Src})
	case l.Name != "":
		return json.Marshal(struct {
			Name string `json:"name"`
		}{l.Name})
	}
	if l.Index < 0 {
		return nil, errors.New("driftstack: a frame path level is a negative position or an empty selector, src or name")
	}
	return json.Marshal(l.Index)
}

// UnmarshalJSON reads a number, a non-empty string, or an object with exactly
// one of "src" and "name", non-empty.
func (l *FrameLevel) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var selector string
		if err := json.Unmarshal(trimmed, &selector); err != nil {
			return err
		}
		if selector == "" {
			return errors.New("driftstack: a frame path level is an empty string")
		}
		*l = FrameLevel{Selector: selector}
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var query map[string]string
		if err := json.Unmarshal(trimmed, &query); err != nil {
			return err
		}
		src, hasSrc := query["src"]
		name, hasName := query["name"]
		switch {
		case len(query) == 1 && hasSrc && src != "":
			*l = FrameLevel{Src: src}
		case len(query) == 1 && hasName && name != "":
			*l = FrameLevel{Name: name}
		default:
			return errors.New(`driftstack: a frame path level object has exactly one of "src" and "name", not empty`)
		}
		return nil
	}
	var index int
	if err := json.Unmarshal(trimmed, &index); err != nil {
		return err
	}
	*l = FrameLevel{Index: index}
	return nil
}

// agentIntentFields is AgentIntent without its JSON methods.
type agentIntentFields AgentIntent

// MarshalJSON encodes FramePath, when it is set, as the step's "frame"; a
// step with only Frame (or neither) is encoded exactly as before FramePath
// existed. Frame and FramePath both set to different paths is an error: a
// step read from an answer holds one of them, and an edit of the other is
// never silently dropped.
func (i AgentIntent) MarshalJSON() ([]byte, error) {
	if len(i.FramePath) == 0 {
		return json.Marshal(agentIntentFields(i))
	}
	if len(i.Frame) > 0 && !samePath(i.Frame, i.FramePath) {
		return nil, errors.New("driftstack: AgentIntent sets Frame and FramePath to different paths; set one of them")
	}
	fields := agentIntentFields(i)
	fields.Frame = nil
	return json.Marshal(struct {
		agentIntentFields
		Frame []FrameLevel `json:"frame"`
	}{fields, i.FramePath})
}

// samePath reports whether path names exactly the positions in positions.
func samePath(positions []int, path []FrameLevel) bool {
	if len(positions) != len(path) {
		return false
	}
	for n, level := range path {
		if !level.isPosition() || level.Index != positions[n] {
			return false
		}
	}
	return true
}

// UnmarshalJSON reads "frame" into Frame when every level is a position —
// exactly as before FramePath existed — and into FramePath, level by level,
// when a level is a selector or a query; the other is left empty. As encoding/json does
// for any field, a step that names no frame leaves both as they were, and
// "frame": null empties both.
func (i *AgentIntent) UnmarshalJSON(data []byte) error {
	var wire struct {
		*agentIntentFields
		Frame json.RawMessage `json:"frame"`
	}
	wire.agentIntentFields = (*agentIntentFields)(i)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Frame == nil {
		return nil
	}
	var levels []FrameLevel
	if err := json.Unmarshal(wire.Frame, &levels); err != nil {
		return err
	}
	i.Frame, i.FramePath = nil, nil
	if levels == nil {
		return nil
	}
	positions := make([]int, 0, len(levels))
	for _, level := range levels {
		if !level.isPosition() {
			i.FramePath = levels
			return nil
		}
		positions = append(positions, level.Index)
	}
	i.Frame = positions
	return nil
}
