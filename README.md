# notify

Send stdin (or a few words) to Slack, Discord or your phone when a long
command finishes.

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
sink. Sinks are sent to at once; one failing doesn't stop the others.

| Flag | |
| --- | --- |
| `--ntfy`, `--slack`, `--discord` | send to that sink |
| `--to LIST` | comma-separated sinks |
| `-t`, `--title TEXT` | notification title |
| `-c`, `--config FILE` | read this config file instead |
| `--version` | print the version |

## Exit status

| Code | Meaning |
| --- | --- |
| 0 | every sink accepted the message |
| 1 | at least one sink failed (each failure is printed to stderr) |
| 2 | usage or configuration error; nothing was sent |

## License

MIT
