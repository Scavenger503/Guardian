# Guardian

**A self-hosted Docker container image update watcher.**  
Built as a secure, minimal replacement for Watchtower.

---

## Features

- Watches running Docker containers for image updates
- Label-based opt-in per container — you control what gets updated
- Dry-run / monitor-only mode
- Telegram notifications (update detected, applied, or failed)
- Single static binary — tiny Docker image, minimal attack surface
- Runs as a non-root user inside the container
- Graceful shutdown on SIGTERM (Docker stop friendly)

---

## Quick Start

```bash
git clone https://github.com/scavengervhs/guardian
cd guardian
cp deploy/docker-compose.yml docker-compose.yml
# Edit docker-compose.yml to add your Telegram token if desired
docker compose up -d
```

Then add the label to any container you want watched:

```yaml
labels:
  guardian.enable: "true"
```

---

## Configuration

All configuration is via environment variables.

| Variable | Default | Description |
|---|---|---|
| `GUARDIAN_POLL_INTERVAL` | `5m` | How often to check for updates (e.g. `30s`, `5m`, `1h`) |
| `GUARDIAN_DRY_RUN` | `false` | Detect updates but don't apply them |
| `GUARDIAN_LOG_LEVEL` | `info` | Log verbosity: `info` or `debug` |
| `GUARDIAN_TELEGRAM_TOKEN` | _(empty)_ | Telegram bot token — leave empty to disable |
| `GUARDIAN_TELEGRAM_CHAT_ID` | _(empty)_ | Telegram chat ID to send alerts to |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon socket/host |

---

## Container Labels

Guardian uses labels to decide which containers to manage.

| Label | Value | Effect |
|---|---|---|
| `guardian.enable` | `true` | Opt this container INTO Guardian monitoring |
| `guardian.disable` | `true` | Opt this container OUT (overrides enable) |

Guardian itself ships with `guardian.disable: "true"` — do not self-update.

---

## Building Locally

```bash
go build -o guardian ./cmd/guardian
```

Or with Docker:

```bash
docker build -t guardian:local .
```

---

## Project Structure

```
guardian/
├── cmd/
│   └── guardian/
│       └── main.go          # Entrypoint
├── internal/
│   ├── config/
│   │   └── config.go        # Environment variable config loader
│   ├── notifier/
│   │   └── notifier.go      # Telegram notification client
│   └── watcher/
│       └── watcher.go       # Core poll loop and update engine
├── deploy/
│   └── docker-compose.yml   # Ready-to-use Compose file
├── Dockerfile                # Multi-stage build
├── go.mod
└── README.md
```

---

## Telegram Setup

1. Create a bot via [@BotFather](https://t.me/botfather) and grab the token
2. Get your chat ID (send a message to your bot, then check `https://api.telegram.org/bot<TOKEN>/getUpdates`)
3. Set `GUARDIAN_TELEGRAM_TOKEN` and `GUARDIAN_TELEGRAM_CHAT_ID` in your compose file

---

## Security Considerations

Guardian requires access to the Docker socket (`/var/run/docker.sock`), which carries inherent risk. This section documents those risks transparently so operators can make informed decisions.

### The Docker Socket

Access to `/var/run/docker.sock` is effectively **root on the host**. Any process with socket access can spawn privileged containers, mount the host filesystem, and escape to the underlying OS. This is not unique to Guardian — it applies to any Docker management tool (Watchtower, Portainer, etc.).

**Mitigations built into Guardian:**
- Runs as a **non-root user** inside the container
- **Label opt-in only** — Guardian never touches a container unless it has `guardian.enable: "true"` explicitly set
- **Digest-based comparison** — compares SHA digests, not tags, reducing exposure to tag-hijacking attacks
- **Minimal final image** — Alpine base with only the compiled binary and CA certificates; no shell, no package manager in production

### Supply Chain Risk

Guardian automatically pulls images from public registries. A compromised upstream image (Docker Hub, GHCR) could execute malicious code on your host if that image's container is opted into Guardian.

**Mitigations:**
- Only opt in containers from registries and publishers you trust
- Pin critical infrastructure containers (Wazuh, reverse proxies, security tooling) with `guardian.disable: "true"`
- Consider enabling Docker Content Trust (`DOCKER_CONTENT_TRUST=1`) for signed image verification

### Recommended Deployment Posture

| Practice | Why |
|---|---|
| Run Guardian on internal networks only | Reduces attack surface; never expose the Docker socket to the internet |
| Use `guardian.disable: "true"` on security-critical containers | Wazuh, Falco, Guardian itself should never be auto-updated |
| Set `GUARDIAN_DRY_RUN=true` initially | Validate behavior before allowing Guardian to restart containers |
| Rotate your Telegram bot token periodically | Prevents persistent access if credentials are leaked |
| Review Guardian logs regularly | Unexpected update cycles (like repeated restarts of the same container) indicate a bug or potential tampering |

### Threat Model

Guardian is designed for **trusted homelab and internal infrastructure environments**. It is not hardened for multi-tenant or hostile environments. If your threat model includes a compromised internal host, you should audit Docker socket access across your entire stack — not just Guardian.

---

## Roadmap

- [ ] Fix: re-inspect digest after successful update to prevent notification loop
- [ ] GitHub Actions CI/CD with GHCR publishing
- [ ] Per-container update schedule override via label
- [ ] Webhook notifications (generic HTTP)
- [ ] Metrics endpoint (Prometheus-compatible)
- [ ] Automatic rollback on failed health check post-update

---

Built by [@scavenger](https://github.com/scavenger503)
