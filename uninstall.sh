#!/usr/bin/env bash
#
# forge-agent uninstaller
#
# Removes the forge-agent binary, wrapper, config, and systemd service.
# Prerequisites (docker, nixpacks) are intentionally left installed.
#
# Usage:
#   sudo ./uninstall.sh [options]
#
# Options:
#   -p, --prefix DIR   Dir the binary was installed to (default: /usr/local/bin)
#       --keep-config  Keep /etc/forge-agent (config) instead of removing it
#   -y, --yes          Do not prompt for confirmation
#   -h, --help         Show this help and exit

set -euo pipefail

PREFIX="/usr/local/bin"
KEEP_CONFIG=0
ASSUME_YES=0

BIN_NAME="forge-agent"
CONFIG_DIR="/etc/forge-agent"
SERVICE_NAME="forge-agent.service"
UNIT_PATH="/etc/systemd/system/$SERVICE_NAME"

log()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m==>\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

usage() { sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

have() { command -v "$1" >/dev/null 2>&1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--prefix)   PREFIX="$2"; shift 2 ;;
    --keep-config) KEEP_CONFIG=1; shift ;;
    -y|--yes)      ASSUME_YES=1; shift ;;
    -h|--help)     usage ;;
    *)             err "unknown option: $1 (see --help)" ;;
  esac
done

[[ $EUID -ne 0 ]] && err "must run as root (use sudo)"

if [[ $ASSUME_YES -ne 1 ]]; then
  echo "This will remove:"
  echo "  - service : $UNIT_PATH"
  echo "  - binary  : $PREFIX/$BIN_NAME, $PREFIX/$BIN_NAME.bin"
  [[ $KEEP_CONFIG -ne 1 ]] && echo "  - config  : $CONFIG_DIR"
  echo "  (docker and nixpacks are left installed)"
  read -r -p "Proceed? [y/N] " ans
  case "$ans" in
    y|Y|yes|YES) : ;;
    *) log "aborted"; exit 0 ;;
  esac
fi

# --- Service -----------------------------------------------------------------
if have systemctl; then
  if systemctl list-unit-files 2>/dev/null | grep -q "^$SERVICE_NAME"; then
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    log "service stopped and disabled"
  fi
  if [[ -f "$UNIT_PATH" ]]; then
    rm -f "$UNIT_PATH"
    systemctl daemon-reload
    systemctl reset-failed "$SERVICE_NAME" 2>/dev/null || true
    log "removed unit -> $UNIT_PATH"
  fi
else
  warn "systemd not available; skipping service removal"
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

# --- Config ------------------------------------------------------------------
if [[ $KEEP_CONFIG -eq 1 ]]; then
  log "keeping config: $CONFIG_DIR (--keep-config)"
elif [[ -d "$CONFIG_DIR" ]]; then
  rm -rf "$CONFIG_DIR"
  log "removed config -> $CONFIG_DIR"
fi

log "done. docker and nixpacks left untouched."
