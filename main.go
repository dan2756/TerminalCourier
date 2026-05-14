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

Send a message to your phone (via ntfy or Pushover), Slack, Discord or
the desktop. The message is taken from the arguments, or read from stdin
when there are none:

  notify build finished
  make 2>&1 | tail -5 | notify --slack

Sinks:
      --ntfy         push to an ntfy topic
      --pushover     push through Pushover
      --slack        post to a Slack incoming webhook
      --discord      post to a Discord webhook
      --mac          show a desktop notification (notify-send on Linux)
      --to LIST      comma-separated sinks, e.g. --to slack,mac

With no sink selected, notify uses the "default" list from the config,
else every configured sink, else a desktop notification.

Flags:
  -t, --title TEXT    notification title
  -p, --priority P    low, normal (default) or high
  -n, --dry-run       print what would be sent, with secrets redacted
  -c, --config FILE   config file (default ~/.config/notify/config)
  -h, --help          show this help
      --version       print the version and exit

Configuration is read from $XDG_CONFIG_HOME/notify/config (or
~/.config/notify/config) as "key = value" lines. Every key can also be set
in the environment as NOTIFY_<KEY>, which overrides the file:

  default           sinks to use when none are selected, e.g. slack,ntfy
  ntfy_topic        ntfy topic to publish to
  ntfy_server       ntfy server (default https://ntfy.sh)
  ntfy_token        ntfy access token, for protected topics
  pushover_token    Pushover application API token
  pushover_user     Pushover user or group key
  slack_webhook     Slack incoming webhook URL
  discord_webhook   Discord webhook URL

Exit status is 0 when every sink succeeded, 1 when any failed, and 2 for
usage or configuration errors.
`

// app is everything notify needs from the outside world, so tests can
// substitute their own.
type app struct {
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	getenv  func(string) string
	desktop *Desktop
}

func main() {
	a := &app{
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		getenv:  os.Getenv,
		desktop: newDesktop(),
	}
	os.Exit(a.run(os.Args[1:]))
}

func (a *app) run(args []string) int {
	stdout, stderr := a.stdout, a.stderr
	fs := flag.NewFlagSet("notify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		title       string
		priority    string
		to          string
		configPath  string
		dryRun      bool
		showVersion bool
		selected    = map[string]*bool{}
	)
	fs.StringVar(&title, "t", "", "")
	fs.StringVar(&title, "title", "", "")
	fs.StringVar(&priority, "p", "", "")
	fs.StringVar(&priority, "priority", "", "")
	fs.StringVar(&to, "to", "", "")
	fs.StringVar(&configPath, "c", "", "")
	fs.StringVar(&configPath, "config", "", "")
	fs.BoolVar(&dryRun, "n", false, "")
	fs.BoolVar(&dryRun, "dry-run", false, "")
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

	prio, err := parsePriority(priority)
	if err != nil {
		fmt.Fprintf(stderr, "notify: %v\n", err)
		return 2
	}

	explicit := configPath != ""
	if !explicit {
		configPath = defaultConfigPath(a.getenv)
	}
	cfg, err := loadConfig(configPath, explicit, a.getenv)
	if err != nil {
		fmt.Fprintf(stderr, "notify: %v\n", err)
		return 2
	}

	names := splitList(to)
	for _, name := range sinkNames {
		if *selected[name] {
			names = append(names, name)
		}
	}
	sinks, err := selectSinks(names, cfg, a.desktop)
	if err != nil {
		fmt.Fprintf(stderr, "notify: %v\n", err)
		return 2
	}

	body, err := readMessage(fs.Args(), a.stdin)
	if err != nil {
		fmt.Fprintf(stderr, "notify: %v\n", err)
		return 2
	}

	msg := Message{Title: title, Body: body, Priority: prio}
	if dryRun {
		return a.preview(sinks, msg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	code := 0
	for i, err := range sendAll(ctx, sinks, msg) {
		if err != nil {
			fmt.Fprintf(stderr, "notify: %s: %v\n", sinks[i].Name(), err)
			code = 1
		}
	}
	return code
}

// preview prints what each sink would send, without sending anything.
func (a *app) preview(sinks []Sink, m Message) int {
	code := 0
	for i, s := range sinks {
		p, err := s.Preview(m)
		if err != nil {
			fmt.Fprintf(a.stderr, "notify: %s: %v\n", s.Name(), err)
			code = 1
			continue
		}
		if i > 0 {
			fmt.Fprintln(a.stdout)
		}
		fmt.Fprintf(a.stdout, "--- %s ---\n%s", s.Name(), p)
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
