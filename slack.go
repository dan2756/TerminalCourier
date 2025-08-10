package main

import (
	"context"
	"strings"
)

// slackEscaper escapes the three characters Slack treats as control
// sequences in message text.
var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// Slack posts to a Slack incoming webhook.
type Slack struct {
	Webhook string
}

type slackMessage struct {
	Text string `json:"text"`
}

func (s *Slack) Name() string { return "slack" }

func (s *Slack) Send(ctx context.Context, m Message) error {
	text := slackEscaper.Replace(m.Body)
	if m.Title != "" {
		text = "*" + slackEscaper.Replace(m.Title) + "*\n" + text
	}
	return postJSON(ctx, s.Webhook, nil, slackMessage{Text: text})
}
