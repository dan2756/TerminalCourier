package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
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
