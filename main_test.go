package main

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// capture is an httptest server that records the last request it got.
type capture struct {
	*httptest.Server
	hits   int
	path   string
	header http.Header
	body   []byte
}

func newCapture(t *testing.T, status int, reply string) *capture {
	t.Helper()
	c := &capture{}
	c.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.hits++
		c.path = r.URL.Path
		c.header = r.Header.Clone()
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

func TestRunEveryConfiguredSink(t *testing.T) {
	slack := newCapture(t, http.StatusOK, "ok")
	discord := newCapture(t, http.StatusNoContent, "")
	env := envMap(map[string]string{
		"NOTIFY_SLACK_WEBHOOK":   slack.URL,
		"NOTIFY_DISCORD_WEBHOOK": discord.URL,
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"deploy", "done"}, nil, &stdout, &stderr, env); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if slack.hits != 1 || discord.hits != 1 {
		t.Errorf("slack got %d requests, discord %d; want 1 each", slack.hits, discord.hits)
	}
}

func TestRunSelectedSinksOnly(t *testing.T) {
	slack := newCapture(t, http.StatusOK, "ok")
	discord := newCapture(t, http.StatusNoContent, "")
	ntfy := newCapture(t, http.StatusOK, "{}")
	env := envMap(map[string]string{
		"NOTIFY_SLACK_WEBHOOK":   slack.URL,
		"NOTIFY_DISCORD_WEBHOOK": discord.URL,
		"NOTIFY_NTFY_SERVER":     ntfy.URL,
		"NOTIFY_NTFY_TOPIC":      "builds",
	})
	for _, args := range [][]string{
		{"--discord", "--ntfy", "hi"},
		{"--to", "discord,ntfy", "hi"},
		{"--to", "ntfy", "--discord", "hi"},
	} {
		slack.hits, discord.hits, ntfy.hits = 0, 0, 0
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr, env); code != 0 {
			t.Fatalf("%v: exit %d, stderr: %s", args, code, stderr.String())
		}
		if slack.hits != 0 || discord.hits != 1 || ntfy.hits != 1 {
			t.Errorf("%v: hits slack=%d discord=%d ntfy=%d", args, slack.hits, discord.hits, ntfy.hits)
		}
	}
}

func TestRunReportsFailedSink(t *testing.T) {
	slack := newCapture(t, http.StatusInternalServerError, "oops")
	discord := newCapture(t, http.StatusNoContent, "")
	env := envMap(map[string]string{
		"NOTIFY_SLACK_WEBHOOK":   slack.URL,
		"NOTIFY_DISCORD_WEBHOOK": discord.URL,
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"hi"}, nil, &stdout, &stderr, env); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if discord.hits != 1 {
		t.Error("discord should still be sent to when slack fails")
	}
	if !strings.Contains(stderr.String(), "slack: HTTP 500") {
		t.Errorf("stderr %q does not name the failed sink", stderr.String())
	}
	if strings.Contains(stderr.String(), "discord") {
		t.Errorf("stderr %q mentions discord, which succeeded", stderr.String())
	}
}

func TestRunSinkErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--ntfy", "hi"}, "ntfy is not configured: set NOTIFY_NTFY_TOPIC or ntfy_topic in the config file"},
		{[]string{"--to", "sms", "hi"}, `unknown sink "sms"`},
		{[]string{"hi"}, "no sinks configured"},
		{[]string{"--pushover", "hi"}, "pushover is not configured: set NOTIFY_PUSHOVER_TOKEN"},
		{[]string{"-p", "urgent", "hi"}, `invalid priority "urgent"`},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, nil, &stdout, &stderr, envMap(nil)); code != 2 {
			t.Errorf("%v: exit %d, want 2", tt.args, code)
		}
		if !strings.Contains(stderr.String(), tt.want) {
			t.Errorf("%v: stderr %q, want %q", tt.args, stderr.String(), tt.want)
		}
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

func TestRunConfigDefault(t *testing.T) {
	slack := newCapture(t, http.StatusOK, "ok")
	discord := newCapture(t, http.StatusNoContent, "")
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "notify")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "default = discord\nslack_webhook = " + slack.URL + "\ndiscord_webhook = " + discord.URL + "\n"
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name                string
		env                 map[string]string
		args                []string
		wantSlack, wantDisc int
	}{
		{"config default", nil, nil, 0, 1},
		{"env overrides file", map[string]string{"NOTIFY_DEFAULT": "slack"}, nil, 1, 0},
		{"flag overrides env", map[string]string{"NOTIFY_DEFAULT": "slack"}, []string{"--to", "slack,discord"}, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slack.hits, discord.hits = 0, 0
			env := map[string]string{"HOME": home}
			maps.Copy(env, tt.env)
			var stdout, stderr bytes.Buffer
			args := append(tt.args, "hello")
			if code := run(args, nil, &stdout, &stderr, envMap(env)); code != 0 {
				t.Fatalf("exit %d, stderr: %s", code, stderr.String())
			}
			if slack.hits != tt.wantSlack || discord.hits != tt.wantDisc {
				t.Errorf("hits slack=%d discord=%d, want %d %d", slack.hits, discord.hits, tt.wantSlack, tt.wantDisc)
			}
		})
	}
}

func TestRunBadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("slack-webhook = x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-c", path, "hi"}, nil, &stdout, &stderr, envMap(nil)); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `:1: unknown key "slack-webhook"`) {
		t.Errorf("stderr %q", stderr.String())
	}
}
