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
	Topic   string `json:"topic"`
	Message string `json:"message"`
	Title   string `json:"title,omitempty"`
}

func (n *Ntfy) Name() string { return "ntfy" }

func (n *Ntfy) Send(ctx context.Context, m Message) error {
	// Publishing JSON to the server root, rather than a plain body with
	// a Title header, keeps non-ASCII titles intact.
	header := http.Header{}
	if n.Token != "" {
		header.Set("Authorization", "Bearer "+n.Token)
	}
	msg := ntfyMessage{Topic: n.Topic, Message: m.Body, Title: m.Title}
	return postJSON(ctx, strings.TrimRight(n.Server, "/")+"/", header, msg)
}
