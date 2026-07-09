package catalog

import (
	"errors"
	"strings"
	"testing"

	"github.com/local/easysearch/backend/internal/model"
)

func goodDef() model.IndexerDefinition {
	return model.IndexerDefinition{
		Schema:   SupportedSchema,
		ID:       "example-public",
		Name:     "Example Public",
		Version:  "1.0.0",
		Type:     "public",
		Protocol: "declarative",
		Links:    []string{"https://example.com/"},
	}
}

func TestValidate_acceptsGoodDefinition(t *testing.T) {
	if err := Validate(goodDef()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidate_rejectsBadSchema(t *testing.T) {
	d := goodDef()
	d.Schema = 99
	err := Validate(d)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != CodeSchemaUnsupported {
		t.Fatalf("want %v, got %v", CodeSchemaUnsupported, err)
	}
}

func TestValidate_rejectsBadID(t *testing.T) {
	for _, bad := range []string{"", "Has-Caps", "1bad", "trailing-", "-leading", "bad space"} {
		d := goodDef()
		d.ID = bad
		err := Validate(d)
		if err == nil {
			t.Errorf("id %q should fail", bad)
		}
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Code != CodeIDInvalid {
			t.Errorf("id %q: want %v, got %v", bad, CodeIDInvalid, err)
		}
	}
}

func TestValidate_rejectsBadName(t *testing.T) {
	d := goodDef()
	d.Name = "   "
	err := Validate(d)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != CodeNameMissing {
		t.Fatalf("want %v, got %v", CodeNameMissing, err)
	}
}

func TestValidate_rejectsNonPublicType(t *testing.T) {
	d := goodDef()
	d.Type = "private"
	if err := Validate(d); err == nil {
		t.Fatalf("want type error")
	}
}

func TestValidate_rejectsUnsupportedProtocol(t *testing.T) {
	d := goodDef()
	d.Protocol = "selenium"
	err := Validate(d)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != CodeProtocolUnsupported {
		t.Fatalf("want %v, got %v", CodeProtocolUnsupported, err)
	}
}

func TestValidate_rejectsNonHTTPSLink(t *testing.T) {
	d := goodDef()
	d.Links = []string{"http://example.com"}
	err := Validate(d)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != CodeLinkNotHTTPS {
		t.Fatalf("want %v, got %v", CodeLinkNotHTTPS, err)
	}
}

func TestValidate_rejectsPrivateIPLink(t *testing.T) {
	for _, link := range []string{
		"https://127.0.0.1",
		"https://localhost/x",
		"https://10.0.0.1",
		"https://192.168.1.1",
		"https://172.16.0.1",
		"https://0.0.0.0",
	} {
		d := goodDef()
		d.Links = []string{link}
		err := Validate(d)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Code != CodeLinkUnsafe {
			t.Errorf("link %q: want %v, got %v", link, CodeLinkUnsafe, err)
		}
	}
}

func TestValidate_rejects172OutsideRange(t *testing.T) {
	d := goodDef()
	// 172.15.x is public, 172.32.x is public; only 172.16..172.31 is private.
	d.Links = []string{"https://172.15.0.1", "https://172.32.0.1"}
	if err := Validate(d); err != nil {
		t.Errorf("public 172.x should pass, got %v", err)
	}
}

func TestValidate_rejectsSelectorInjection(t *testing.T) {
	d := goodDef()
	d.Result.Format = "html"
	d.Result.Fields = map[string]model.FieldDefinition{
		"title": {Selector: "div .title <script>alert(1)</script>", Value: "text"},
	}
	err := Validate(d)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != CodeSelectorForbidden {
		t.Fatalf("want %v, got %v", CodeSelectorForbidden, err)
	}
}

func TestValidate_rejectsBadTemplateVar(t *testing.T) {
	d := goodDef()
	d.Search.Query = map[string]string{
		"keyword": "{{ .Env.HOME }}", // forbidden var
	}
	err := Validate(d)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != CodeTemplateForbidden {
		t.Fatalf("want %v, got %v", CodeTemplateForbidden, err)
	}
}

func TestValidate_allowsSpecExampleVars(t *testing.T) {
	d := goodDef()
	d.Search.Query = map[string]string{
		"keyword":     "{{ query.keyword }}",
		"category_id": "{{ query.category_id }}",
		"page":        "{{ query.page }}",
	}
	if err := Validate(d); err != nil {
		t.Fatalf("allowed template should pass: %v", err)
	}
}

func TestValidate_rejectsEmptyLinks(t *testing.T) {
	d := goodDef()
	d.Links = nil
	err := Validate(d)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Code != CodeLinksMissing {
		t.Fatalf("want %v, got %v", CodeLinksMissing, err)
	}
}

func TestIsAllowedTemplate_handlesNoBraces(t *testing.T) {
	if !isAllowedTemplate("plain text") {
		t.Fatalf("plain text should pass")
	}
	if isAllowedTemplate("{{ .Exec \"ls\" }}") {
		t.Fatalf("malicious template should fail")
	}
	if isAllowedTemplate("{{ query.keyword }} extra") {
		// Template, contains nothing forbidden, should pass.
		// (extra text after }} is fine.)
	} else {
		t.Fatalf("good var should pass")
	}
}

func TestHostIs172Private(t *testing.T) {
	tests := map[string]bool{
		"172.15.0.1": false,
		"172.16.0.1": true,
		"172.31.9.9": true,
		"172.32.0.1": false,
		"172.40.0.1": false,
	}
	for host, want := range tests {
		if got := hostIs172Private(host); got != want {
			t.Errorf("%s: want %v, got %v", host, want, got)
		}
	}
}

func TestAllSelectors_collectsRowsAndFields(t *testing.T) {
	d := goodDef()
	d.Result.Rows.Selector = "tr.r"
	d.Result.Fields = map[string]model.FieldDefinition{
		"title": {Selector: "td.t"},
		"magnet": {Selector: ""}, // missing
	}
	sels := AllSelectors(d)
	if len(sels) != 2 {
		t.Fatalf("want 2 selectors, got %d (%v)", len(sels), sels)
	}
	if sels["title"] != "td.t" || sels["_rows"] != "tr.r" {
		t.Fatalf("missing: %#v", sels)
	}
}

func TestAllTemplates_capturesEveryQueryKey(t *testing.T) {
	d := goodDef()
	d.Search.Query = map[string]string{
		"keyword": "{{ query.keyword }}",
		"page":    "{{ query.page }}",
	}
	tmps := AllTemplates(d)
	if len(tmps) != 2 {
		t.Fatalf("want 2 tmpls, got %d (%v)", len(tmps), tmps)
	}
	for _, k := range []string{"query.keyword", "query.page"} {
		if _, ok := tmps[k]; !ok {
			t.Errorf("missing %q in %v", k, tmps)
		}
	}
}

// Suppress unused-import warning when entire suites get dropped.
var _ = strings.TrimSpace
