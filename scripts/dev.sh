#!/usr/bin/env bash
# Run the whole stack with reloading (`npm run dev`): the codegen watcher, the Go server
# on 127.0.0.1:43000 with dev-datadir/, and the web dev server on 127.0.0.1:43210. Sets
# the checkout up first (scripts/setup.sh) when it never was, or when
# package-lock.json changed since.
#
#   scripts/dev.sh                  everything
#   scripts/dev.sh go               one part: go, web or codegen (`npm run dev:<part>`)
source "$(dirname -- "${BASH_SOURCE[0]}")/lib.sh"
help_option "$@"
case ${1:-} in
  "") script=dev ;;
  go | web | codegen) script=dev:$1 ;;
  *) die "unknown part: $1 (see --help)" 2 ;;
esac

if needs_setup; then "$ROOT/scripts/setup.sh"; fi
cd "$ROOT"
exec npm run "$script"
