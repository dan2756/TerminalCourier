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
func readMessage(args []string, stdin io.Reader) (string, error) {
	var msg string
	if len(args) > 0 {
		msg = strings.Join(args, " ")
	} else {
		if isTerminal(stdin) {
			return "", errors.New("no message: pass it as arguments or pipe it on stdin")
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		msg = string(b)
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

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
