package main

import (
	"context"
	"net/url"
	"strconv"
)

const defaultPushoverURL = "https://api.pushover.net/1/messages.json"

// Pushover sends a push notification through the Pushover API.
type Pushover struct {
	Token string // application API token
	User  string // user or group key
	URL   string // API endpoint
}

func (p *Pushover) Name() string { return "pushover" }

func (p *Pushover) Send(ctx context.Context, m Message) error {
	form := url.Values{
		"token":   {p.Token},
		"user":    {p.User},
		"message": {m.Body},
	}
	if m.Title != "" {
		form.Set("title", m.Title)
	}
	// Pushover's own scale: -1 is quiet, 1 bypasses the user's quiet hours.
	if m.Priority != Normal {
		form.Set("priority", strconv.Itoa(int(m.Priority)))
	}
	return postForm(ctx, p.URL, form)
}
