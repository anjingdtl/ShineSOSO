package webembed

import (
    "io"
    "io/fs"
    "strings"
)

// readSeeker wraps an fs.File (which may not implement io.Seeker) by
// reading its full contents into memory. Embedded assets are tiny so this
// is acceptable; it lets us pass the body to http.ServeContent which
// supports Range requests.
func readSeeker(f fs.File) io.ReadSeeker {
    b, err := io.ReadAll(f)
    if err != nil {
        return strings.NewReader("")
    }
    return strings.NewReader(string(b))
}
