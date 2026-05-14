package main

import "context"

// discordSuppressNotifications is the SUPPRESS_NOTIFICATIONS message flag:
// the message is posted without pushing a notification to anyone.
const discordSuppressNotifications = 1 << 12

// Discord posts to a Discord channel webhook.
type Discord struct {
	Webhook string
}

type discordMessage struct {
	Content         string          `json:"content"`
	Flags           int             `json:"flags,omitempty"`
	AllowedMentions allowedMentions `json:"allowed_mentions"`
}

type allowedMentions struct {
	Parse []string `json:"parse"`
}

func (d *Discord) Name() string { return "discord" }

func (d *Discord) Send(ctx context.Context, m Message) error {
	return sendRequest(ctx, d.request, m)
}

func (d *Discord) Preview(m Message) (string, error) {
	safe := &Discord{Webhook: redactURL(d.Webhook)}
	return previewRequest(safe.request, m)
}

func (d *Discord) request(m Message) (*request, error) {
	content := m.Body
	if m.Title != "" {
		content = "**" + m.Title + "**\n" + content
	}
	msg := discordMessage{
		Content: truncate(content, discordLimit),
		// Piped output may well contain "@everyone"; never ping anyone.
		AllowedMentions: allowedMentions{Parse: []string{}},
	}
	if m.Priority == Low {
		msg.Flags = discordSuppressNotifications
	}
	return jsonRequest(d.Webhook, msg)
}
