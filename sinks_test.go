package main

import (
	"context"
	"encoding/json"
	"net/http"
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
