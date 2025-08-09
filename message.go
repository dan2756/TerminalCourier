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
	Title string
	Body  string
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
