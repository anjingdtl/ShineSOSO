// Phase 5 — YAML indexer definition validator.
//
// Implements spec §13.8: schema version, id regex, name required,
// ≥1 https link, type=="public", supported protocol, ≤512 KB (checked
// in the loader), no private/link-local/loopback hosts, no
// code-injection hints in selectors or templates.
//
// The validator is a pure function on the parsed struct; it does not
// resolve DNS. SSRF blocking for *runtime* requests lives in
// security.DefaultValidator.
package catalog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/local/easysearch/backend/internal/model"
)

// SupportedSchema is the only YAML schema version this build understands.
const SupportedSchema = 1

// SupportedProtocols is the allow-list for IndexerDefinition.Protocol.
var SupportedProtocols = []string{"declarative", "torznab", "mock"}

// idRegex matches spec §13.8: lowercase letters, digits, and dashes,
// must start with a letter (no leading digit) and end with a letter/digit.
var idRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// selectorForbidden is a blacklist of substrings that look like code
// injection (script tags, shell backticks, function definitions).
// goquery already escapes matching nodes; this is just defense-in-depth.
var selectorForbidden = []string{
	"<script",
	"javascript:",
	"vbscript:",
	"`",
	"$((",
}

// ValidationError is a structured failure carrying a code the API
// layer surfaces to the UI as a localized message.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Is lets callers do errors.Is(err, ErrValidation).
func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

// Error codes (UI maps these to localized strings).
const (
	CodeSchemaUnsupported  = "SCHEMA_UNSUPPORTED"
	CodeIDInvalid          = "ID_INVALID"
	CodeNameMissing        = "NAME_MISSING"
	CodeLinksMissing       = "LINKS_MISSING"
	CodeLinkNotHTTPS       = "LINK_NOT_HTTPS"
	CodeLinkUnsafe         = "LINK_UNSAFE"
	CodeTypeInvalid        = "TYPE_INVALID"
	CodeProtocolUnsupported = "PROTOCOL_UNSUPPORTED"
	CodeSelectorForbidden  = "SELECTOR_FORBIDDEN"
	CodeTemplateForbidden  = "TEMPLATE_FORBIDDEN"
)

// ErrValidation is the sentinel for all validator failures.
var ErrValidation = errors.New("validation failed")

// Validate checks a parsed definition against spec §13.8. Returns nil
// on success or the FIRST violation (validator is fail-fast).
func Validate(def model.IndexerDefinition) error {
	if def.Schema != SupportedSchema {
		return &ValidationError{
			Code:    CodeSchemaUnsupported,
			Message: fmt.Sprintf("schema %d is not supported (need %d)", def.Schema, SupportedSchema),
		}
	}
	if !idRegex.MatchString(def.ID) {
		return &ValidationError{
			Code:    CodeIDInvalid,
			Message: fmt.Sprintf("id %q must match [a-z0-9-]+ (start/end alphanumeric)", def.ID),
		}
	}
	if strings.TrimSpace(def.Name) == "" {
		return &ValidationError{Code: CodeNameMissing, Message: "name is required"}
	}
	if def.Type != "public" {
		return &ValidationError{Code: CodeTypeInvalid, Message: "type must be 'public'"}
	}
	if !containsString(SupportedProtocols, def.Protocol) {
		return &ValidationError{
			Code:    CodeProtocolUnsupported,
			Message: fmt.Sprintf("protocol %q not in %v", def.Protocol, SupportedProtocols),
		}
	}

	if len(def.Links) == 0 {
		return &ValidationError{Code: CodeLinksMissing, Message: "at least one link is required"}
	}
	for _, link := range def.Links {
		if !strings.HasPrefix(link, "https://") {
			return &ValidationError{
				Code:    CodeLinkNotHTTPS,
				Message: fmt.Sprintf("link %q must start with https://", link),
			}
		}
		if err := quickHostSafetyCheck(link); err != nil {
			return &ValidationError{
				Code:    CodeLinkUnsafe,
				Message: fmt.Sprintf("link %q is unsafe: %v", link, err),
			}
		}
	}

	for name, sel := range AllSelectors(def) {
		if forbidsAny(sel, selectorForbidden) {
			return &ValidationError{
				Code:    CodeSelectorForbidden,
				Message: fmt.Sprintf("field %q selector %q contains forbidden substring", name, sel),
			}
		}
	}
	for name, tmpl := range AllTemplates(def) {
		if !isAllowedTemplate(tmpl) {
			return &ValidationError{
				Code:    CodeTemplateForbidden,
				Message: fmt.Sprintf("field %q template %q uses a forbidden variable", name, tmpl),
			}
		}
	}
	return nil
}

// AllSelectors returns every selector string in a definition
// (rows.selector + each field.selector) keyed by where it came from.
func AllSelectors(def model.IndexerDefinition) map[string]string {
	out := map[string]string{}
	if def.Result.Rows.Selector != "" {
		out["_rows"] = def.Result.Rows.Selector
	}
	for name, f := range def.Result.Fields {
		if f.Selector != "" {
			out[name] = f.Selector
		}
	}
	return out
}

// AllTemplates returns every template string (search.query values +
// search.body) keyed by name.
func AllTemplates(def model.IndexerDefinition) map[string]string {
	out := map[string]string{}
	for k, v := range def.Search.Query {
		out["query."+k] = v
	}
	if def.Search.Body != "" {
		out["body"] = def.Search.Body
	}
	return out
}

// helpers ----------------------------------------------------------------

func containsString(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func forbidsAny(s string, needles []string) bool {
	low := strings.ToLower(s)
	for _, n := range needles {
		if strings.Contains(low, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// isAllowedTemplate permits only the explicit var names from §13.7.
func isAllowedTemplate(tmpl string) bool {
	if !strings.Contains(tmpl, "{{") {
		return true
	}
	allowed := map[string]bool{
		"query.keyword":     true,
		"query.category":    true,
		"query.category_id": true,
		"query.page":        true,
		"indexer.base_url":  true,
	}
	for i := 0; i+1 < len(tmpl); {
		if tmpl[i] != '{' || tmpl[i+1] != '{' {
			i++
			continue
		}
		end := strings.Index(tmpl[i+2:], "}}")
		if end < 0 {
			return false
		}
		expr := strings.TrimSpace(tmpl[i+2 : i+2+end])
		varIdent := expr
		if idx := strings.IndexAny(expr, " |"); idx >= 0 {
			varIdent = strings.TrimSpace(expr[:idx])
		}
		if !allowed[varIdent] {
			return false
		}
		i += 2 + end + 2
	}
	return true
}

// quickHostSafetyCheck rejects obvious unsafe URL forms without DNS.
func quickHostSafetyCheck(raw string) error {
	if !strings.HasPrefix(raw, "https://") {
		return fmt.Errorf("must be https")
	}
	host := strings.ToLower(strings.TrimPrefix(raw, "https://"))
	if slash := strings.IndexAny(host, "/?#"); slash >= 0 {
		host = host[:slash]
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	if strings.HasPrefix(host, "localhost") || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("localhost not allowed")
	}
	if strings.HasPrefix(host, "127.") || host == "::1" {
		return fmt.Errorf("loopback not allowed")
	}
	if strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		(strings.HasPrefix(host, "172.") && hostIs172Private(host)) ||
		strings.HasPrefix(host, "169.254.") ||
		strings.HasPrefix(host, "0.") ||
		strings.HasPrefix(host, "[fd") ||
		strings.HasPrefix(host, "[fe80") {
		return fmt.Errorf("private network not allowed")
	}
	return nil
}

func hostIs172Private(host string) bool {
	parts := strings.SplitN(host, ".", 4)
	if len(parts) < 2 {
		return false
	}
	var second int
	_, err := fmt.Sscanf(parts[1], "%d", &second)
	if err != nil {
		return false
	}
	return second >= 16 && second <= 31
}
