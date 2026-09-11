package main

import (
	"context"
	"strings"
	"unicode"
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
	return sendRequest(ctx, s.request, m)
}

func (s *Slack) Preview(m Message) (string, error) {
	safe := &Slack{Webhook: redactURL(s.Webhook)}
	return previewRequest(safe.request, m)
}

func (s *Slack) request(m Message) (*request, error) {
	text := slackEscaper.Replace(m.Body)
	if m.Title != "" {
		text = "*" + slackEscaper.Replace(m.Title) + "*\n" + text
	}
	return jsonRequest(s.Webhook, slackMessage{Text: slackTruncate(text, slackLimit)})
}

// slackTruncate is truncate for escaped Slack text. It never leaves half
// of an &amp;, &lt; or &gt; at the cut, which Slack would show as is.
func slackTruncate(s string, max int) string {
	t := truncate(s, max)
	if t == s {
		return s
	}
	kept := strings.TrimSuffix(t, ellipsis)
	// Every & in escaped text starts an entity, so one without a closing
	// ; was cut short.
	if i := strings.LastIndexByte(kept, '&'); i >= 0 && !strings.Contains(kept[i:], ";") {
		kept = strings.TrimRightFunc(kept[:i], unicode.IsSpace)
	}
	return kept + ellipsis
}
