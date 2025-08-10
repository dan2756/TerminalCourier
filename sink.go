package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"
)

// A Sink delivers a message to one notification service.
type Sink interface {
	Name() string
	Send(ctx context.Context, m Message) error
}

// sinkNames lists every sink in the order they are reported.
var sinkNames = []string{"ntfy", "slack", "discord"}

// sinkKeys are the settings each sink needs. A sink counts as configured
// when any of them is set, so a half-finished setup is reported rather
// than silently skipped.
var sinkKeys = map[string][]string{
	"ntfy":    {"ntfy_topic"},
	"slack":   {"slack_webhook"},
	"discord": {"discord_webhook"},
}

func (c config) configured(name string) bool {
	for _, k := range sinkKeys[name] {
		if c[k] != "" {
			return true
		}
	}
	return false
}

// newSink builds the named sink from c, or explains what it is missing.
func (c config) newSink(name string) (Sink, error) {
	for _, k := range sinkKeys[name] {
		if c[k] == "" {
			return nil, fmt.Errorf("%s is not configured: set %s or %s in the config file", name, envName(k), k)
		}
	}
	switch name {
	case "ntfy":
		server := c["ntfy_server"]
		if server == "" {
			server = defaultNtfyServer
		}
		return &Ntfy{Server: server, Topic: c["ntfy_topic"], Token: c["ntfy_token"]}, nil
	case "slack":
		return &Slack{Webhook: c["slack_webhook"]}, nil
	case "discord":
		return &Discord{Webhook: c["discord_webhook"]}, nil
	}
	return nil, fmt.Errorf("unknown sink %q (want %s)", name, strings.Join(sinkNames, ", "))
}

// selectSinks resolves the sinks to send to: the ones named, else the
// configured default list, else every configured sink.
func selectSinks(names []string, c config) ([]Sink, error) {
	if len(names) == 0 {
		names = splitList(c["default"])
	}
	if len(names) == 0 {
		for _, n := range sinkNames {
			if c.configured(n) {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			return nil, errors.New("no sinks configured: set e.g. NOTIFY_SLACK_WEBHOOK, or see notify --help")
		}
	}
	var sinks []Sink
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		s, err := c.newSink(n)
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, s)
	}
	return sinks, nil
}

// splitList splits a sink list such as "slack, discord" or "slack ntfy".
func splitList(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

// sendAll sends m to every sink at once and returns each sink's error,
// in the same order as sinks.
func sendAll(ctx context.Context, sinks []Sink, m Message) []error {
	errs := make([]error, len(sinks))
	var wg sync.WaitGroup
	for i, s := range sinks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.Send(ctx, m)
		}()
	}
	wg.Wait()
	return errs
}
