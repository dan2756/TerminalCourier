package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Message is what gets delivered to a notification service.
type Message struct {
	Title    string
	Body     string
	Priority Priority
}

// Priority is how urgently a message should be delivered. Each sink maps
// it onto whatever its service supports.
type Priority int

const (
	Low    Priority = -1
	Normal Priority = 0
	High   Priority = 1
)

func parsePriority(s string) (Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return Low, nil
	case "", "normal":
		return Normal, nil
	case "high":
		return High, nil
	}
	return Normal, fmt.Errorf("invalid priority %q (want low, normal or high)", s)
}

// readMessage returns the message to send: the arguments joined with
// spaces, or everything on stdin when there are no arguments.
//
// With tee set, stdin is copied to it unchanged as it arrives, and read to
// the end even when the message comes from the arguments, so that notify
// fires once the command feeding the pipe is done.
func readMessage(args []string, stdin io.Reader, tee io.Writer) (string, error) {
	readStdin := len(args) == 0 || tee != nil
	if readStdin && isTerminal(stdin) {
		if len(args) == 0 {
			return "", errors.New("no message: pass it as arguments or pipe it on stdin")
		}
		readStdin = false // nothing to pass through
	}
	var msg string
	if readStdin {
		r := stdin
		if tee != nil {
			r = io.TeeReader(stdin, &passThrough{w: tee})
		}
		b, err := io.ReadAll(r)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		msg = string(b)
	}
	if len(args) > 0 {
		msg = strings.Join(args, " ")
	}
	msg = clean(msg)
	if msg == "" {
		return "", errors.New("message is empty, nothing to send")
	}
	return msg, nil
}

// clean drops the leading blank lines and trailing whitespace that command
// output usually carries, keeping any indentation, and replaces invalid
// UTF-8 so services don't reject the payload.
func clean(s string) string {
	s = strings.ToValidUTF8(s, "�")
	s = strings.TrimRight(s, " \t\r\n")
	return strings.TrimLeft(s, "\r\n")
}

// passThrough copies everything to w until a write fails, say because the
// reader at the other end of the pipe exited, and then quietly drops the
// rest. Reading stdin carries on, so the message still gets sent.
type passThrough struct {
	w      io.Writer
	failed bool
}

func (p *passThrough) Write(b []byte) (int, error) {
	if !p.failed {
		if _, err := p.w.Write(b); err != nil {
			p.failed = true
		}
	}
	return len(b), nil
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
