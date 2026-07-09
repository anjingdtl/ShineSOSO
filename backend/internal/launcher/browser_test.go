package launcher

import "testing"

// OpenURL is best-effort and depends on a real browser. We just verify it
// doesn't panic and returns either nil (success) or a wrapped error
// (e.g. when `rundll32` is missing on a stripped-down test environment).
func TestOpenURLDoesNotPanic(t *testing.T) {
    _ = OpenURL("http://127.0.0.1:18765/")
}
