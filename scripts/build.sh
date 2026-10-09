#!/usr/bin/env bash
# Build the server binary, the web interface embedded, into dist/seanime (`npm run
# build`: clean, codegen, web, embed, static Go binary) and print its path.
#
#   scripts/build.sh
source "$(dirname -- "${BASH_SOURCE[0]}")/lib.sh"
help_option "$@"
[[ $# -eq 0 ]] || die "build.sh takes no arguments (see --help)" 2

require_packages git go nodejs npm
if needs_setup; then "$ROOT/scripts/setup.sh"; fi
npm_in_root run build
print_existing "$ROOT/dist/seanime"
