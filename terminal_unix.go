//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package main

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether r is a terminal. Character devices such as
// /dev/null are not terminals; only a real tty should stop notify from
// reading stdin.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(), ioctlReadTermios,
		uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return errno == 0
}
