package catalog

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

const sampleYAML = `
schema: 1
id: example-public
name: Example Public
version: 1.0.0
description: Example public indexer
language: zh-CN
type: public
protocol: declarative

links:
  - https://example.com

categories:
  movie:
    - "1"

search:
  method: GET
  path: /search
  query:
    keyword: "{{ query.keyword }}"

response:
  format: html
  rows:
    selector: ".result-item"
  fields:
    title:
      selector: ".title"
      value: text
      required: true
    detail_url:
      selector: "a"
      value: attr
      attribute: href
      resolveUrl: true
`

func TestLoadDefinition_parsesValidYAML(t *testing.T) {
	def, err := LoadDefinition([]byte(sampleYAML), "example.yml")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if def.ID != "example-public" {
		t.Errorf("id: got %q want %q", def.ID, "example-public")
	}
	if def.Protocol != "declarative" {
		t.Errorf("protocol: got %q want %q", def.Protocol, "declarative")
	}
	if def.Search.Method != "GET" {
		t.Errorf("search.method: got %q want %q", def.Search.Method, "GET")
	}
	if def.Search.Query["keyword"] != "{{ query.keyword }}" {
		t.Errorf("search.query.keyword: got %q", def.Search.Query["keyword"])
	}
	if def.Result.Format != "html" {
		t.Errorf("result.format: got %q want html", def.Result.Format)
	}
	title, ok := def.Result.Fields["title"]
	if !ok || !title.Required {
		t.Errorf("fields.title.required missing; got %#v", title)
	}
	detail, ok := def.Result.Fields["detail_url"]
	if !ok || !detail.ResolveURL || detail.Attribute != "href" {
		t.Errorf("fields.detail_url wrong: %#v", detail)
	}
}

func TestLoadDefinition_rejectsTooLarge(t *testing.T) {
	big := strings.Repeat("x", MaxDefinitionBytes+1)
	_, err := LoadDefinition([]byte(big), "huge.yml")
	if err == nil {
		t.Fatalf("expected oversize error")
	}
	if !errors.Is(err, ErrDefinitionTooLarge) {
		t.Errorf("want ErrDefinitionTooLarge, got %v", err)
	}
}

func TestLoadDefinition_rejectsInvalidYAML(t *testing.T) {
	_, err := LoadDefinition([]byte("this is : not: valid yaml ::: ::: :::"), "bad.yml")
	if err == nil {
		t.Fatalf("expected yaml error")
	}
	if !errors.Is(err, ErrInvalidYAML) {
		t.Errorf("want ErrInvalidYAML, got %v", err)
	}
}

func TestLoadDefinition_rejectsEmpty(t *testing.T) {
	_, err := LoadDefinition(nil, "empty.yml")
	if err == nil {
		t.Fatalf("expected empty error")
	}
	if !errors.Is(err, ErrInvalidYAML) {
		t.Errorf("want ErrInvalidYAML, got %v", err)
	}
}

func TestLoadDefinition_rejectsUnknownField(t *testing.T) {
	// "nope" is not a known field in model.IndexerDefinition; strict mode
	// should reject it.
	bad := []byte(`
id: foo
name: Foo
version: 1.0.0
type: public
protocol: declarative
nope: 1
`)
	_, err := LoadDefinition(bad, "unknown.yml")
	if err == nil {
		t.Fatalf("expected strict-mode error for unknown field")
	}
	if !errors.Is(err, ErrInvalidYAML) {
		t.Errorf("want ErrInvalidYAML, got %v", err)
	}
}

func TestLoadDir_readsFixturesAndAggregatesErrors(t *testing.T) {
	good := `
id: alpha
name: Alpha
version: 1.0.0
type: public
protocol: mock
links:
  - https://example.com
`
	bad := `not : yaml : at : all`
	fsys := fstest.MapFS{
		"indexers/alpha.yml":  &fstest.MapFile{Data: []byte(good)},
		"indexers/ignore.txt": &fstest.MapFile{Data: []byte("skip")},
		"indexers/bad.yml":    &fstest.MapFile{Data: []byte(bad)},
	}
	defs, errs := LoadDir(fsys, "indexers")
	if len(errs) != 1 {
		t.Fatalf("want 1 error (bad.yml), got %d (%v)", len(errs), errs)
	}
	if _, ok := defs["alpha"]; !ok {
		t.Fatalf("alpha should have loaded")
	}
	if _, ok := defs["bad"]; ok {
		t.Fatalf("bad should not have loaded")
	}
}
