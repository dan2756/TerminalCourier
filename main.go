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

Send a message to a Slack incoming webhook. The message is taken from the
arguments, or read from stdin when there are none:

  notify build finished
  make 2>&1 | tail -5 | notify

Flags:
  -t, --title TEXT   notification title
  -h, --help         show this help
      --version      print the version and exit

Environment:
  NOTIFY_SLACK_WEBHOOK   Slack incoming webhook URL
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("notify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		title       string
		showVersion bool
	)
	fs.StringVar(&title, "t", "", "")
	fs.StringVar(&title, "title", "", "")
	fs.BoolVar(&showVersion, "version", false, "")
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

	webhook := getenv("NOTIFY_SLACK_WEBHOOK")
	if webhook == "" {
		fmt.Fprintln(stderr, "notify: NOTIFY_SLACK_WEBHOOK is not set")
		return 2
	}

	body, err := readMessage(fs.Args(), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "notify: %v\n", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	slack := &Slack{Webhook: webhook}
	if err := slack.Send(ctx, Message{Title: title, Body: body}); err != nil {
		fmt.Fprintf(stderr, "notify: slack: %v\n", err)
		return 1
	}
	return 0
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
