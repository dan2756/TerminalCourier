// Command notify sends a message from stdin or its arguments to a
// notification service.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = ""

// timeout bounds every request to a notification service.
const timeout = 10 * time.Second

const usage = `Usage: notify [flags] [message...]

Send a message to Slack, Discord or your phone (via ntfy). The message is
taken from the arguments, or read from stdin when there are none:

  notify build finished
  make 2>&1 | tail -5 | notify --slack

Sinks (with none selected, every configured sink is used):
      --ntfy         push to an ntfy topic
      --slack        post to a Slack incoming webhook
      --discord      post to a Discord webhook
      --to LIST      comma-separated sinks, e.g. --to slack,discord

Flags:
  -t, --title TEXT   notification title
  -h, --help         show this help
      --version      print the version and exit

Environment:
  NOTIFY_NTFY_TOPIC        ntfy topic to publish to
  NOTIFY_NTFY_SERVER       ntfy server (default https://ntfy.sh)
  NOTIFY_NTFY_TOKEN        ntfy access token, for protected topics
  NOTIFY_SLACK_WEBHOOK     Slack incoming webhook URL
  NOTIFY_DISCORD_WEBHOOK   Discord webhook URL

Exit status is 0 when every sink succeeded, 1 when any failed, and 2 for
usage or configuration errors.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("notify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		title       string
		to          string
		showVersion bool
		selected    = map[string]*bool{}
	)
	fs.StringVar(&title, "t", "", "")
	fs.StringVar(&title, "title", "", "")
	fs.StringVar(&to, "to", "", "")
	fs.BoolVar(&showVersion, "version", false, "")
	for _, name := range sinkNames {
		selected[name] = fs.Bool(name, false, "")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "notify: %v\nRun 'notify --help' for usage.\n", err)
		return 2
	}
	if showVersion {
		fmt.Fprintln(stdout, "notify", versionString())
		return 0
	}

	names := splitList(to)
	for _, name := range sinkNames {
		if *selected[name] {
			names = append(names, name)
		}
	}
	sinks, err := selectSinks(names, configFromEnv(getenv))
	if err != nil {
		fmt.Fprintf(stderr, "notify: %v\n", err)
		return 2
	}

	body, err := readMessage(fs.Args(), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "notify: %v\n", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	code := 0
	for i, err := range sendAll(ctx, sinks, Message{Title: title, Body: body}) {
		if err != nil {
			fmt.Fprintf(stderr, "notify: %s: %v\n", sinks[i].Name(), err)
			code = 1
		}
	}
	return code
}

func versionString() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
