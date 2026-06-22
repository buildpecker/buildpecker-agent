# forge-agent

The agent that runs on each VPS connected to forge.sh. It registers the
machine with forge, then runs as a background service that builds and deploys
your apps, ships logs and metrics, and keeps your machine reachable from the
internet.

## What it does

Once installed and registered, the agent:

- **Polls forge for work.** Every 5 seconds it asks the backend for queued
  deployments and deletes. Every 30 seconds it runs health checks on your
  infrastructure containers (databases, caches, and so on).
- **Builds and deploys apps.** It clones your git repo, detects the framework,
  builds a Docker image with [Nixpacks](https://nixpacks.com), and runs it as a
  container on the shared `forge` Docker network. Framework detection currently
  covers Node.js apps: Next.js, SvelteKit, Astro, Remix, and Vite. It can also
  roll back a deployment and run post-install steps.
- **Exposes your apps.** It runs a [Cloudflare Tunnel](https://www.cloudflare.com/products/tunnel/)
  so your machine is reachable from the internet without opening any ports.
- **Reports status.** It sends a heartbeat to forge every minute and pushes
  metrics (CPU, memory, disk) over OpenTelemetry.
- **Ships logs.** It writes logs to `~/.forge/logs` and runs a
  [Grafana Alloy](https://grafana.com/docs/alloy/) container that forwards them
  to Loki.

## Requirements

The installer sets these up for you if they are missing:

- A Linux machine you can `sudo` on (Debian/Ubuntu, Fedora/RHEL, Arch, or
  openSUSE for automatic prerequisite installs).
- Docker
- Nixpacks
- cloudflared
- cron (used for supervision instead of systemd)

## Install

```
curl -fsSL https://raw.githubusercontent.com/forge-paas/forge-agent/refs/heads/main/install.sh | sudo bash
```

The installer:

1. Installs any missing prerequisites.
2. Creates the `forge` Docker network.
3. Fetches the prebuilt `forge-agent` binary and puts it on your `PATH`.
4. Writes config to `/etc/forge-agent/.env`.
5. Starts a Grafana Alloy container for log shipping.
6. Sets up cron so the agent starts on boot and restarts if it crashes.

### Install options

Pass options after `bash -s --`. For example:

```
curl -fsSL .../install.sh | sudo bash -s -- --no-alloy
```

| Option | Default | Meaning |
| --- | --- | --- |
| `-r, --repo URL` | forge-agent repo | Git repo to fetch the binary from |
| `-b, --branch NAME` | `main` | Branch to check out |
| `-p, --prefix DIR` | `/usr/local/bin` | Where to install the binary |
| `--convex-url URL` | forge cloud URL | Backend cloud URL |
| `--convex-site URL` | forge site URL | Backend site URL |
| `--otel-endpoint URL` | forge OTel URL | Where metrics are pushed |
| `--loki-url URL` | forge Loki URL | Where logs are shipped |
| `--no-alloy` | (off) | Skip the Grafana Alloy container |

## Register and run

Run these as your normal user, **not** with `sudo`. The agent reads its node
config from `~/.forge`, so it must run as the same user that owns that
directory.

1. **Register the machine.** Get a token from the forge dashboard, then:

   ```
   forge-agent register <TOKEN>
   ```

   This registers the machine with forge and sets up its Cloudflare Tunnel.

2. **Start the agent.** The cron watchdog starts it within a minute, or start
   it yourself:

   ```
   nohup forge-agent daemon >> ~/.forge/daemon.log 2>&1 &
   ```

Until you register, the agent is a harmless no-op: with no node config it just
exits.

## Commands

| Command | What it does |
| --- | --- |
| `forge-agent register <TOKEN>` | Register this machine with forge |
| `forge-agent daemon` | Run the long-lived service that does the work |

## How it stays running

The agent is supervised with cron, not systemd. (Under systemd the daemon hit
TLS errors caused by the service environment; a login shell avoids them.) The
installer adds two cron jobs for both the agent and cloudflared:

- An `@reboot` job that starts them on boot.
- A per-minute job that restarts them if they are not running.

Both run through a login shell, matching a working manual run. The agent's
output goes to `~/.forge/daemon.log`.

## Files and paths

| Path | What it is |
| --- | --- |
| `/usr/local/bin/forge-agent` | The binary (a wrapper that loads config, then runs the real binary) |
| `/etc/forge-agent/.env` | Global config: backend URLs and the OTel endpoint |
| `~/.forge/config.json` | Per-machine node config, written at registration |
| `~/.forge/logs/` | Agent logs (api, system, deploy, and per-deployment) |
| `~/.forge/daemon.log` | Output from the supervised daemon |
| `~/.forge/grafana/alloy/` | Alloy config and read-position store |

## Uninstall

```
curl -fsSL https://raw.githubusercontent.com/forge-paas/forge-agent/refs/heads/main/uninstall.sh | sudo bash -s -- -y
```
