package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// configKeys are the settings notify understands. Each one can be set in
// the config file or with an environment variable named NOTIFY_ followed
// by the key in upper case, e.g. NOTIFY_SLACK_WEBHOOK.
var configKeys = []string{
	"default",
	"ntfy_server",
	"ntfy_topic",
	"ntfy_token",
	"pushover_token",
	"pushover_user",
	"pushover_url",
	"slack_webhook",
	"discord_webhook",
}

// config maps setting keys to their values.
type config map[string]string

func envName(key string) string { return "NOTIFY_" + strings.ToUpper(key) }

// defaultConfigPath returns $XDG_CONFIG_HOME/notify/config, falling back
// to ~/.config/notify/config (on macOS too), or "" if neither is known.
func defaultConfigPath(getenv func(string) string) string {
	if dir := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "notify", "config")
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "notify", "config")
	}
	return ""
}

// loadConfig reads the config file at path, if there is one, and lets
// environment variables override it. A missing file is only an error when
// the path was asked for explicitly.
func loadConfig(path string, explicit bool, getenv func(string) string) (config, error) {
	c := config{}
	if path != "" {
		f, err := os.Open(path)
		switch {
		case err == nil:
			c, err = parseConfig(f, path)
			f.Close()
			if err != nil {
				return nil, err
			}
		case errors.Is(err, fs.ErrNotExist) && !explicit:
			// No config file is fine; everything can come from the environment.
		default:
			return nil, err
		}
	}
	for _, k := range configKeys {
		if v := getenv(envName(k)); v != "" {
			c[k] = v
		}
	}
	return c, nil
}

// parseConfig reads "key = value" lines. Blank lines and lines starting
// with # or ; are ignored, as is anything after a " #" in a value. Values
// may be quoted with " or ' to keep a # or surrounding spaces.
func parseConfig(r io.Reader, name string) (config, error) {
	c := config{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected key = value", name, n)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if !slices.Contains(configKeys, key) {
			return nil, fmt.Errorf("%s:%d: unknown key %q", name, n, key)
		}
		v, err := parseValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %s: %v", name, n, key, err)
		}
		c[key] = v
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return c, nil
}

func parseValue(s string) (string, error) {
	if s != "" && (s[0] == '"' || s[0] == '\'') {
		end := strings.IndexByte(s[1:], s[0])
		if end < 0 {
			return "", errors.New("missing closing quote")
		}
		if rest := strings.TrimSpace(s[end+2:]); rest != "" && rest[0] != '#' {
			return "", fmt.Errorf("unexpected %q after closing quote", rest)
		}
		return s[1 : end+1], nil
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return strings.TrimSpace(s[:i]), nil
		}
	}
	return s, nil
}
