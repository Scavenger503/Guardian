# Guardian 🛡️

<img width="1024" height="1024" alt="Guardian-Logo" src="https://github.com/user-attachments/assets/7704e7ce-fa40-4aa4-8c70-6a8c727e3db4" />


A lightweight, self-hosted Docker container update watcher written in Go. Guardian monitors labeled containers, detects new image versions via digest comparison, and automatically pulls and restarts updated containers — with Telegram notifications at every step.

## Why Guardian?

Watchtower was archived in December 2025. Guardian is a clean, minimal replacement built for homelabs that care about security and control.

## Features

- **Label opt-in** — only watches containers you explicitly mark
- **Digest comparison** — compares SHA digests, not tags, so it only acts on real changes
- *0*Telegram notifications** — alerts on update detected, applied, or failed
- **Self-disabling** — ships with `guardian.disable=true` so it never updates itself
- **Tiny footprint** — ~10MB Docker image, single Go binary, no runtime dependencies

## Quick Start

```yaml
services:
  guardian:
    image: ghcr.io/scavenger503/guardian:latest
    container_name: guardian
    restart: unless-stopped
    environment:
      - TELEGRAM_TOKEN=your_bot_token
      - TELEGRAM_CHAT_ID=your_chat_id
      - POLL_INTERVAL=300
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    labels:
      - guardian.disable=true
```

## Opting Containers In

Add this label to any container you want Guardian to watch:

```yaml
labels:
  - guardian.enable=true
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `TELEGRAM_TOKEN` | — | Telegram bot token |
| `TELEGRAM_CHAT_ID` | — | Telegram chat/group ID |
| `POLL_INTERVAL` | `300` | Check interval in seconds |

## How It Works

1. Guardian polls Docker every `POLL_INTERVAL` seconds
2. Finds all containers with `guardian.enable=true`
3. Compares the local image digest against the remote registry digest
4. If a new digest is found — pulls the new image, stops the old container, recreates it with the same config, starts it
5. Sends Telegram notification at each step

## Building from Source

```bash
git clone https://github.com/Scavenger503/Guardian.git
cd Guardian
go build -o guardian cmd/guardian/main.go
```

## License

MIT
