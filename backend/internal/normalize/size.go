package normalize

import (
    "fmt"
    "strconv"
    "strings"
)

// ParseSizeBytes converts a human-readable size string (e.g. "8.4 GB",
// "1.2 MiB", "512KB") into a byte count (spec §16.2). Returns an error
// for empty, malformed, or negative inputs.
func ParseSizeBytes(s string) (int64, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return 0, fmt.Errorf("normalize: empty size")
    }
    // Split numeric and unit parts.
    cut := 0
    for cut < len(s) {
        r := s[cut]
        if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '-' || r == '+' {
            cut++
            continue
        }
        break
    }
    if cut == 0 {
        return 0, fmt.Errorf("normalize: no number in %q", s)
    }
    numStr := strings.ReplaceAll(s[:cut], ",", "")
    num, err := strconv.ParseFloat(numStr, 64)
    if err != nil {
        return 0, fmt.Errorf("normalize: parse number %q: %w", numStr, err)
    }
    if num < 0 {
        return 0, fmt.Errorf("normalize: negative size %q", s)
    }
    unit := strings.ToLower(strings.TrimSpace(s[cut:]))
    var mult float64
    switch unit {
    case "", "b", "byte", "bytes":
        mult = 1
    case "k", "kb":
        mult = 1e3
    case "kib":
        mult = 1 << 10
    case "m", "mb":
        mult = 1e6
    case "mib":
        mult = 1 << 20
    case "g", "gb":
        mult = 1e9
    case "gib":
        mult = 1 << 30
    case "t", "tb":
        mult = 1e12
    case "tib":
        mult = 1 << 40
    default:
        return 0, fmt.Errorf("normalize: unknown size unit %q", unit)
    }
    bytes := int64(num * mult)
    if bytes < 0 {
        return 0, fmt.Errorf("normalize: size overflow for %q", s)
    }
    return bytes, nil
}
