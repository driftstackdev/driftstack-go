package driftstack

import "context"

// PublicArchetype is one customer-selectable device/iOS/browser combination.
type PublicArchetype struct {
	ID           string `json:"id"`
	DisplayLabel string `json:"display_label"`
	Device       string `json:"device"`
	IOSVersion   string `json:"ios_version"`
	// SafariVersion is always a Safari release. On a Chrome entry it is the
	// Safari release Chrome for iPhone is built on, never the Chrome version:
	// that is BrowserVersion.
	SafariVersion string `json:"safari_version"`
	// Browser is the browser the entry runs: currently "safari" or "chrome"
	// (Chrome for iPhone); "other" is a browser the server cannot name yet. An
	// open string: treat a value you do not know as another browser. Empty from
	// a server older than the field; such an entry is Safari.
	Browser string `json:"browser,omitempty"`
	// BrowserVersion is that browser's version: SafariVersion on a Safari
	// entry, the Chrome version ("149") on a Chrome entry. Empty from an older
	// server and on an "other" entry.
	BrowserVersion string `json:"browser_version,omitempty"`
	Status         string `json:"status"`
	IsDefault      bool   `json:"is_default"`
}

// ListArchetypesResponse contains the current default and complete public catalog.
type ListArchetypesResponse struct {
	DefaultArchetypeID string            `json:"default_archetype_id"`
	Data               []PublicArchetype `json:"data"`
}

// ArchetypesResource handles the public server-authoritative archetype catalog.
type ArchetypesResource struct {
	client *Client
}

// List returns every customer-selectable archetype and the current default.
func (r *ArchetypesResource) List(ctx context.Context) (*ListArchetypesResponse, error) {
	var out ListArchetypesResponse
	if err := r.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/v1/archetypes",
		out:    &out,
	}); err != nil {
		return nil, err
	}
	return &out, nil
}
