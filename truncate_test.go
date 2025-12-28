package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"eleven chars", 10, "eleven ch…"},
		{"cut at space here", 8, "cut at…"},
		{"ünïcødé ✓✓✓", 8, "ünïcødé…"},
		{"abc", 1, "…"},
		{"abc", 0, ""},
		{"", 0, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.in, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
		if n := utf8.RuneCountInString(got); n > tt.max {
			t.Errorf("truncate(%q, %d) is %d characters long", tt.in, tt.max, n)
		}
	}
}

func TestTruncateBytes(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"abcdefghijk", 10, "abcdefg…"},
		// "é" is two bytes; cutting at byte 7 would split it.
		{"abcdeéfgh", 9, "abcde…"},
		{"✓✓✓✓", 9, "✓✓…"},
		{"abcdef", 2, ""},
	}
	for _, tt := range tests {
		got := truncateBytes(tt.in, tt.max)
		if got != tt.want {
			t.Errorf("truncateBytes(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
		if len(got) > tt.max || !utf8.ValidString(got) {
			t.Errorf("truncateBytes(%q, %d) = %q: too long or invalid UTF-8", tt.in, tt.max, got)
		}
	}
}

func TestSinksTruncateToLimit(t *testing.T) {
	long := strings.Repeat("ö", 50000)
	m := Message{Title: "CI", Body: long}
	srv := newCapture(t, http.StatusOK, "{}")
	ctx := context.Background()

	if err := (&Slack{Webhook: srv.URL}).Send(ctx, m); err != nil {
		t.Fatal(err)
	}
	var s slackMessage
	json.Unmarshal(srv.body, &s)
	if n := utf8.RuneCountInString(s.Text); n != slackLimit || !strings.HasSuffix(s.Text, ellipsis) {
		t.Errorf("slack text is %d characters", n)
	}

	if err := (&Discord{Webhook: srv.URL}).Send(ctx, m); err != nil {
		t.Fatal(err)
	}
	var d discordMessage
	json.Unmarshal(srv.body, &d)
	if n := utf8.RuneCountInString(d.Content); n != discordLimit || !strings.HasPrefix(d.Content, "**CI**\n") {
		t.Errorf("discord content is %d characters", n)
	}

	if err := (&Ntfy{Server: srv.URL, Topic: "x"}).Send(ctx, m); err != nil {
		t.Fatal(err)
	}
	var n ntfyMessage
	json.Unmarshal(srv.body, &n)
	if len(n.Message) > ntfyLimit || !strings.HasSuffix(n.Message, ellipsis) {
		t.Errorf("ntfy message is %d bytes", len(n.Message))
	}

	if err := (&Pushover{URL: srv.URL}).Send(ctx, Message{Title: long, Body: long}); err != nil {
		t.Fatal(err)
	}
	form, _ := url.ParseQuery(string(srv.body))
	if n := utf8.RuneCountInString(form.Get("message")); n != pushoverLimit {
		t.Errorf("pushover message is %d characters", n)
	}
	if n := utf8.RuneCountInString(form.Get("title")); n != pushoverTitleLimit {
		t.Errorf("pushover title is %d characters", n)
	}

	desk := newFakeDesktop()
	if err := desk.Send(ctx, m); err != nil {
		t.Fatal(err)
	}
	if body := desk.calls[0][len(desk.calls[0])-1]; utf8.RuneCountInString(body) != desktopLimit {
		t.Errorf("desktop body is %d characters", utf8.RuneCountInString(body))
	}
}
