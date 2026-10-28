//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package main

import "syscall"

const ioctlReadTermios = syscall.TIOCGETA
