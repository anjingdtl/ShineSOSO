package normalize

import (
    "testing"
    "time"
)

func TestParseDate(t *testing.T) {
    cases := []struct {
        in       string
        wantYear int
        err      bool
    }{
        {"2026-07-09T12:34:56Z", 2026, false},
        {"2026-07-09 12:34:56", 2026, false},
        {"2026-07-09", 2026, false},
        {"09 Jul 2026 12:34", 2026, false},
        {"", 0, true},
        {"not a date", 0, true},
    }
    for _, tc := range cases {
        t.Run(tc.in, func(t *testing.T) {
            got, err := ParseDate(tc.in)
            if tc.err {
                if err == nil {
                    t.Fatalf("expected error for %q, got %v", tc.in, got)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if got.Year() != tc.wantYear {
                t.Errorf("year want %d, got %d", tc.wantYear, got.Year())
            }
            if got.Location() != time.UTC {
                t.Errorf("result should be UTC, got %v", got.Location())
            }
        })
    }
}

func TestParseDateCustomLayouts(t *testing.T) {
    got, err := ParseDate("09.07.2026", "02.01.2006")
    if err != nil {
        t.Fatal(err)
    }
    if got.Day() != 9 || got.Month() != time.July || got.Year() != 2026 {
        t.Errorf("want 9 Jul 2026, got %v", got)
    }
}

func TestParseUnixSeconds(t *testing.T) {
    got := ParseUnixSeconds(0)
    if !got.Equal(time.Unix(0, 0).UTC()) {
        t.Fatalf("unix 0 should be 1970-01-01 UTC, got %v", got)
    }
    if got.Location() != time.UTC {
        t.Errorf("should be UTC, got %v", got.Location())
    }
}
