package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestRunDryRun(t *testing.T) {
	srv := newCapture(t, http.StatusOK, "ok")
	secrets := map[string]string{
		"NOTIFY_NTFY_SERVER":     "https://alice:hunter2@ntfy.example.com",
		"NOTIFY_NTFY_TOPIC":      "topic-s3cret",
		"NOTIFY_NTFY_TOKEN":      "tk_s3cret",
		"NOTIFY_PUSHOVER_TOKEN":  "app-s3cret",
		"NOTIFY_PUSHOVER_USER":   "user-s3cret",
		"NOTIFY_PUSHOVER_URL":    srv.URL,
		"NOTIFY_SLACK_WEBHOOK":   srv.URL + "/services/T0/B0/hook-s3cret",
		"NOTIFY_DISCORD_WEBHOOK": srv.URL + "/api/webhooks/1/disc-s3cret",
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--dry-run", "-t", "CI", "-p", "high"}, strings.NewReader("all <green>\n"), &stdout, &stderr, envMap(secrets))
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if srv.hits != 0 {
		t.Errorf("--dry-run sent %d requests", srv.hits)
	}
	out := stdout.String()
	for _, secret := range []string{"hunter2", "s3cret"} {
		if strings.Contains(out, secret) {
			t.Errorf("output leaks %q:\n%s", secret, out)
		}
	}
	for _, want := range []string{
		"--- ntfy ---\nPOST https://alice:xxxxx@ntfy.example.com/\n",
		"Authorization: Bearer REDACTED\n",
		`{"topic":"REDACTED","message":"all <green>","title":"CI","priority":4}`,
		"--- pushover ---\nPOST " + srv.URL + "\n",
		"message=all+%3Cgreen%3E&priority=1&title=CI&token=REDACTED&user=REDACTED",
		"--- slack ---\nPOST " + srv.URL + "/REDACTED\nContent-Type: application/json\n\n" +
			`{"text":"*CI*\nall &lt;green&gt;"}` + "\n",
		"--- discord ---\nPOST " + srv.URL + "/REDACTED\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

func TestPreviewDesktop(t *testing.T) {
	d := newFakeDesktop()
	d.goos = "darwin"
	got, err := d.Preview(Message{Title: "it's done", Body: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	want := `osascript -e 'display notification "ok" with title "it'\''s done"'` + "\n"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if len(d.calls) != 0 {
		t.Errorf("Preview ran %v", d.calls)
	}
}

func TestRedactURL(t *testing.T) {
	tests := map[string]string{
		"https://hooks.slack.com/services/T0/B0/XXXX":      "https://hooks.slack.com/REDACTED",
		"https://discord.com/api/webhooks/1/tok?wait=true": "https://discord.com/REDACTED",
		"https://user:pass@example.com/hook":               "https://example.com/REDACTED",
		"not a url":                                        "REDACTED",
		"hooks.slack.com/services/T0/B0/XXXX":              "REDACTED",
	}
	for in, want := range tests {
		if got := redactURL(in); got != want {
			t.Errorf("redactURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShellJoin(t *testing.T) {
	got := shellJoin([]string{"notify-send", "--urgency=low", "--", "", "it's", "a b", "ok"})
	want := `notify-send --urgency=low -- '' 'it'\''s' 'a b' ok`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
