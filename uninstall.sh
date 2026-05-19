#!/usr/bin/env bash
#
# forge-agent uninstaller
#
# Stops the running forge-agent daemon and removes the binary, wrapper, config,
# the passwordless-tailscale sudoers rule, the Grafana Alloy container/config,
# and the cloudflared binary + tunnel token (the running tunnel IS stopped — it
# exposes the node). Prerequisites (docker, nixpacks, tailscale) are left
# installed.
#
# Usage:
#   sudo ./uninstall.sh [options]
#
# Options:
#   -p, --prefix DIR   Dir the binary was installed to (default: /usr/local/bin)
#       --keep-config  Keep /etc/forge-agent (config) instead of removing it
#       --keep-tailnet Do not log this device out of the tailnet
#       --keep-alloy   Keep the Grafana Alloy container and its config
#       --keep-traefik Keep the Traefik container and its ACME store
#       --keep-cloudflared Keep the cloudflared binary, token, and running tunnel
#   -y, --yes          Do not prompt for confirmation
#   -h, --help         Show this help and exit

set -euo pipefail

PREFIX="/usr/local/bin"
KEEP_CONFIG=0
KEEP_TAILNET=0
KEEP_ALLOY=0
KEEP_TRAEFIK=0
KEEP_CLOUDFLARED=0
ASSUME_YES=0

BIN_NAME="forge-agent"
CONFIG_DIR="/etc/forge-agent"
SUDOERS_PATH="/etc/sudoers.d/forge-agent-tailscale"
ALLOY_CONTAINER="alloy"
TRAEFIK_CONTAINER="traefik"

log()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m==>\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

usage() { sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

have() { command -v "$1" >/dev/null 2>&1; }

run_user_home() {
  local u h
  u="${SUDO_USER:-root}"
  h="$(getent passwd "$u" 2>/dev/null | cut -d: -f6)"
  [[ -z "$h" ]] && h="$([[ "$u" == "root" ]] && echo /root || echo "/home/$u")"
  printf '%s' "$h"
}
ALLOY_CONFIG_DIR="$(run_user_home)/.forge/grafana/alloy"
TRAEFIK_CONFIG_DIR="$(run_user_home)/.forge/traefik"
CLOUDFLARED_TOKEN="$(run_user_home)/.forge/cloudflared.token"
CLOUDFLARED_LOG="$(run_user_home)/.forge/cloudflared.log"
RUN_USER="${SUDO_USER:-root}"
CRON_MARKER="# forge-agent daemon (managed by install.sh)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--prefix)    PREFIX="$2"; shift 2 ;;
    --keep-config)  KEEP_CONFIG=1; shift ;;
    --keep-tailnet) KEEP_TAILNET=1; shift ;;
    --keep-alloy)   KEEP_ALLOY=1; shift ;;
    --keep-traefik) KEEP_TRAEFIK=1; shift ;;
    --keep-cloudflared) KEEP_CLOUDFLARED=1; shift ;;
    -y|--yes)       ASSUME_YES=1; shift ;;
    -h|--help)     usage ;;
    *)             err "unknown option: $1 (see --help)" ;;
  esac
done

[[ $EUID -ne 0 ]] && err "must run as root (use sudo)"

if [[ $ASSUME_YES -ne 1 ]]; then
  echo "This will remove:"
  echo "  - daemon  : running '$BIN_NAME daemon' is stopped"
  echo "  - binary  : $PREFIX/$BIN_NAME, $PREFIX/$BIN_NAME.bin"
  [[ $KEEP_CONFIG -ne 1 ]] && echo "  - config  : $CONFIG_DIR"
  echo "  - sudoers : $SUDOERS_PATH"
  [[ $KEEP_TAILNET -ne 1 ]] && echo "  - tailnet : log this device out (tailscale logout)"
  [[ $KEEP_ALLOY -ne 1 ]] && echo "  - alloy   : container '$ALLOY_CONTAINER' + $ALLOY_CONFIG_DIR"
  [[ $KEEP_TRAEFIK -ne 1 ]] && echo "  - traefik : container '$TRAEFIK_CONTAINER' + $TRAEFIK_CONFIG_DIR"
  [[ $KEEP_CLOUDFLARED -ne 1 ]] && echo "  - cflared : $PREFIX/cloudflared + token; running tunnel STOPPED"
  echo "  - cron    : daemon supervision entries for '$RUN_USER'"
  echo "  (docker, nixpacks, tailscale binaries are left installed)"
  read -r -p "Proceed? [y/N] " ans
  case "$ans" in
    y|Y|yes|YES) : ;;
    *) log "aborted"; exit 0 ;;
  esac
fi

# --- Binary ------------------------------------------------------------------
removed_any=0
for f in "$PREFIX/$BIN_NAME" "$PREFIX/$BIN_NAME.bin"; do
  if [[ -e "$f" ]]; then
    rm -f "$f"
    log "removed $f"
    removed_any=1
  fi
done
[[ $removed_any -eq 0 ]] && warn "no binary found under $PREFIX"

# --- sudoers (passwordless tailscale) ----------------------------------------
if [[ -f "$SUDOERS_PATH" ]]; then
  rm -f "$SUDOERS_PATH"
  log "removed sudoers rule -> $SUDOERS_PATH"
fi

# --- Reboot crontab ----------------------------------------------------------
if have crontab; then
  cur="$(crontab -u "$RUN_USER" -l 2>/dev/null || true)"
  if printf '%s\n' "$cur" | grep -qF "$CRON_MARKER"; then
    printf '%s\n' "$cur" | grep -vF "$CRON_MARKER" | sed '/^$/d' \
      | crontab -u "$RUN_USER" - \
      && log "removed daemon crontab entries for '$RUN_USER'" \
      || warn "failed to update crontab for '$RUN_USER'"
  fi
fi

# --- Daemon ------------------------------------------------------------------
# Done after supervision (cron) is removed so the per-minute watchdog can't
# relaunch it in the gap; the binary is already gone so a stray relaunch would
# no-op anyway. Frees the deleted-inode the live process still holds.
if pgrep -f "$BIN_NAME.bin daemon" >/dev/null 2>&1; then
  pkill -f "$BIN_NAME.bin daemon" 2>/dev/null || true
  sleep 1
  pkill -9 -f "$BIN_NAME.bin daemon" 2>/dev/null || true
  log "stopped running daemon"
else
  log "no running daemon"
fi

# --- Config ------------------------------------------------------------------
if [[ $KEEP_CONFIG -eq 1 ]]; then
  log "keeping config: $CONFIG_DIR (--keep-config)"
elif [[ -d "$CONFIG_DIR" ]]; then
  rm -rf "$CONFIG_DIR"
  log "removed config -> $CONFIG_DIR"
fi

# --- Grafana Alloy -----------------------------------------------------------
if [[ $KEEP_ALLOY -eq 1 ]]; then
  log "keeping Grafana Alloy (--keep-alloy)"
else
  if have docker && docker ps -a --format '{{.Names}}' | grep -qx "$ALLOY_CONTAINER"; then
    docker rm -f "$ALLOY_CONTAINER" >/dev/null 2>&1 \
      && log "removed alloy container -> $ALLOY_CONTAINER" \
      || warn "failed to remove alloy container"
  fi
  if [[ -d "$ALLOY_CONFIG_DIR" ]]; then
    rm -rf "$ALLOY_CONFIG_DIR"
    log "removed alloy config -> $ALLOY_CONFIG_DIR"
  fi
fi

# --- Traefik -----------------------------------------------------------------
if [[ $KEEP_TRAEFIK -eq 1 ]]; then
  log "keeping Traefik (--keep-traefik)"
else
  if have docker && docker ps -a --format '{{.Names}}' | grep -qx "$TRAEFIK_CONTAINER"; then
    docker rm -f "$TRAEFIK_CONTAINER" >/dev/null 2>&1 \
      && log "removed traefik container -> $TRAEFIK_CONTAINER" \
      || warn "failed to remove traefik container"
  fi
  if [[ -d "$TRAEFIK_CONFIG_DIR" ]]; then
    rm -rf "$TRAEFIK_CONFIG_DIR"
    log "removed traefik config -> $TRAEFIK_CONFIG_DIR"
  fi
fi

# --- cloudflared -------------------------------------------------------------
if [[ $KEEP_CLOUDFLARED -eq 1 ]]; then
  log "keeping cloudflared (--keep-cloudflared)"
else
  if pgrep -f 'cloudflared tunnel' >/dev/null 2>&1; then
    pkill -f 'cloudflared tunnel' 2>/dev/null || true
    log "stopped cloudflared tunnel"
  fi
  if [[ -e "$PREFIX/cloudflared" ]]; then
    rm -f "$PREFIX/cloudflared"
    log "removed cloudflared binary -> $PREFIX/cloudflared"
  fi
  for f in "$CLOUDFLARED_TOKEN" "$CLOUDFLARED_LOG"; do
    if [[ -e "$f" ]]; then
      rm -f "$f"
      log "removed $f"
    fi
  done
fi

# --- Tailnet -----------------------------------------------------------------
# Deregister this node from the tailnet. `tailscale logout` drops the node's
# auth key so it no longer appears as an active device. The tailscaled daemon
# and the tailscale binary stay installed.
if [[ $KEEP_TAILNET -eq 1 ]]; then
  log "keeping tailnet membership (--keep-tailnet)"
elif have tailscale; then
  state="$(tailscale status --json 2>/dev/null | grep -o '"BackendState":[^,]*' | head -1 || true)"
  if [[ -z "$state" || "$state" == *NeedsLogin* || "$state" == *NoState* ]]; then
    log "tailscale not logged in; nothing to deregister"
  else
    if tailscale logout 2>/dev/null; then
      log "device logged out of tailnet"
    else
      warn "tailscale logout failed; remove the device manually from the admin console"
    fi
  fi
else
  warn "tailscale binary not found; skipping tailnet logout"
fi

log "done. docker, nixpacks, tailscale binaries left untouched."
