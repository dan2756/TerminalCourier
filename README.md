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

Configure one or more sinks with environment variables:

| Sink | Variables |
| --- | --- |
| `ntfy` | `NOTIFY_NTFY_TOPIC`, optional `NOTIFY_NTFY_SERVER` (default `https://ntfy.sh`) and `NOTIFY_NTFY_TOKEN` |
| `slack` | `NOTIFY_SLACK_WEBHOOK` ([incoming webhook](https://api.slack.com/messaging/webhooks)) |
| `discord` | `NOTIFY_DISCORD_WEBHOOK` (channel settings, Integrations, Webhooks) |

## Usage

Pass the message as arguments, or pipe it in:

```sh
notify deploy finished
./long-job.sh 2>&1 | tail -20 | notify -t "long-job"
notify --to slack,ntfy "backup done"
```

With no sink selected, notify sends to every configured sink at once.

| Flag | |
| --- | --- |
| `--ntfy`, `--slack`, `--discord` | send to that sink |
| `--to LIST` | comma-separated sinks |
| `-t`, `--title TEXT` | notification title |
| `--version` | print the version |

## Exit status

| Code | Meaning |
| --- | --- |
| 0 | every sink accepted the message |
| 1 | at least one sink failed (each failure is printed to stderr) |
| 2 | usage or configuration error; nothing was sent |

## License

MIT
