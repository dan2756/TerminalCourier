package main

import "strings"

// configKeys are the settings notify understands. Each one is read from an
// environment variable named NOTIFY_ followed by the key in upper case,
// e.g. NOTIFY_SLACK_WEBHOOK.
var configKeys = []string{
	"ntfy_server",
	"ntfy_topic",
	"ntfy_token",
	"slack_webhook",
	"discord_webhook",
}

// config maps setting keys to their values.
type config map[string]string

func envName(key string) string { return "NOTIFY_" + strings.ToUpper(key) }

// configFromEnv collects every setting present in the environment.
func configFromEnv(getenv func(string) string) config {
	c := config{}
	for _, k := range configKeys {
		if v := getenv(envName(k)); v != "" {
			c[k] = v
		}
	}
	return c
}
