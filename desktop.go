package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Desktop shows a notification on this machine: with osascript on macOS,
// or notify-send (libnotify) on Linux and the BSDs.
type Desktop struct {
	goos     string
	lookPath func(file string) (string, error)
	run      func(ctx context.Context, name string, args ...string) error
}

func newDesktop() *Desktop {
	return &Desktop{goos: runtime.GOOS, lookPath: exec.LookPath, run: runCommand}
}

func (d *Desktop) Name() string { return "mac" }

// available reports whether this machine can show desktop notifications.
func (d *Desktop) available() bool {
	_, _, err := d.command(Message{})
	return err == nil
}

func (d *Desktop) Send(ctx context.Context, m Message) error {
	name, args, err := d.command(m)
	if err != nil {
		return err
	}
	return d.run(ctx, name, args...)
}

// command returns the program and arguments that display m.
func (d *Desktop) command(m Message) (string, []string, error) {
	title := m.Title
	if title == "" {
		title = "notify"
	}
	if d.goos == "darwin" {
		return "osascript", []string{"-e", appleScript(title, m.Body, m.Priority)}, nil
	}
	if _, err := d.lookPath("notify-send"); err != nil {
		return "", nil, errors.New("no desktop notifier found: needs osascript (macOS) or notify-send (libnotify)")
	}
	urgency := "normal"
	switch m.Priority {
	case Low:
		urgency = "low"
	case High:
		urgency = "critical"
	}
	// "--" so that a message starting with a dash isn't read as an option.
	return "notify-send", []string{"--app-name=notify", "--urgency=" + urgency, "--", title, m.Body}, nil
}

// appleScript returns a "display notification" statement for osascript.
// High priority messages also play a sound.
func appleScript(title, body string, p Priority) string {
	s := "display notification " + appleScriptString(body) + " with title " + appleScriptString(title)
	if p == High {
		s += ` sound name "Glass"`
	}
	return s
}

// appleScriptString quotes s as an AppleScript string literal. Backslashes
// and double quotes are escaped, newlines, returns and tabs are written as
// \n, \r and \t, and other control characters are dropped.
func appleScriptString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			// Not representable in a literal; leave it out.
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func runCommand(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return fmt.Errorf("%s: %v: %s", name, err, msg)
	}
	return fmt.Errorf("%s: %v", name, err)
}
