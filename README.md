# notify

Send stdin (or a few words) to your phone, Slack, Discord or a desktop
notification when a long command finishes.

```sh
make 2>&1 | tail -5 | notify --slack -t "build"
```

## Install

```sh
go install github.com/dan2756/notify@latest
```

## Setup

notify reads `~/.config/notify/config` (or `$XDG_CONFIG_HOME/notify/config`),
a file of `key = value` lines:

```ini
# sinks to use when none are given on the command line
default = slack

ntfy_topic = builds-7f3a          # https://ntfy.sh/builds-7f3a
slack_webhook = https://hooks.slack.com/services/T000/B000/XXXX
discord_webhook = https://discord.com/api/webhooks/1234/abcd
```

| Key | |
| --- | --- |
| `default` | sinks used when none are selected, e.g. `slack, ntfy` |
| `ntfy_topic` | ntfy topic to publish to |
| `ntfy_server` | ntfy server, default `https://ntfy.sh` |
| `ntfy_token` | ntfy access token, for protected topics |
| `pushover_token` | Pushover application API token |
| `pushover_user` | Pushover user or group key |
| `pushover_url` | Pushover API endpoint, default `https://api.pushover.net/1/messages.json` |
| `slack_webhook` | Slack [incoming webhook](https://api.slack.com/messaging/webhooks) URL |
| `discord_webhook` | Discord webhook URL (channel settings, Integrations, Webhooks) |

Lines starting with `#` or `;` are comments, as is anything after ` #`.
Quote a value (`"..."` or `'...'`) to keep a `#` in it.

Every key can also be set in the environment as `NOTIFY_` plus the key in
upper case (`NOTIFY_SLACK_WEBHOOK`, `NOTIFY_NTFY_TOPIC`, `NOTIFY_DEFAULT`,
...). Environment variables override the file, and command-line flags
override both.

## Usage

Pass the message as arguments, or pipe it in:

```sh
notify deploy finished
./long-job.sh 2>&1 | tail -20 | notify -t "long-job"
notify --to slack,ntfy "backup done"
```

With no sink selected, notify uses `default`, or else every configured
sink, or else (with nothing configured at all) a desktop notification.
Sinks are sent to at once, each with a 10 second timeout; one failing
doesn't stop the others.

Flags go before the message. Use `--` if the message itself starts with a
dash: `notify -t cron -- --dry-run finished`.

| Flag | |
| --- | --- |
| `--ntfy`, `--pushover`, `--slack`, `--discord`, `--mac` | send to that sink |
| `--to LIST` | comma-separated sinks |
| `-t`, `--title TEXT` | notification title |
| `-p`, `--priority P` | `low`, `normal` (default) or `high` |
| `--tee` | copy stdin to stdout unchanged, so notify can sit in the middle of a pipe |
| `-n`, `--dry-run` | print each request instead of sending it, secrets redacted |
| `-c`, `--config FILE` | read this config file instead |
| `--version` | print the version |

### In the middle of a pipe

`--tee` passes stdin through to stdout untouched, as it arrives, and sends
the notification once stdin closes. The message is stdin itself, or the
arguments if there are any:

```sh
# see the output live, keep a log, and get it on Slack as well
make 2>&1 | notify --tee --slack -t make | tee build.log

# get "backup finished" when rsync is done, with its output still shown
rsync -av src/ dst/ | notify --tee --pushover "backup finished"
```

If the reader goes away early (`| head`), notify keeps reading so the
notification is still sent, and if notify can't send at all (no sinks
configured, say) it still passes everything through. With `--tee`,
`--dry-run`, `--help` and `--version` write to stderr.

### Dry run

`--dry-run` shows exactly what would be sent where, without sending it.
Webhook paths, tokens and the ntfy topic are replaced with `REDACTED`, so
the output is safe to paste into an issue:

```console
$ notify -n -t CI --to slack,mac "tests <passed>"
--- slack ---
POST https://hooks.slack.com/REDACTED
Content-Type: application/json

{"text":"*CI*\ntests &lt;passed&gt;"}

--- mac ---
osascript -e 'display notification "tests <passed>" with title "CI"'
```

### Priority

| | low | high |
| --- | --- | --- |
| ntfy | priority 2 (no sound) | priority 4 (long vibration, pop-over) |
| Pushover | -1 (quiet) | 1 (bypasses quiet hours) |
| Discord | posted without a notification | - |
| Slack | - | - |
| macOS | - | plays a sound |
| notify-send | `--urgency=low` | `--urgency=critical` |

### Long messages

Each service has a size limit, and notify cuts longer messages short with
an ellipsis instead of having them rejected:

| Sink | Limit |
| --- | --- |
| Slack | 40,000 characters |
| Discord | 2,000 characters (title included) |
| Pushover | 1,024 characters, title 250 |
| ntfy | 4,096 bytes, title 1,024 bytes |
| desktop | 1,024 characters |

Pipe through `tail` to send the end of a long log rather than its start.

### Formatting

The title is shown as a bold first line on Slack and Discord and as the
notification title elsewhere. On Slack, `&`, `<` and `>` are escaped so
command output shows up as is rather than as links or mentions; on
Discord, `@everyone` and other mentions in the output never ping anyone.

## Desktop notifications

`--mac` (or `desktop` in `--to` and `default`) needs no setup. On macOS it
runs `osascript -e 'display notification ...'`, with the message escaped
as an AppleScript string; the title defaults to "notify". macOS shows
these as coming from Script Editor, so if nothing appears, check that
Script Editor is allowed in System Settings, Notifications.

On Linux it uses `notify-send` from libnotify (`libnotify-bin` on Debian
and Ubuntu, `libnotify` elsewhere) and fails with a clear error if that
isn't installed.

## Exit status

| Code | Meaning |
| --- | --- |
| 0 | every sink accepted the message |
| 1 | at least one sink failed (each failure is printed to stderr) |
| 2 | usage or configuration error; nothing was sent |

## License

MIT
