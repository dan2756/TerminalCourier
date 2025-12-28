package main

import (
	"context"
	"net/http"
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
	// Publishing JSON to the server root, rather than a plain body with
	// a Title header, keeps non-ASCII titles intact.
	header := http.Header{}
	if n.Token != "" {
		header.Set("Authorization", "Bearer "+n.Token)
	}
	msg := ntfyMessage{
		Topic:    n.Topic,
		Message:  truncateBytes(m.Body, ntfyLimit),
		Title:    m.Title,
		Priority: ntfyPriority(m.Priority),
	}
	return postJSON(ctx, strings.TrimRight(n.Server, "/")+"/", header, msg)
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
