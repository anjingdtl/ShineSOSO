package indexer

import (
	"strings"
	"testing"
)

func TestApplyFilters_Replace(t *testing.T) {
	got, err := ApplyFilters("hello world", []string{"replace"}, "")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	// replace without args is a no-op (intentional; see filters.go)
	if got != "hello world" {
		t.Errorf("replace should be no-op, got %q", got)
	}
}

func TestApplyFilters_RegexExtract(t *testing.T) {
	in := "magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12"
	got, err := ApplyFilters(in, []string{"regex_extract"}, "")
	if err != nil {
		t.Fatalf("regex_extract: %v", err)
	}
	// 40-char hex captured
	if len(got) < 32 || len(got) > 40 {
		t.Errorf("regex_extract out of expected range: %q", got)
	}
}

func TestApplyFilters_RegexExtractNoMatch(t *testing.T) {
	_, err := ApplyFilters("no hash here", []string{"regex_extract"}, "")
	if err == nil {
		t.Error("expected error when no hash found")
	}
}

func TestApplyFilters_ParseInt_PlainNumber(t *testing.T) {
	got, _ := ApplyFilters("42", []string{"parse_int"}, "")
	if got != "42" {
		t.Errorf("got %q want 42", got)
	}
}

func TestApplyFilters_ParseInt_WithTrailingText(t *testing.T) {
	got, _ := ApplyFilters("42 seeds", []string{"parse_int"}, "")
	if got != "42" {
		t.Errorf("got %q want 42", got)
	}
}

func TestApplyFilters_ParseInt_Negative(t *testing.T) {
	got, _ := ApplyFilters("-7", []string{"parse_int"}, "")
	if got != "-7" {
		t.Errorf("got %q want -7", got)
	}
}

func TestApplyFilters_ParseInt_Invalid(t *testing.T) {
	_, err := ApplyFilters("not a number", []string{"parse_int"}, "")
	if err == nil {
		t.Error("expected error for non-numeric input")
	}
}

func TestApplyFilters_ParseFloat_European(t *testing.T) {
	got, _ := ApplyFilters("3,14", []string{"parse_float"}, "")
	if got != "3.14" {
		t.Errorf("got %q want 3.14", got)
	}
}

func TestApplyFilters_ParseFloat_Invalid(t *testing.T) {
	_, err := ApplyFilters("not a number", []string{"parse_float"}, "")
	if err == nil {
		t.Error("expected error for non-numeric input")
	}
}

func TestApplyFilters_ResolveURL_NoBase(t *testing.T) {
	got, err := ApplyFilters("/detail/1", []string{"resolve_url"}, "")
	if err != nil {
		t.Fatalf("resolve_url without base: %v", err)
	}
	if got != "/detail/1" {
		t.Errorf("no-base resolve_url should return input unchanged, got %q", got)
	}
}

func TestApplyFilters_ResolveURL_AbsoluteRef(t *testing.T) {
	got, err := ApplyFilters("https://other.com/x", []string{"resolve_url"}, "https://example.com/page")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(got, "https://other.com/") {
		t.Errorf("absolute ref should not be merged with base, got %q", got)
	}
}

func TestApplyFilters_ResolveURL_RelativeRef(t *testing.T) {
	got, err := ApplyFilters("../foo", []string{"resolve_url"}, "https://example.com/a/b")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(got, "https://example.com/") {
		t.Errorf("relative ref should be merged, got %q", got)
	}
}

func TestApplyFilters_ExtractInfoHash_FromMagnet(t *testing.T) {
	got, err := ApplyFilters("magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12", []string{"extract_info_hash"}, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.EqualFold(got, "abcdef1234567890abcdef1234567890abcdef12") {
		t.Errorf("extract_info_hash got %q", got)
	}
}

func TestApplyFilters_ExtractInfoHash_FromBare40Hex(t *testing.T) {
	got, err := ApplyFilters("prefix abcdef1234567890abcdef1234567890abcdef12 suffix", []string{"extract_info_hash"}, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.EqualFold(got, "abcdef1234567890abcdef1234567890abcdef12") {
		t.Errorf("extract_info_hash got %q", got)
	}
}

func TestApplyFilters_ExtractInfoHash_NoMatch(t *testing.T) {
	_, err := ApplyFilters("nothing here", []string{"extract_info_hash"}, "")
	if err == nil {
		t.Error("expected error when no hash")
	}
}

func TestApplyFilters_UnknownFilter(t *testing.T) {
	_, err := ApplyFilters("x", []string{"made_up_filter"}, "")
	if err == nil {
		t.Error("expected error for unknown filter")
	}
}

func TestApplyFiltersByLayout_ParseDate(t *testing.T) {
	got, err := ApplyFiltersByLayout("2026-07-09", []string{"parse_date"}, "", []string{"2006-01-02"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(got, "2026-07-09T") {
		t.Errorf("want ISO8601 prefix, got %q", got)
	}
}

func TestApplyFiltersByLayout_ParseDate_BundledLayouts(t *testing.T) {
	// normalize.ParseDate supports a set of common date formats including
	// unix timestamps and ISO dates.
	got, err := ApplyFiltersByLayout("2026-07-09", []string{"parse_date"}, "", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(got, "2026-07-09T") {
		t.Errorf("want ISO8601 prefix, got %q", got)
	}
}

func TestApplyFiltersByLayout_ParseDate_Invalid(t *testing.T) {
	_, err := ApplyFiltersByLayout("not a date", []string{"parse_date"}, "", []string{"2006-01-02"})
	if err == nil {
		t.Error("expected error for unparseable date")
	}
}

func TestApplyFiltersByLayout_UnknownFilter(t *testing.T) {
	_, err := ApplyFiltersByLayout("x", []string{"made_up"}, "", nil)
	if err == nil {
		t.Error("expected error for unknown filter")
	}
}

func TestApplyFilters_TrimLeadingTrailing(t *testing.T) {
	got, _ := ApplyFilters("  hello  ", []string{"trim"}, "")
	if got != "hello" {
		t.Errorf("got %q want hello", got)
	}
}

func TestApplyFilters_LowerUpper(t *testing.T) {
	if got, _ := ApplyFilters("Hello", []string{"lower"}, ""); got != "hello" {
		t.Errorf("lower: %q", got)
	}
	if got, _ := ApplyFilters("Hello", []string{"upper"}, ""); got != "HELLO" {
		t.Errorf("upper: %q", got)
	}
}

func TestApplyFilters_PipelineComposition(t *testing.T) {
	got, err := ApplyFilters("  3.14 GB  ", []string{"trim", "parse_size"}, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 3.14 GB (decimal SI) = 3.14 * 1e9 = 3_140_000_000
	if got != "3140000000" {
		t.Errorf("pipeline size: got %q want 3140000000", got)
	}
}