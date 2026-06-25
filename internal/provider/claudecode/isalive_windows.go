//go:build windows

package claudecode

import "golang.org/x/sys/windows"

// isAlive reports whether a process is running, using a single
// OpenProcess/GetExitCodeProcess syscall pair. The previous implementation
// shelled out to `tasklist` on every call, spawning a subprocess per live
// session every poll tick — a heavy, Windows-only CPU cost. golang.org/x/sys
// is already in the module graph (via fsnotify), so this adds no dependency.
func isAlive(pid int) bool {
	const stillActive = 259 // STILL_ACTIVE
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		// Handle opened but exit code unreadable: treat as alive rather than
		// risk a false "ended".
		return true
	}
	return code == stillActive
}
