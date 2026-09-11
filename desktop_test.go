package main

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestAppleScriptString(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", `""`},
		{"build finished", `"build finished"`},
		{`say "hi"`, `"say \"hi\""`},
		{`C:\path\to`, `"C:\\path\\to"`},
		{"line one\nline two", `"line one\nline two"`},
		{"a\tb\r\n", `"a\tb\r\n"`},
		{"bell\a and esc\x1b[0m", `"bell and esc[0m"`},
		{"ünïcødé ✓", `"ünïcødé ✓"`},
		// A quote must not be able to end the literal early.
		{`" & (do shell script "id") & "`, `"\" & (do shell script \"id\") & \""`},
		{`\"`, `"\\\""`},
	}
	for _, tt := range tests {
		if got := appleScriptString(tt.in); got != tt.want {
			t.Errorf("appleScriptString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestAppleScript(t *testing.T) {
	got := appleScript(`CI "main"`, "tests passed", Normal)
	want := `display notification "tests passed" with title "CI \"main\""`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if got := appleScript("t", "b", High); !strings.HasSuffix(got, ` sound name "Glass"`) {
		t.Errorf("high priority should play a sound: %s", got)
	}
}

func TestDesktopDarwin(t *testing.T) {
	d := newFakeDesktop()
	d.goos = "darwin"
	if err := d.Send(context.Background(), Message{Body: "done"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"osascript", "-e", `display notification "done" with title "notify"`}
	if len(d.calls) != 1 || !slices.Equal(d.calls[0], want) {
		t.Errorf("ran %q, want %q", d.calls, want)
	}
}

func TestDesktopLinux(t *testing.T) {
	for p, urgency := range map[Priority]string{Low: "low", Normal: "normal", High: "critical"} {
		d := newFakeDesktop()
		if err := d.Send(context.Background(), Message{Title: "CI", Body: "-rf done", Priority: p}); err != nil {
			t.Fatal(err)
		}
		want := []string{"notify-send", "--app-name=notify", "--urgency=" + urgency, "--", "CI", "-rf done"}
		if len(d.calls) != 1 || !slices.Equal(d.calls[0], want) {
			t.Errorf("ran %q, want %q", d.calls, want)
		}
	}
}

func TestDesktopDropsNUL(t *testing.T) {
	// As from find -print0: exec can't pass a NUL in an argument.
	d := newFakeDesktop()
	if err := d.Send(context.Background(), Message{Body: "a\x00b"}); err != nil {
		t.Fatal(err)
	}
	if body := d.calls[0][len(d.calls[0])-1]; body != "ab" {
		t.Errorf("body = %q, want %q", body, "ab")
	}
	if err := runCommand(context.Background(), "true", d.calls[0][1:]...); err != nil {
		t.Errorf("running with these arguments: %v", err)
	}
}

func TestDesktopUnavailable(t *testing.T) {
	d := noDesktop()
	if d.available() {
		t.Error("available() = true without notify-send")
	}
	if err := d.Send(context.Background(), Message{Body: "hi"}); err == nil {
		t.Error("expected an error without notify-send")
	}
}

func TestRunCommandReportsOutput(t *testing.T) {
	err := runCommand(context.Background(), "sh", "-c", "echo boom >&2; exit 3")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want it to include the command's output", err)
	}
}
