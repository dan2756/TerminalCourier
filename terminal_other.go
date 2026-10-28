//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package main

import (
	"io"
	"os"
)

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
