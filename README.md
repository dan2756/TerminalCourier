# notify

Send stdin (or a few words) to Slack when a long command finishes.

```sh
make 2>&1 | tail -5 | notify -t "build"
```

## Install

```sh
go install github.com/dan2756/notify@latest
```

## Usage

Create a [Slack incoming webhook](https://api.slack.com/messaging/webhooks)
and export it:

```sh
export NOTIFY_SLACK_WEBHOOK=https://hooks.slack.com/services/T000/B000/XXXX
```

Then pass the message as arguments, or pipe it in:

```sh
notify deploy finished
./long-job.sh 2>&1 | tail -20 | notify -t "long-job"
```

| Flag | |
| --- | --- |
| `-t`, `--title TEXT` | bold title line above the message |
| `--version` | print the version |

notify exits 0 when the message was delivered, 1 when Slack rejected it,
and 2 for usage errors.

## License

MIT
