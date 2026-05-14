package main

import (
	"context"
	"net/url"
	"strings"
)

const defaultNtfyServer = "https://ntfy.sh"

// Ntfy publishes to a topic on ntfy.sh or a self-hosted ntfy server.
type Ntfy struct {
	Server string
	Topic  string
	Token  string // access token, for protected topics
}

type ntfyMessage struct {
	Topic    string `json:"topic"`
	Message  string `json:"message"`
	Title    string `json:"title,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

func (n *Ntfy) Name() string { return "ntfy" }

func (n *Ntfy) Send(ctx context.Context, m Message) error {
	return sendRequest(ctx, n.request, m)
}

// Preview hides the topic as well as the token: on a public server, anyone
// who knows the topic can read it.
func (n *Ntfy) Preview(m Message) (string, error) {
	safe := &Ntfy{Server: n.Server, Topic: redacted}
	if u, err := url.Parse(n.Server); err == nil {
		safe.Server = u.Redacted()
	}
	if n.Token != "" {
		safe.Token = redacted
	}
	return previewRequest(safe.request, m)
}

func (n *Ntfy) request(m Message) (*request, error) {
	msg := ntfyMessage{
		Topic:    n.Topic,
		Message:  truncateBytes(m.Body, ntfyLimit),
		Title:    m.Title,
		Priority: ntfyPriority(m.Priority),
	}
	// Publishing JSON to the server root, rather than a plain body with
	// a Title header, keeps non-ASCII titles intact.
	r, err := jsonRequest(strings.TrimRight(n.Server, "/")+"/", msg)
	if err != nil {
		return nil, err
	}
	if n.Token != "" {
		r.header.Set("Authorization", "Bearer "+n.Token)
	}
	return r, nil
}

// ntfyPriority maps onto ntfy's scale of 1 (min) to 5 (max). Zero is
// left out of the request, so the server uses its default of 3.
func ntfyPriority(p Priority) int {
	switch p {
	case Low:
		return 2
	case High:
		return 4
	}
	return 0
}
