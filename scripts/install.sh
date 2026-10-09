#!/usr/bin/env bash
# Build this checkout and install it for the current user with install-linux.sh: the
# binary in ~/.local/bin, the launcher, menu entry and icons in ~/.local/share. No sudo.
# Run it again to update. Arguments go to install-linux.sh (see `./install-linux.sh
# --help`); --system needs root, so run that one through sudo.
#
#   scripts/install.sh                  build, then install or update
#   scripts/install.sh --systemd        also a systemd user unit
#   scripts/install.sh --no-build       install dist/seanime as it is
#   scripts/install.sh --uninstall      remove it (the data directory is kept)
source "$(dirname -- "${BASH_SOURCE[0]}")/lib.sh"
help_option "$@"

build=true
args=()
for arg in "$@"; do
  case $arg in
    --no-build) build=false ;;
    --uninstall) build=false; args+=("$arg") ;;
    *) args+=("$arg") ;;
  esac
done

if $build; then
  "$ROOT/scripts/build.sh" >/dev/null
fi
binary=()
[[ " ${args[*]-} " == *" --uninstall "* ]] || binary=(--binary "$ROOT/dist/seanime")
sh "$ROOT/install-linux.sh" "${binary[@]}" "${args[@]}" >&2
