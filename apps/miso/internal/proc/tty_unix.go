//go:build !windows

package proc

import (
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// GiveTerminal makes pgid the terminal's foreground process group so a spawned
// child can drive the TTY (raw mode, cursor queries, etc.) instead of stalling
// on SIGTTOU in a background group. Returns the previous foreground group to
// hand back later, and ok=false when fd is not a controlling terminal.
// SIGTTOU/SIGTTIN are ignored so miso's own tcsetpgrp calls aren't stopped.
func GiveTerminal(fd uintptr, pgid int) (prev int, ok bool) {
	prev, err := unix.IoctlGetInt(int(fd), unix.TIOCGPGRP)
	if err != nil {
		return 0, false
	}
	signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN)
	if err := unix.IoctlSetPointerInt(int(fd), unix.TIOCSPGRP, pgid); err != nil {
		signal.Reset(syscall.SIGTTOU, syscall.SIGTTIN)
		return 0, false
	}
	return prev, true
}

// RestoreTerminal hands foreground control back to pgid and stops ignoring the
// job-control stop signals.
func RestoreTerminal(fd uintptr, pgid int) {
	_ = unix.IoctlSetPointerInt(int(fd), unix.TIOCSPGRP, pgid)
	signal.Reset(syscall.SIGTTOU, syscall.SIGTTIN)
}
