package normalize

import "testing"

func TestParseSizeBytes(t *testing.T) {
    cases := []struct {
        in   string
        want int64
        err  bool
    }{
        {"1 B", 1, false},
        {"512", 512, false},
        {"1 KB", 1000, false},
        {"1 KiB", 1024, false},
        {"1.5 MB", 1_500_000, false},
        {"2 MiB", 2 * 1024 * 1024, false},
        {"8.4 GB", 8_400_000_000, false},
        {"1 GiB", 1 << 30, false},
        {"1 TB", 1_000_000_000_000, false},
        {"1 TiB", 1 << 40, false},
        {"3,200 KB", 3_200_000, false}, // comma thousands separator
        {"", 0, true},
        {"abc", 0, true},
        {"-1 GB", 0, true},
        {"1 XB", 0, true},
    }
    for _, tc := range cases {
        t.Run(tc.in, func(t *testing.T) {
            got, err := ParseSizeBytes(tc.in)
            if tc.err {
                if err == nil {
                    t.Fatalf("expected error for %q, got %d", tc.in, got)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error for %q: %v", tc.in, err)
            }
            if got != tc.want {
                t.Errorf("ParseSizeBytes(%q) = %d, want %d", tc.in, got, tc.want)
            }
        })
    }
}
