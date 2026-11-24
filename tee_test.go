package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
)

const buildLog = "compiling...\n  ok  pkg/a\n  FAIL pkg/b\n\n"

func TestRunTee(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"message from stdin", []string{"--tee"}, "compiling...\n  ok  pkg/a\n  FAIL pkg/b"},
		{"message from args", []string{"--tee", "build", "finished"}, "build finished"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slack := newCapture(t, http.StatusOK, "ok")
			env := envMap(map[string]string{"NOTIFY_SLACK_WEBHOOK": slack.URL})
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, strings.NewReader(buildLog), &stdout, &stderr, env); code != 0 {
				t.Fatalf("exit %d, stderr: %s", code, stderr.String())
			}
			if stdout.String() != buildLog {
				t.Errorf("stdout = %q, want stdin unchanged", stdout.String())
			}
			var got slackMessage
			json.Unmarshal(slack.body, &got)
			if got.Text != tt.want {
				t.Errorf("sent %q, want %q", got.Text, tt.want)
			}
		})
	}
}

// TestClosedStdout checks that a closed stdout kills notify, and that
// --tee survives it so the notification can still be sent. Each case
// runs in a child process: once SIGPIPE is ignored it stays ignored.
func TestClosedStdout(t *testing.T) {
	mode := os.Getenv("NOTIFY_STDOUT_MODE")
	if mode != "" {
		// Wait until the parent has closed the read end of stdout.
		_, _ = io.Copy(io.Discard, os.Stdin)
		var stderr bytes.Buffer
		var args []string
		var in io.Reader
		switch mode {
		case "plain":
			args = []string{"--version"}
		case "tee":
			args = []string{"--tee", "--version"}
			in = strings.NewReader(buildLog)
		default:
			fmt.Fprintf(os.Stderr, "unknown mode %s\n", mode)
			os.Exit(2)
		}
		code := run(args, in, os.Stdout, &stderr, envMap(nil))
		fmt.Fprintf(os.Stderr, "code=%d\n%s", code, stderr.String())
		os.Exit(code)
	}

	for _, mode := range []string{"plain", "tee"} {
		t.Run(mode, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			defer w.Close()
			cmd := exec.Command(os.Args[0], "-test.run=^TestClosedStdout$")
			cmd.Env = append(os.Environ(), "NOTIFY_STDOUT_MODE="+mode)
			cmd.Stdout = w
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			w.Close()
			r.Close() // the child's next write to stdout is EPIPE
			stdin.Close()
			err = cmd.Wait()
			if mode == "tee" {
				if err != nil {
					t.Fatalf("--tee died from a closed stdout: %v\n%s", err, stderr.String())
				}
				if !strings.Contains(stderr.String(), "notify "+versionString()) {
					t.Fatalf("stderr %q, want the version", stderr.String())
				}
				return
			}
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				t.Fatalf("closed stdout exited %v, want SIGPIPE\n%s", err, stderr.String())
			}
			ws, ok := ee.Sys().(syscall.WaitStatus)
			if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGPIPE {
				t.Fatalf("closed stdout status %v, want SIGPIPE\n%s", ee.Sys(), stderr.String())
			}
		})
	}
}

func TestRunTeePassesThroughOnError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--tee", "--ntfy"}, strings.NewReader(buildLog), &stdout, &stderr, envMap(nil))
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if stdout.String() != buildLog {
		t.Errorf("stdout = %q, want stdin unchanged", stdout.String())
	}
	if !strings.Contains(stderr.String(), "ntfy is not configured") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunMalformedFlag(t *testing.T) {
	tests := [][]string{
		{"---bogus"},
		{"---bogus", "--tee"},
		{"--tee", "---bogus"},
		{"---a", "--tee", "---b"},
		{"-=x", "--tee"},
		{"--=slack", "--tee"},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		code := run(args, strings.NewReader(buildLog), &stdout, &stderr, envMap(nil))
		if code != 2 {
			t.Errorf("%v: exit %d, want 2; stderr %q", args, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "bad flag syntax") {
			t.Errorf("%v: stderr %q, want the bad flag syntax", args, stderr.String())
		}
		wantOut := ""
		for _, a := range args {
			if a == "--tee" {
				wantOut = buildLog
			}
		}
		if stdout.String() != wantOut {
			t.Errorf("%v: stdout %q, want %q", args, stdout.String(), wantOut)
		}
	}
}

func TestRunTeePassesThroughOnBadFlag(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--tee", "--slak", "-t", "build"}, "flag provided but not defined: -slak"},
		{[]string{"--tee", "-t"}, "flag needs an argument: -t"},
		{[]string{"--slak", "--tee"}, "flag provided but not defined: -slak"},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(buildLog), &stdout, &stderr, envMap(nil)); code != 2 {
			t.Errorf("%v: exit %d, want 2", tt.args, code)
		}
		if stdout.String() != buildLog {
			t.Errorf("%v: stdout = %q, want stdin unchanged", tt.args, stdout.String())
		}
		if !strings.Contains(stderr.String(), tt.want) {
			t.Errorf("%v: stderr %q, want %q", tt.args, stderr.String(), tt.want)
		}
	}
}

func TestRunTeeHelpAndVersionPassStdin(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--tee", "--help"}, "Usage: notify"},
		{[]string{"--help", "--tee"}, "Usage: notify"},
		{[]string{"-h", "--tee"}, "Usage: notify"},
		{[]string{"--tee", "--version"}, "notify " + versionString()},
		{[]string{"--version", "--tee"}, "notify " + versionString()},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(buildLog), &stdout, &stderr, envMap(nil)); code != 0 {
			t.Errorf("%v: exit %d, stderr %s", tt.args, code, stderr.String())
		}
		if stdout.String() != buildLog {
			t.Errorf("%v: stdout = %q, want stdin unchanged", tt.args, stdout.String())
		}
		if !strings.Contains(stderr.String(), tt.want) {
			t.Errorf("%v: stderr %q, want %q", tt.args, stderr.String(), tt.want)
		}
	}

	// Without --tee, help and version stay on stdout and stdin is left alone.
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, strings.NewReader(buildLog), &stdout, &stderr, envMap(nil)); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if stdout.String() != "notify "+versionString()+"\n" || stderr.Len() != 0 {
		t.Errorf("stdout %q stderr %q", stdout.String(), stderr.String())
	}
}

func TestRunTeeDryRunUsesStderr(t *testing.T) {
	env := envMap(map[string]string{"NOTIFY_SLACK_WEBHOOK": "https://hooks.slack.com/services/T0/B0/X"})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--tee", "-n"}, strings.NewReader(buildLog), &stdout, &stderr, env); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if stdout.String() != buildLog {
		t.Errorf("stdout = %q, want stdin unchanged", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--- slack ---") {
		t.Errorf("dry-run output should go to stderr, got %q", stderr.String())
	}
}

type brokenPipe struct{ writes int }

func (b *brokenPipe) Write(p []byte) (int, error) {
	b.writes++
	return 0, errors.New("broken pipe")
}

func TestTeeKeepsReadingAfterWriteFails(t *testing.T) {
	w := &brokenPipe{}
	// One byte per read, so there are many writes to fail.
	r := iotest.OneByteReader(strings.NewReader(buildLog))
	got, err := readMessage(nil, r, w)
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.TrimSpace(buildLog) {
		t.Errorf("message = %q", got)
	}
	if w.writes != 1 {
		t.Errorf("tried to write %d times after the first failure", w.writes-1)
	}
}

// output is n bytes of command output, made up as it is read.
type output struct{ n int }

func (o *output) Read(p []byte) (int, error) {
	if o.n == 0 {
		return 0, io.EOF
	}
	p = p[:min(len(p), o.n)]
	for i := range p {
		p[i] = 'x'
	}
	o.n -= len(p)
	return len(p), nil
}

type countingWriter struct{ n int }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += len(p)
	return len(p), nil
}

func TestTeeKeepsBoundedMemory(t *testing.T) {
	const size = 64 << 20
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"message from stdin", nil, strings.Repeat("x", maxStdin) + ellipsis},
		{"message from args", []string{"backup", "finished"}, "backup finished"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var passed countingWriter
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			got, err := readMessage(tt.args, &output{n: size}, &passed)
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatal(err)
			}
			if passed.n != size {
				t.Errorf("passed %d bytes through, want %d", passed.n, size)
			}
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 8<<20 {
				t.Errorf("allocated %d MB to read %d MB of stdin", alloc>>20, size>>20)
			}
			if got != tt.want {
				t.Errorf("message is %d bytes, want %d", len(got), len(tt.want))
			}
		})
	}
}

func TestReadMessageAtStdinLimit(t *testing.T) {
	in := strings.Repeat("x", maxStdin)
	got, err := readMessage(nil, strings.NewReader(in+"y"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != in+ellipsis {
		t.Errorf("message is %d bytes, want %d", len(got), len(in+ellipsis))
	}
	if got, _ := readMessage(nil, strings.NewReader(in), nil); got != in {
		t.Errorf("message is %d bytes, want %d with nothing cut", len(got), len(in))
	}
}
