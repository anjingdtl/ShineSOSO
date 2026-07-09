package normalize

import "encoding/base32"

// decodeBase32 wraps base32.StdEncoding.DecodeString so the import is
// not visible in the public API file.
func decodeBase32(s string) ([]byte, error) {
    return base32.StdEncoding.DecodeString(s)
}
