#!/usr/bin/env bash
#
# buildpecker-agent installer
#
# Installs the buildpecker-agent VPS agent and its prerequisites (docker, nixpacks,
# cloudflared), fetches the prebuilt binary from the git repository, installs it
# onto PATH, seeds configuration, and spins up a Grafana Alloy container for log
# shipping.
#
# Usage:
#   sudo ./install.sh [options]
#
# Options:
#   -r, --repo URL        Git repo to fetch the binary from
#                         (default: https://github.com/buildpecker-paas/buildpecker-agent.git)
#   -b, --branch NAME     Branch to checkout (default: main)
#   -p, --prefix DIR      Install dir for the binary (default: /usr/local/bin)
#       --convex-url URL  CONVEX_CLOUD_URL value (default: http://localhost:3210)
#       --convex-site URL CONVEX_SITE_URL value (default: http://localhost:3211)
#       --otel-endpoint A OTEL_EXPORTER_OTLP_ENDPOINT value (default: localhost:4318)
#       --loki-url URL    Loki base URL for Alloy (default: https://loki.parthajeet.xyz)
#       --no-alloy        Skip the Grafana Alloy container
#   -h, --help            Show this help and exit

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
REPO_URL="https://github.com/buildpecker-paas/buildpecker-agent.git"
BRANCH="main"
PREFIX="/usr/local/bin"
CONVEX_CLOUD_URL="https://convex-cloud.parthajeet.xyz"
CONVEX_SITE_URL="https://convex-site.parthajeet.xyz"
OTEL_EXPORTER_OTLP_ENDPOINT="https://otel-collector.parthajeet.xyz"
LOKI_URL="https://loki.parthajeet.xyz"
INSTALL_ALLOY=1

ALLOY_CONTAINER="alloy"
DOCKER_NETWORK="buildpecker"

BIN_NAME="buildpecker-agent"
CONFIG_DIR="/etc/buildpecker-agent"

# The human who ran `sudo ./install.sh`. `buildpecker-agent register` writes the
# node config to that user's ~/.buildpecker/config.json, and the daemon reads it
# back via os.UserHomeDir(). Register AND run the daemon as this same user,
# else it looks in /root/.buildpecker, finds nothing, and fails.
RUN_USER="${SUDO_USER:-root}"

TMP_DIR=""
cleanup() { [[ -n "$TMP_DIR" && -d "$TMP_DIR" ]] && rm -rf "$TMP_DIR"; }
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m==>\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

usage() { sed -n '2,23p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

need_root() {
  if [[ $EUID -ne 0 ]]; then
    err "must run as root (use sudo)"
  fi
}

have() { command -v "$1" >/dev/null 2>&1; }

# ---------------------------------------------------------------------------
# Arg parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    -r|--repo)          REPO_URL="$2"; shift 2 ;;
    -b|--branch)        BRANCH="$2"; shift 2 ;;
    -p|--prefix)        PREFIX="$2"; shift 2 ;;
    --convex-url)       CONVEX_CLOUD_URL="$2"; shift 2 ;;
    --convex-site)      CONVEX_SITE_URL="$2"; shift 2 ;;
    --otel-endpoint)    OTEL_EXPORTER_OTLP_ENDPOINT="$2"; shift 2 ;;
    --loki-url)         LOKI_URL="$2"; shift 2 ;;
    --no-alloy)         INSTALL_ALLOY=0; shift ;;
    -h|--help)          usage ;;
    *)                  err "unknown option: $1 (see --help)" ;;
  esac
done

# ---------------------------------------------------------------------------
# OS / package manager detection
# ---------------------------------------------------------------------------
PKG=""
detect_pkg() {
  if   have apt-get; then PKG="apt"
  elif have dnf;     then PKG="dnf"
  elif have yum;     then PKG="yum"
  elif have pacman;  then PKG="pacman"
  elif have zypper;  then PKG="zypper"
  else warn "no supported package manager found; prerequisite auto-install disabled"
  fi
}

pkg_install() {
  # $@ = package names
  case "$PKG" in
    apt)    apt-get update -qq && apt-get install -y "$@" ;;
    dnf)    dnf install -y "$@" ;;
    yum)    yum install -y "$@" ;;
    pacman) pacman -Sy --noconfirm "$@" ;;
    zypper) zypper install -y "$@" ;;
    *)      return 1 ;;
  esac
}

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------
ensure_base_tools() {
  for t in curl git; do
    if ! have "$t"; then
      log "installing missing tool: $t"
      pkg_install "$t" || err "could not install $t; install it manually and re-run"
    fi
  done
  ensure_cron
}

# The crontab supervisor is useless if the cron daemon isn't installed/running.
# Package + service names differ per distro.
ensure_cron() {
  local pkg="" svc=""
  case "$PKG" in
    apt)            pkg="cron";   svc="cron" ;;
    dnf|yum)        pkg="cronie"; svc="crond" ;;
    pacman|zypper)  pkg="cronie"; svc="cronie" ;;
  esac

  if ! have crontab; then
    if [[ -z "$pkg" ]]; then
      warn "crontab missing and no known package manager; install cron manually"
      return
    fi
    log "cron not found; installing ($pkg)"
    pkg_install "$pkg" || { warn "could not install $pkg; supervision will be skipped"; return; }
  fi

  # Already up via any mechanism (systemd, SysV, container init)? Done.
  if pgrep -x crond >/dev/null 2>&1 || pgrep -x cron >/dev/null 2>&1; then
    log "cron daemon already running"
    return
  fi

  if have systemctl; then
    # A freshly installed unit isn't visible until systemd rescans.
    systemctl daemon-reload 2>/dev/null || true
    local s
    for s in "$svc" cron crond cronie; do
      [[ -z "$s" ]] && continue
      systemctl enable --now "$s.service" >/dev/null 2>&1 || true
      if systemctl is-active --quiet "$s.service" 2>/dev/null; then
        log "cron daemon enabled and running ($s)"
        return
      fi
    done
  fi

  # SysV / no-systemd fallback.
  if have service; then
    for s in "$svc" cron crond cronie; do
      [[ -z "$s" ]] && continue
      service "$s" start >/dev/null 2>&1 && { log "cron daemon started via service ($s)"; return; }
    done
  fi

  pgrep -x crond >/dev/null 2>&1 || pgrep -x cron >/dev/null 2>&1 \
    && { log "cron daemon running"; return; }

  warn "cron installed but could not enable its service; enable it manually"
}

ensure_docker() {
  if have docker; then
    log "docker present: $(docker --version 2>/dev/null || echo unknown)"
    return
  fi
  log "docker not found; installing via get.docker.com"
  if ! have curl; then pkg_install curl || err "curl required to install docker"; fi
  curl -fsSL https://get.docker.com | sh || err "docker installation failed"
  systemctl enable --now docker 2>/dev/null || warn "could not enable docker service (no systemd?)"
  have docker || err "docker still not on PATH after install"
  log "docker installed: $(docker --version)"
}

# The shared docker network every buildpecker container (Alloy, future app
# containers) attaches to. Created once, up-front, so later steps can assume it.
ensure_docker_network() {
  have docker || return 0
  if docker network inspect "$DOCKER_NETWORK" >/dev/null 2>&1; then
    log "docker network present: $DOCKER_NETWORK"
    return 0
  fi
  docker network create "$DOCKER_NETWORK" >/dev/null \
    && log "created docker network: $DOCKER_NETWORK" \
    || warn "could not create docker network: $DOCKER_NETWORK"
}

ensure_nixpacks() {
  if have nixpacks; then
    log "nixpacks present: $(nixpacks --version 2>/dev/null || echo unknown)"
    return
  fi
  log "nixpacks not found; installing via official script"
  # Installs to /usr/local/bin by default.
  curl -fsSL https://nixpacks.com/install.sh | bash || err "nixpacks installation failed"
  if ! have nixpacks && [[ -x /usr/local/bin/nixpacks ]]; then
    : # on PATH via /usr/local/bin
  fi
  have nixpacks || err "nixpacks still not on PATH after install"
  log "nixpacks installed: $(nixpacks --version)"
}

# cloudflared: the per-node tunnel client. Installed as a static host binary
# (no systemd, no package manager). The tunnel itself is started later by the
# agent (system.SetupCloudflared) once registration hands it a tunnel token.
ensure_cloudflared() {
  if have cloudflared; then
    log "cloudflared present: $(cloudflared --version 2>/dev/null | head -1 || echo unknown)"
    return
  fi
  log "cloudflared not found; installing static binary"

  local arch bin_arch url dest
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)  bin_arch="amd64" ;;
    aarch64|arm64) bin_arch="arm64" ;;
    armv7l|armv6l) bin_arch="arm" ;;
    i386|i686)     bin_arch="386" ;;
    *) warn "unsupported arch '$arch' for cloudflared; install it manually"; return ;;
  esac

  url="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${bin_arch}"
  dest="$PREFIX/cloudflared"
  install -d "$PREFIX"
  if ! curl -fsSL "$url" -o "$dest"; then
    warn "cloudflared download failed ($url); install it manually"
    return
  fi
  chmod 0755 "$dest"
  have cloudflared || warn "$PREFIX not on PATH; cloudflared installed at $dest"
  log "cloudflared installed: $("$dest" --version 2>/dev/null | head -1 || echo unknown)"
}

# ---------------------------------------------------------------------------
# Binary fetch + install
# ---------------------------------------------------------------------------
install_binary() {
  local src=""

  # If invoked from within a checkout that already has the binary, use it.
  local self="${BASH_SOURCE[0]:-}"
  local here=""
  [[ -n "$self" && -f "$self" ]] && here="$(cd "$(dirname "$self")" && pwd)"

  if [[ -n "$here" && -x "$here/$BIN_NAME" ]]; then
    log "using binary from local checkout: $here/$BIN_NAME"
    src="$here/$BIN_NAME"
  else
    TMP_DIR="$(mktemp -d)"
    log "cloning $REPO_URL (branch $BRANCH)"
    git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$TMP_DIR/repo" \
      || err "git clone failed"
    [[ -f "$TMP_DIR/repo/$BIN_NAME" ]] \
      || err "$BIN_NAME not found in repo root; expected a committed prebuilt binary"
    src="$TMP_DIR/repo/$BIN_NAME"
  fi

  install -d "$PREFIX"
  install -m 0755 "$src" "$PREFIX/$BIN_NAME.bin"

  # Wrapper: the binary loads .env from the current working directory, so a
  # bare `buildpecker-agent register ...` run from $HOME sees no config. The wrapper
  # sources the global config into the environment before exec'ing the real
  # binary, making it work from any cwd.
  cat > "$PREFIX/$BIN_NAME" <<EOF
#!/usr/bin/env bash
set -a
[ -f "$CONFIG_DIR/.env" ] && . "$CONFIG_DIR/.env"
set +a
exec "$PREFIX/$BIN_NAME.bin" "\$@"
EOF
  chmod 0755 "$PREFIX/$BIN_NAME"
  log "installed binary -> $PREFIX/$BIN_NAME (wrapper) + $PREFIX/$BIN_NAME.bin"

  case ":$PATH:" in
    *":$PREFIX:"*) : ;;
    *) warn "$PREFIX is not on PATH; add it or move the binary into a PATH dir" ;;
  esac
}

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
write_config() {
  install -d "$CONFIG_DIR"
  local env_file="$CONFIG_DIR/.env"
  if [[ -f "$env_file" ]]; then
    # Ensure a non-root user can read it (the wrapper sources this file).
    chmod 0644 "$env_file"
    log "config exists, leaving contents as-is: $env_file"
    return
  fi
  cat > "$env_file" <<EOF
CONVEX_CLOUD_URL="$CONVEX_CLOUD_URL"
CONVEX_SITE_URL="$CONVEX_SITE_URL"

# OpenTelemetry Exporter
OTEL_EXPORTER_OTLP_ENDPOINT="$OTEL_EXPORTER_OTLP_ENDPOINT"
EOF
  # World-readable: the wrapper sources this as the invoking (non-root) user.
  chmod 0644 "$env_file"
  log "wrote config -> $env_file"
}

# ---------------------------------------------------------------------------
# Grafana Alloy (log shipping)
# ---------------------------------------------------------------------------
run_user_home() {
  local h
  h="$(getent passwd "$RUN_USER" 2>/dev/null | cut -d: -f6)"
  [[ -z "$h" ]] && h="$([[ "$RUN_USER" == "root" ]] && echo /root || echo "/home/$RUN_USER")"
  printf '%s' "$h"
}

write_alloy_config() {
  local cfg="$1"
  local dir
  dir="$(dirname "$cfg")"
  # Create the full path recursively (mkdir -p semantics) so a fresh host with
  # no ~/.buildpecker tree doesn't error on the `cat >` / docker bind-mount.
  mkdir -p "$dir" || err "could not create alloy config dir: $dir"
  [[ -d "$dir" ]] || err "alloy config dir missing after create: $dir"
  # Quoted heredoc: the config contains backticks/`${}`/`sys.env(...)` that must
  # be written literally. The Loki push URL is templated via a placeholder and
  # substituted afterwards so --loki-url stays configurable.
  cat > "$cfg" <<'ALLOY_EOF'
// ~/.buildpecker/grafana/alloy/config.alloy
// Run: alloy run ~/.buildpecker/grafana/alloy/config.alloy --storage.path=/tmp/alloy

logging {
  level  = "info"
  format = "logfmt"
}

/* ---------- file discovery ---------- */

local.file_match "buildpecker_api" {
  path_targets = [{
    __path__ = "/var/log/buildpecker/api.log",
    job      = "buildpecker-agent",
    service  = "api",
    host     = constants.hostname,
  }]
}

local.file_match "buildpecker_system" {
  path_targets = [{
    __path__ = "/var/log/buildpecker/system.log",
    job      = "buildpecker-agent",
    service  = "system",
    host     = constants.hostname,
  }]
}

local.file_match "buildpecker_deploy" {
  path_targets = [{
    __path__ = "/var/log/buildpecker/deploy.log",
    job      = "buildpecker-agent",
    service  = "deploy",
    host     = constants.hostname,
  }]
}

local.file_match "buildpecker_deployments" {
  path_targets = [{
    __path__ = "/var/log/buildpecker/deployments/*.log",
    job      = "buildpecker-agent",
    service  = "deployment",
    host     = constants.hostname,
  }]
}

/* ---------- tail ---------- */

loki.source.file "buildpecker_api" {
  targets    = local.file_match.buildpecker_api.targets
  forward_to = [loki.process.buildpecker_std.receiver]
}

loki.source.file "buildpecker_system" {
  targets    = local.file_match.buildpecker_system.targets
  forward_to = [loki.process.buildpecker_std.receiver]
}

loki.source.file "buildpecker_deploy" {
  targets    = local.file_match.buildpecker_deploy.targets
  forward_to = [loki.process.buildpecker_std.receiver]
}

loki.source.file "buildpecker_deployments" {
  targets    = local.file_match.buildpecker_deployments.targets
  forward_to = [loki.process.buildpecker_deployment.receiver]
}

/* ---------- parse: api / system / deploy ----------
   line shape: "[API] 2026/05/14 12:34:56.123456 message..." (UTC)
*/
loki.process "buildpecker_std" {
  forward_to = [loki.write.default.receiver]

  stage.regex {
    expression = `^\[(?P<tag>[A-Z]+)\]\s+(?P<ts>\d{4}/\d{2}/\d{2}\s\d{2}:\d{2}:\d{2}\.\d+)\s+(?P<msg>.*)$`
  }

  stage.timestamp {
    source   = "ts"
    format   = "2006/01/02 15:04:05.000000"
    location = "UTC"
  }

  // tag becomes a label (low cardinality: API/SYSTEM/DEPLOY)
  stage.labels {
    values = { tag = "" }
  }

  // drop the prefix from the stored line
  stage.output {
    source = "msg"
  }

  // crude level inference for Loki/Grafana level filter
  stage.match {
    selector = `{tag=~".+"} |~ "(?i)failed|error"`
    stage.static_labels {
      values = { level = "error" }
    }
  }
  stage.match {
    selector = `{tag=~".+"} |~ "(?i)warn"`
    stage.static_labels {
      values = { level = "warn" }
    }
  }
}

/* ---------- parse: per-deployment ----------
   line shape: "[DEPLOYMENT:<id>] 2026/05/14 12:34:56.123456 message..."
   NB: deployment_id is high cardinality — kept in structured_metadata,
   not as a label, to avoid blowing up the index.
*/
loki.process "buildpecker_deployment" {
  forward_to = [loki.write.default.receiver]

  stage.regex {
    expression = `^\[DEPLOYMENT:(?P<deployment_id>[^\]]+)\]\s+(?P<ts>\d{4}/\d{2}/\d{2}\s\d{2}:\d{2}:\d{2}\.\d+)\s+(?P<msg>.*)$`
  }

  stage.timestamp {
    source   = "ts"
    format   = "2006/01/02 15:04:05.000000"
    location = "UTC"
  }

  stage.structured_metadata {
    values = { deployment_id = "" }
  }

  stage.static_labels {
    values = { tag = "DEPLOYMENT" }
  }

  stage.output {
    source = "msg"
  }
}

/* ---------- ship ---------- */

loki.write "default" {
  endpoint {
    url = "__LOKI_PUSH_URL__"

    // Grafana Cloud: uncomment + set env vars
    // tenant_id = sys.env("LOKI_TENANT_ID")
    // basic_auth {
    //   username = sys.env("LOKI_USER")
    //   password = sys.env("LOKI_PASSWORD")
    // }
  }

  external_labels = {
    env = "dev",
  }
}
ALLOY_EOF

  local push_url="${LOKI_URL%/}/loki/api/v1/push"
  sed -i "s|__LOKI_PUSH_URL__|$push_url|" "$cfg"
  chown -R "$RUN_USER": "$(dirname "$(dirname "$(dirname "$cfg")")")" 2>/dev/null || true
  log "wrote alloy config -> $cfg (loki: $push_url)"
}

ensure_alloy() {
  if [[ $INSTALL_ALLOY -ne 1 ]]; then
    log "skipping Grafana Alloy (--no-alloy)"
    return
  fi
  have docker || { warn "docker missing; skipping Alloy"; return; }

  local home cfg log_dir
  home="$(run_user_home)"
  cfg="$home/.buildpecker/grafana/alloy/config.alloy"
  write_alloy_config "$cfg"

  # The agent writes logs to ~/.buildpecker/logs (utils.GetLoggerInstance), but the
  # Alloy config tails /var/log/buildpecker/*. Bind-mount the real log dir to that
  # path so Alloy actually sees the files. Created up-front in case the agent
  # hasn't started yet (empty dir is fine; loki.source.file picks files up).
  log_dir="$home/.buildpecker/logs"
  mkdir -p "$log_dir/deployments"
  chown -R "$RUN_USER": "$log_dir" 2>/dev/null || true

  # Persist Alloy's read-position store so recreating the container resumes
  # instead of re-reading every file from the start (which resends logs).
  local data_dir="$home/.buildpecker/grafana/alloy/data"
  mkdir -p "$data_dir"
  chown -R "$RUN_USER": "$data_dir" 2>/dev/null || true

  # Pin the container hostname to the real host. The config labels logs with
  # `constants.hostname`; left unset, Docker assigns a fresh random hostname
  # per `docker run`, so every recreate becomes a NEW Loki stream and the
  # resent lines can't be deduplicated (you get N copies after N reinstalls).
  local host_name; host_name="$(hostname 2>/dev/null || echo buildpecker-node)"

  # Recreate the container so it picks up the (possibly new) config.
  if docker ps -a --format '{{.Names}}' | grep -qx "$ALLOY_CONTAINER"; then
    docker rm -f "$ALLOY_CONTAINER" >/dev/null 2>&1 || true
  fi

  docker run \
    -d \
    -v "$cfg":/etc/alloy/config.alloy \
    -v "$log_dir":/var/log/buildpecker:ro \
    -v "$data_dir":/var/lib/alloy/data \
    --hostname "$host_name" \
    -p 127.0.0.1:12345:12345 --name "$ALLOY_CONTAINER" --network "$DOCKER_NETWORK" \
    --restart unless-stopped \
    grafana/alloy:latest \
      run --server.http.listen-addr=0.0.0.0:12345 --storage.path=/var/lib/alloy/data \
      /etc/alloy/config.alloy >/dev/null \
    && log "alloy container started (http://localhost:12345)" \
    || warn "failed to start alloy container"
}

# ---------------------------------------------------------------------------
# Supervision (no systemd): crontab watchdog + @reboot for the run user
#
# Deliberately NOT a systemd service: under a unit the daemon hits HTTPS/TLS
# errors that never occur from an interactive run. Root cause is the systemd
# execution context — `EnvironmentFile` mangles the quoted .env URLs and a
# service has no login env. The watchdog runs the daemon through a LOGIN
# shell (`bash -lc`), i.e. the exact environment where it works today.
# ---------------------------------------------------------------------------
CRON_MARKER="# buildpecker-agent daemon (managed by install.sh)"

# A daemon started by a previous install keeps running the OLD binary even
# after the file is replaced (Linux holds the deleted inode). The per-minute
# watchdog only launches when NO daemon is running, so it never supersedes a
# live old process. Kill it here so the watchdog relaunches the fresh binary.
stop_running_daemon() {
  if pgrep -f "$BIN_NAME.bin daemon" >/dev/null 2>&1; then
    pkill -f "$BIN_NAME.bin daemon" 2>/dev/null || true
    log "stopped running daemon (old binary); watchdog will relaunch the new one"
  fi
}

install_supervisor_cron() {
  if ! have crontab; then
    warn "crontab not found; skipping supervision (start the daemon manually)"
    return
  fi

  local home log_file start_cmd reboot_line watch_line existing
  home="$(run_user_home)"
  log_file="$home/.buildpecker/daemon.log"

  # Same invocation as a working manual run: login shell so HOME/PATH/env
  # match, wrapper sources /etc/buildpecker-agent/.env, nohup detaches from cron.
  start_cmd="nohup \"$PREFIX/$BIN_NAME\" daemon >> \"$log_file\" 2>&1 &"
  reboot_line="@reboot /bin/bash -lc '$start_cmd'  $CRON_MARKER"
  # Every minute: relaunch only if no daemon is running (matches the real
  # process — the wrapper execs \$BIN_NAME.bin). The leading char is bracketed
  # ("[f]orge-agent...") so the pgrep pattern in THIS cron line's own cmdline
  # does not match itself — without it the watchdog always thinks the daemon
  # is up and never (re)starts it.
  watch_line="* * * * * /bin/bash -lc 'pgrep -f \"[${BIN_NAME:0:1}]${BIN_NAME:1}.bin daemon\" >/dev/null 2>&1 || { $start_cmd }'  $CRON_MARKER"

  # cloudflared tunnel supervision. The token is written by the agent
  # (system.SetupCloudflared) to ~/.buildpecker/cloudflared.token (0600) at
  # registration; until then both lines are a no-op (token file absent),
  # mirroring the daemon's "harmless until you register" behaviour. Bracketed
  # pgrep so the watchdog's own cmdline doesn't self-match.
  local cf_bin cf_token cf_log cf_start
  cf_bin="$PREFIX/cloudflared"
  cf_token="$home/.buildpecker/cloudflared.token"
  cf_log="$home/.buildpecker/cloudflared.log"
  cf_start="nohup \"$cf_bin\" tunnel --no-autoupdate run --token \"\$(cat $cf_token)\" >> \"$cf_log\" 2>&1 &"
  cf_reboot_line="@reboot /bin/bash -lc 'test -s $cf_token && { $cf_start }'  $CRON_MARKER"
  cf_watch_line="* * * * * /bin/bash -lc 'test -s $cf_token && { pgrep -f \"[c]loudflared tunnel\" >/dev/null 2>&1 || { $cf_start }; }'  $CRON_MARKER"

  existing="$(crontab -u "$RUN_USER" -l 2>/dev/null || true)"
  # Drop ALL prior buildpecker-agent cron lines so reinstall never duplicates.
  # Match two independent tokens: the current marker AND the log-redirect
  # path that every version we've ever shipped writes (`.buildpecker/daemon.log`).
  # The marker text changed across builds; the log path never did, so an
  # old line with a different/missing marker is still stripped here.
  existing="$(printf '%s\n' "$existing" \
    | grep -vF "$CRON_MARKER" \
    | grep -vF '/.buildpecker/daemon.log' \
    || true)"

  printf '%s\n%s\n%s\n%s\n%s\n' "$existing" "$reboot_line" "$watch_line" "$cf_reboot_line" "$cf_watch_line" \
    | sed '/^$/d' \
    | crontab -u "$RUN_USER" - \
    && log "supervision installed (@reboot + per-minute watchdog for daemon + cloudflared, '$RUN_USER')" \
    || warn "could not install crontab for '$RUN_USER'"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  need_root
  detect_pkg
  ensure_base_tools
  ensure_docker
  ensure_docker_network
  ensure_nixpacks
  ensure_cloudflared
  install_binary
  write_config
  ensure_alloy
  stop_running_daemon
  install_supervisor_cron

  local home; home="$(run_user_home)"

  log "done."
  echo
  echo "  binary : $PREFIX/$BIN_NAME"
  echo "  config : $CONFIG_DIR/.env"
  if [[ $INSTALL_ALLOY -eq 1 ]]; then
    echo "  alloy  : $home/.buildpecker/grafana/alloy/config.alloy (container '$ALLOY_CONTAINER', :12345)"
  fi
  echo
  echo "  Next steps (run as user '$RUN_USER' — NOT via sudo; the daemon reads"
  echo "  its node config from ~/.buildpecker, so it must be the same user):"
  echo "   1. register : $BIN_NAME register <TOKEN>"
  echo "   2. start    : nohup $BIN_NAME daemon >> $home/.buildpecker/daemon.log 2>&1 &"
  echo "                 (or just wait <=60s for the watchdog to start it)"
  echo
  echo "  Supervision (no systemd) via crontab for '$RUN_USER':"
  echo "   - @reboot      : starts the daemon on boot"
  echo "   - per-minute   : relaunches it if not running (crash recovery)"
  echo "   - logs         : $home/.buildpecker/daemon.log"
  echo "  Both run through a login shell, matching a working manual run."
  echo "  Harmless no-op until you register (daemon exits without node config)."
}

main
