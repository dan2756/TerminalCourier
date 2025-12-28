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
Sinks are sent to at once; one failing doesn't stop the others.

| Flag | |
| --- | --- |
| `--ntfy`, `--pushover`, `--slack`, `--discord`, `--mac` | send to that sink |
| `--to LIST` | comma-separated sinks |
| `-t`, `--title TEXT` | notification title |
| `-p`, `--priority P` | `low`, `normal` (default) or `high` |
| `-c`, `--config FILE` | read this config file instead |
| `--version` | print the version |

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
| ntfy | 4,096 bytes |
| desktop | 1,024 characters |

Pipe through `tail` to send the end of a long log rather than its start.

## Desktop notifications

`--mac` (or `desktop` in `--to` and `default`) needs no setup. On macOS it
runs `osascript -e 'display notification ...'`; the title defaults to
"notify". The first notification may ask for permission for Script Editor
in System Settings, Notifications.

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
