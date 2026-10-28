package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
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
