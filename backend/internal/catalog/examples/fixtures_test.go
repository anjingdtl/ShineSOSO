package examples_test

import (
	"embed"
	"errors"
	"path"
	"testing"

	"github.com/local/easysearch/backend/internal/catalog"
)

//go:embed *.yml
var fsys embed.FS

func TestExampleYAMLs_areValid(t *testing.T) {
	defs, errs := catalog.LoadDir(fsys, ".")
	for _, e := range errs {
		t.Errorf("loader error: %v", e)
	}
	if len(defs) != 2 {
		t.Fatalf("want 2 definitions, got %d: %v", len(defs), defs)
	}

	wantIDs := map[string]bool{
		"example-public-html": false,
		"example-torznab":     false,
	}
	for id, def := range defs {
		if _, ok := wantIDs[id]; !ok {
			t.Errorf("unexpected id %q", id)
			continue
		}
		wantIDs[id] = true
		if err := catalog.Validate(def); err != nil {
			t.Errorf("validate %s: %v", id, err)
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Errorf("missing fixture: %s.yml", id)
			_ = path.Join
		}
	}
}

func TestExampleYAMLs_rejectedForPrivateIP(t *testing.T) {
	// Reuse the HTML fixture's structure but rewrite one link to a
	// private IP. Should fail §13.8 quickHostSafetyCheck.
	const bad = `
schema: 1
id: bad-copy
name: Bad
version: 1.0.0
type: public
protocol: declarative
links:
  - https://127.0.0.1/
search:
  method: GET
  path: /search
  query:
    q: "{{ .Query.Keyword }}"
response:
  format: html
  rows:
    selector: "li"
  fields:
    title: { selector: "a", value: text, required: true }
`
	def, err := catalog.LoadDefinition([]byte(bad), "bad.yml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	err = catalog.Validate(def)
	var ve *catalog.ValidationError
	if !errors.As(err, &ve) || ve.Code != catalog.CodeLinkUnsafe {
		t.Fatalf("want LINK_UNSAFE, got %v", err)
	}
}
