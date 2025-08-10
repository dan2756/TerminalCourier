package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNtfy(t *testing.T) {
	srv := newCapture(t, http.StatusOK, `{"id":"abc"}`)
	n := &Ntfy{Server: srv.URL + "/", Topic: "builds", Token: "tk_secret"}
	if err := n.Send(context.Background(), Message{Title: "Bau ✓", Body: "fertig"}); err != nil {
		t.Fatal(err)
	}
	if srv.path != "/" {
		t.Errorf("posted to %q, want the server root", srv.path)
	}
	if got := srv.header.Get("Authorization"); got != "Bearer tk_secret" {
		t.Errorf("Authorization = %q", got)
	}
	var got ntfyMessage
	if err := json.Unmarshal(srv.body, &got); err != nil {
		t.Fatal(err)
	}
	want := ntfyMessage{Topic: "builds", Message: "fertig", Title: "Bau ✓"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestNtfyNoToken(t *testing.T) {
	srv := newCapture(t, http.StatusOK, "{}")
	n := &Ntfy{Server: srv.URL, Topic: "builds"}
	if err := n.Send(context.Background(), Message{Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	if got := srv.header.Get("Authorization"); got != "" {
		t.Errorf("unexpected Authorization header %q", got)
	}
}

func TestNtfyError(t *testing.T) {
	srv := newCapture(t, http.StatusForbidden, `{"code":40301,"http":403,"error":"forbidden"}`)
	err := (&Ntfy{Server: srv.URL, Topic: "x"}).Send(context.Background(), Message{Body: "hi"})
	if err == nil || err.Error() != "HTTP 403 Forbidden: forbidden" {
		t.Errorf("err = %v", err)
	}
}

func TestDiscord(t *testing.T) {
	srv := newCapture(t, http.StatusNoContent, "")
	d := &Discord{Webhook: srv.URL + "/api/webhooks/1/tok"}
	if err := d.Send(context.Background(), Message{Title: "CI", Body: "@everyone tests passed"}); err != nil {
		t.Fatal(err)
	}
	if srv.path != "/api/webhooks/1/tok" {
		t.Errorf("posted to %q", srv.path)
	}
	body := string(srv.body)
	if !strings.Contains(body, `"content":"**CI**\n@everyone tests passed"`) {
		t.Errorf("body %s", body)
	}
	if !strings.Contains(body, `"allowed_mentions":{"parse":[]}`) {
		t.Errorf("body %s does not disable mentions", body)
	}
}

func TestDiscordError(t *testing.T) {
	srv := newCapture(t, http.StatusNotFound, `{"message": "Unknown Webhook", "code": 10015}`)
	err := (&Discord{Webhook: srv.URL}).Send(context.Background(), Message{Body: "hi"})
	if err == nil || err.Error() != "HTTP 404 Not Found: Unknown Webhook" {
		t.Errorf("err = %v", err)
	}
}

func TestPushover(t *testing.T) {
	srv := newCapture(t, http.StatusOK, `{"status":1,"request":"abc"}`)
	p := &Pushover{Token: "app-token", User: "user-key", URL: srv.URL + "/1/messages.json"}
	if err := p.Send(context.Background(), Message{Title: "backup", Body: "done & dusted", Priority: High}); err != nil {
		t.Fatal(err)
	}
	if srv.path != "/1/messages.json" {
		t.Errorf("posted to %q", srv.path)
	}
	if ct := srv.header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", ct)
	}
	form, err := url.ParseQuery(string(srv.body))
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{
		"token":    {"app-token"},
		"user":     {"user-key"},
		"message":  {"done & dusted"},
		"title":    {"backup"},
		"priority": {"1"},
	}
	if form.Encode() != want.Encode() {
		t.Errorf("form = %v, want %v", form, want)
	}
}

func TestPushoverError(t *testing.T) {
	srv := newCapture(t, http.StatusBadRequest, `{"user":"invalid","errors":["user identifier is invalid"],"status":0}`)
	err := (&Pushover{Token: "t", User: "u", URL: srv.URL}).Send(context.Background(), Message{Body: "hi"})
	if err == nil || err.Error() != "HTTP 400 Bad Request: user identifier is invalid" {
		t.Errorf("err = %v", err)
	}
}

func TestParsePriority(t *testing.T) {
	for in, want := range map[string]Priority{"": Normal, "normal": Normal, "LOW": Low, " high ": High} {
		got, err := parsePriority(in)
		if err != nil || got != want {
			t.Errorf("parsePriority(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parsePriority("urgent"); err == nil {
		t.Error("parsePriority(urgent) should fail")
	}
}

func TestPriorityMapping(t *testing.T) {
	for _, tt := range []struct {
		p               Priority
		ntfy, flags     int
		pushover        string
		pushoverPresent bool
	}{
		{Low, 2, discordSuppressNotifications, "-1", true},
		{Normal, 0, 0, "", false},
		{High, 4, 0, "1", true},
	} {
		m := Message{Body: "hi", Priority: tt.p}

		srv := newCapture(t, http.StatusOK, "{}")
		if err := (&Ntfy{Server: srv.URL, Topic: "x"}).Send(context.Background(), m); err != nil {
			t.Fatal(err)
		}
		var n ntfyMessage
		json.Unmarshal(srv.body, &n)
		if n.Priority != tt.ntfy {
			t.Errorf("%v: ntfy priority %d, want %d", tt.p, n.Priority, tt.ntfy)
		}

		if err := (&Discord{Webhook: srv.URL}).Send(context.Background(), m); err != nil {
			t.Fatal(err)
		}
		var d discordMessage
		json.Unmarshal(srv.body, &d)
		if d.Flags != tt.flags {
			t.Errorf("%v: discord flags %d, want %d", tt.p, d.Flags, tt.flags)
		}

		if err := (&Pushover{URL: srv.URL}).Send(context.Background(), m); err != nil {
			t.Fatal(err)
		}
		form, _ := url.ParseQuery(string(srv.body))
		if got, ok := form["priority"]; ok != tt.pushoverPresent || (ok && got[0] != tt.pushover) {
			t.Errorf("%v: pushover priority %v, want %q", tt.p, got, tt.pushover)
		}
	}
}
