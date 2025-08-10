package main

import "context"

// Discord posts to a Discord channel webhook.
type Discord struct {
	Webhook string
}

type discordMessage struct {
	Content         string          `json:"content"`
	AllowedMentions allowedMentions `json:"allowed_mentions"`
}

type allowedMentions struct {
	Parse []string `json:"parse"`
}

func (d *Discord) Name() string { return "discord" }

func (d *Discord) Send(ctx context.Context, m Message) error {
	content := m.Body
	if m.Title != "" {
		content = "**" + m.Title + "**\n" + content
	}
	msg := discordMessage{
		Content: content,
		// Piped output may well contain "@everyone"; never ping anyone.
		AllowedMentions: allowedMentions{Parse: []string{}},
	}
	return postJSON(ctx, d.Webhook, nil, msg)
}
