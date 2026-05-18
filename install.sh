#!/usr/bin/env bash
#
# forge-agent installer
#
# Installs the forge-agent VPS agent and its prerequisites (docker, nixpacks,
# tailscale; the run user gets passwordless sudo for tailscale),
# fetches the prebuilt binary from the git repository, installs it onto PATH,
# seeds configuration, and registers a systemd service for the daemon.
#
# Usage:
#   sudo ./install.sh [options]
#
# Options:
#   -r, --repo URL        Git repo to fetch the binary from
#                         (default: https://github.com/forge-paas/forge-agent.git)
#   -b, --branch NAME     Branch to checkout (default: main)
#   -p, --prefix DIR      Install dir for the binary (default: /usr/local/bin)
#       --convex-url URL  CONVEX_CLOUD_URL value (default: http://localhost:3210)
#       --convex-site URL CONVEX_SITE_URL value (default: http://localhost:3211)
#       --otel-endpoint A OTEL_EXPORTER_OTLP_ENDPOINT value (default: localhost:4318)
#       --no-service      Skip systemd service installation
#   -h, --help            Show this help and exit

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
REPO_URL="https://github.com/forge-paas/forge-agent.git"
BRANCH="main"
PREFIX="/usr/local/bin"
CONVEX_CLOUD_URL="https://convex-cloud.parthajeet.xyz"
CONVEX_SITE_URL="https://convex-site.parthajeet.xyz"
OTEL_EXPORTER_OTLP_ENDPOINT="https://otel-collector.parthajeet.xyz"
INSTALL_SERVICE=1

BIN_NAME="forge-agent"
CONFIG_DIR="/etc/forge-agent"
SERVICE_NAME="forge-agent.service"

# The human who ran `sudo ./install.sh`. `forge-agent register` writes the
# node config to that user's ~/.forge/config.json, and the daemon reads it
# back via os.UserHomeDir(). The service must run as the SAME user or it
# looks in /root/.forge, finds nothing, and crash-loops in "activating".
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

usage() { sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

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
    --no-service)       INSTALL_SERVICE=0; shift ;;
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

ensure_tailscale() {
  if have tailscale; then
    log "tailscale present: $(tailscale version 2>/dev/null | head -1 || echo unknown)"
  else
    log "tailscale not found; installing via official script"
    curl -fsSL https://tailscale.com/install.sh | sh || err "tailscale installation failed"
    have tailscale || err "tailscale still not on PATH after install"
    log "tailscale installed: $(tailscale version 2>/dev/null | head -1)"
  fi
  grant_tailscale_sudo
  set_tailscale_operator
}

# Drop the need to type `sudo` at all: setting the tailscale operator to the
# run user lets that user run `tailscale up` / `tailscale set` directly.
# Requires tailscaled to be running; the official installer enables it.
set_tailscale_operator() {
  [[ "$RUN_USER" == "root" ]] && return

  # tailscaled may need a moment after install to come up.
  local i
  for i in 1 2 3 4 5; do
    tailscale version >/dev/null 2>&1 && break
    sleep 1
  done

  if tailscale set --operator="$RUN_USER" 2>/dev/null; then
    log "tailscale operator set to '$RUN_USER' (no sudo needed for tailscale)"
  else
    warn "could not set tailscale operator now (daemon not ready / not logged in)"
    warn "run once after first 'tailscale up': sudo tailscale set --operator=$RUN_USER"
  fi
}

# Let the run user drive tailscale without typing a sudo password.
# `tailscale up` etc. require root; a NOPASSWD sudoers rule scoped to the
# tailscale binary means `sudo tailscale ...` runs without a prompt.
grant_tailscale_sudo() {
  [[ "$RUN_USER" == "root" ]] && { log "run user is root; no sudoers rule needed"; return; }

  local ts_bin
  ts_bin="$(command -v tailscale || echo /usr/bin/tailscale)"

  local sudoers="/etc/sudoers.d/forge-agent-tailscale"
  local tmp
  tmp="$(mktemp)"
  cat > "$tmp" <<EOF
# Managed by forge-agent install.sh — passwordless tailscale for the agent user
$RUN_USER ALL=(root) NOPASSWD: $ts_bin, $ts_bin *
EOF

  if visudo -cf "$tmp" >/dev/null 2>&1; then
    install -m 0440 "$tmp" "$sudoers"
    rm -f "$tmp"
    log "passwordless sudo for tailscale granted to '$RUN_USER' -> $sudoers"
  else
    rm -f "$tmp"
    err "generated sudoers rule failed validation; not installing it"
  fi
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
  # bare `forge-agent register ...` run from $HOME sees no config. The wrapper
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
# systemd service
# ---------------------------------------------------------------------------
install_service() {
  if [[ $INSTALL_SERVICE -ne 1 ]]; then
    log "skipping systemd service (--no-service)"
    return
  fi
  if ! have systemctl; then
    warn "systemd not available; skipping service install"
    return
  fi
  local unit="/etc/systemd/system/$SERVICE_NAME"
  cat > "$unit" <<EOF
[Unit]
Description=forge-agent VPS agent (forge.sh)
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
User=$RUN_USER
EnvironmentFile=$CONFIG_DIR/.env
ExecStart=$PREFIX/$BIN_NAME daemon
Restart=on-failure
RestartSec=5
WorkingDirectory=$CONFIG_DIR

[Install]
WantedBy=multi-user.target
EOF
  log "wrote unit -> $unit (runs as user: $RUN_USER)"
  systemctl daemon-reload
  # Intentionally NOT enabled/started: the daemon must only run after the
  # node has been registered. Enable + start manually post-registration.
  log "service installed but not started (register the node first)"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  need_root
  detect_pkg
  ensure_base_tools
  ensure_docker
  ensure_nixpacks
  ensure_tailscale
  install_binary
  write_config
  install_service

  log "done."
  echo
  echo "  binary : $PREFIX/$BIN_NAME"
  echo "  config : $CONFIG_DIR/.env"
  echo
  echo "  Next steps (in order):"
  echo "   1. register : $BIN_NAME register <TOKEN>"
  echo "                 (run as user '$RUN_USER' — NOT via sudo;"
  echo "                  the service runs as '$RUN_USER' and reads its ~/.forge config)"
  if [[ $INSTALL_SERVICE -eq 1 ]] && have systemctl; then
    echo "   2. start    : sudo systemctl enable --now $SERVICE_NAME"
    echo "   3. logs     : journalctl -u $SERVICE_NAME -f"
  else
    echo "   2. start    : $BIN_NAME daemon"
  fi
}

main
