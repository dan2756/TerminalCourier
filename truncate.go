package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Size limits of each service. Longer messages are cut short with an
// ellipsis rather than rejected (Discord, Pushover) or turned into an
// attachment (ntfy).
const (
	slackLimit         = 40000 // characters of text
	discordLimit       = 2000  // characters of content
	pushoverLimit      = 1024  // characters of message
	pushoverTitleLimit = 250   // characters of title
	ntfyLimit          = 4096  // bytes of message
	desktopLimit       = 1024  // characters; also keeps argv to a sane size
)

const ellipsis = "…"

// truncate shortens s to at most max characters, ending in an ellipsis
// when anything was cut.
func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	if max < 1 {
		return ""
	}
	n := 0
	for i := range s {
		if n == max-1 {
			return strings.TrimRightFunc(s[:i], unicode.IsSpace) + ellipsis
		}
		n++
	}
	return s
}

// truncateBytes is truncate for limits measured in bytes. It never splits
// a multi-byte character.
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max - len(ellipsis)
	if cut < 0 {
		return ""
	}
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimRightFunc(s[:cut], unicode.IsSpace) + ellipsis
}
