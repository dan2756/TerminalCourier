package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// capture is an httptest server that records the last request body.
type capture struct {
	*httptest.Server
	path string
	body []byte
}

func newCapture(t *testing.T, status int, reply string) *capture {
	t.Helper()
	c := &capture{}
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.path = r.URL.Path
		c.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		io.WriteString(w, reply)
	}))
	t.Cleanup(c.Close)
	return c
}

func TestReadMessage(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{"args", []string{"build", "finished"}, "ignored", "build finished"},
		{"stdin", nil, "line one\nline two\n", "line one\nline two"},
		{"trims blank lines", nil, "\n\n  indented\n\n\n", "  indented"},
		{"invalid utf-8", nil, "bad \xff byte", "bad � byte"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readMessage(tt.args, strings.NewReader(tt.stdin))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadMessageEmpty(t *testing.T) {
	if _, err := readMessage(nil, strings.NewReader(" \n\n")); err == nil {
		t.Fatal("expected an error for an empty message")
	}
}

func TestRunSlack(t *testing.T) {
	srv := newCapture(t, http.StatusOK, "ok")
	var stdout, stderr bytes.Buffer
	env := envMap(map[string]string{"NOTIFY_SLACK_WEBHOOK": srv.URL + "/services/T0/B0/secret"})

	code := run([]string{"-t", "CI"}, strings.NewReader("tests <passed> & done\n"), &stdout, &stderr, env)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if srv.path != "/services/T0/B0/secret" {
		t.Errorf("posted to %q", srv.path)
	}
	var got slackMessage
	if err := json.Unmarshal(srv.body, &got); err != nil {
		t.Fatalf("bad json %q: %v", srv.body, err)
	}
	if want := "*CI*\ntests &lt;passed&gt; &amp; done"; got.Text != want {
		t.Errorf("text = %q, want %q", got.Text, want)
	}
}

func TestRunSlackError(t *testing.T) {
	srv := newCapture(t, http.StatusNotFound, "no_service")
	var stdout, stderr bytes.Buffer
	env := envMap(map[string]string{"NOTIFY_SLACK_WEBHOOK": srv.URL})

	code := run([]string{"hello"}, strings.NewReader(""), &stdout, &stderr, env)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if want := "slack: HTTP 404 Not Found: no_service"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr %q does not mention %q", stderr.String(), want)
	}
}

func TestRunUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, nil, &stdout, &stderr, envMap(nil)); code != 0 {
		t.Errorf("--help exited %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: notify") {
		t.Errorf("--help printed %q", stdout.String())
	}
	if code := run([]string{"--bogus"}, nil, &stdout, &stderr, envMap(nil)); code != 2 {
		t.Errorf("unknown flag exited %d, want 2", code)
	}
}
