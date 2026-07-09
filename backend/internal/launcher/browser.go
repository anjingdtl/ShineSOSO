package launcher

import (
    "fmt"
    "os/exec"
	"runtime"
)

// OpenURL asks the OS to open url with the default handler.
//
// On Windows we use `rundll32 url.dll,FileProtocolHandler` which respects
// the user's default browser choice. On macOS we use `open`; on Linux we
// use `xdg-open`.
//
// The function is best-effort: it returns nil even when the OS command
// exits non-zero, because users on headless / RDP machines often have no
// browser. Callers should log but never block on this.
func OpenURL(url string) error {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "windows":
        // rundll32 url.dll,FileProtocolHandler <url> opens the default browser.
        // Note: we use exec.Command, not CommandContext, because we want the
        // browser to outlive the launcher.
        cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
    case "darwin":
        cmd = exec.Command("open", url)
    default:
        cmd = exec.Command("xdg-open", url)
    }
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("open url %q: %w", url, err)
    }
    // Detach: do not wait. Some browsers return quickly via the protocol
    // handler; others may not. We must not block shutdown.
    go func() { _ = cmd.Wait() }()
    return nil
}
