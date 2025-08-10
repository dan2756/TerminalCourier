package main

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	f, err := os.Open("testdata/config")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := parseConfig(f, "testdata/config")
	if err != nil {
		t.Fatal(err)
	}
	want := config{
		"default":         "slack, ntfy",
		"ntfy_topic":      "builds-7f3a",
		"ntfy_server":     "https://ntfy.example.com",
		"slack_webhook":   "https://hooks.slack.com/services/T000/B000/XXXX#frag",
		"discord_webhook": "https://discord.com/api/webhooks/1/abc",
	}
	if !maps.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestParseValue(t *testing.T) {
	tests := map[string]string{
		"plain":           "plain",
		"a#b":             "a#b",
		"a # comment":     "a",
		"a\t# comment":    "a",
		"# only comment":  "",
		"":                "",
		`"  spaced  "`:    "  spaced  ",
		`"has # hash"`:    "has # hash",
		`'single' # note`: "single",
		`""`:              "",
	}
	for in, want := range tests {
		got, err := parseValue(in)
		if err != nil {
			t.Errorf("parseValue(%q): %v", in, err)
		} else if got != want {
			t.Errorf("parseValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseConfigErrors(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"default = slack\nslack_webhook\n", "cfg:2: expected key = value"},
		{"# comment\n\nslak_webhook = x\n", `cfg:3: unknown key "slak_webhook"`},
		{`ntfy_topic = "builds`, "cfg:1: ntfy_topic: missing closing quote"},
		{`ntfy_topic = "a" b`, `cfg:1: ntfy_topic: unexpected "b" after closing quote`},
	}
	for _, tt := range tests {
		_, err := parseConfig(strings.NewReader(tt.in), "cfg")
		if err == nil || err.Error() != tt.want {
			t.Errorf("parseConfig(%q) error = %v, want %q", tt.in, err, tt.want)
		}
	}
}

func TestDefaultConfigPath(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"XDG_CONFIG_HOME": "/xdg", "HOME": "/home/me"}, "/xdg/notify/config"},
		{map[string]string{"XDG_CONFIG_HOME": "relative", "HOME": "/home/me"}, "/home/me/.config/notify/config"},
		{map[string]string{"HOME": "/home/me"}, "/home/me/.config/notify/config"},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := defaultConfigPath(envMap(tt.env)); got != filepath.FromSlash(tt.want) {
			t.Errorf("env %v: got %q, want %q", tt.env, got, tt.want)
		}
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("ntfy_topic = from-file\nslack_webhook = https://file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(path, true, envMap(map[string]string{"NOTIFY_NTFY_TOPIC": "from-env"}))
	if err != nil {
		t.Fatal(err)
	}
	if c["ntfy_topic"] != "from-env" || c["slack_webhook"] != "https://file" {
		t.Errorf("got %v", c)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope")
	if _, err := loadConfig(path, false, envMap(nil)); err != nil {
		t.Errorf("missing default config should be ignored, got %v", err)
	}
	if _, err := loadConfig(path, true, envMap(nil)); err == nil {
		t.Error("missing --config file should be an error")
	}
}
