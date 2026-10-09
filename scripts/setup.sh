#!/usr/bin/env bash
# Prepare the checkout for development and builds: the npm packages exactly as
# package-lock.json pins them (`npm ci`, the web workspace included) and the Go modules.
# Safe to run again.
#
#   scripts/setup.sh                set it up
#   scripts/setup.sh --deps         the pacman packages it needs, one per line
source "$(dirname -- "${BASH_SOURCE[0]}")/lib.sh"
help_option "$@"
case ${1:-} in
  "") ;;
  --deps) print_packages "${DEPS[@]}"; exit 0 ;;
  *) die "unknown option: $1 (see --help)" 2 ;;
esac

require_packages "${DEPS[@]}"
log "npm packages"
npm_in_root ci --no-audit --no-fund
log "Go modules"
(cd "$ROOT" && go mod download >&2)
