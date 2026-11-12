# What seanime needs from the system, for scripts/lib.sh. The pnpm scripts in package.json
# stay the cross-platform interface; these scripts are the Arch one.
# shellcheck shell=bash

NAME=seanime
# The pacman packages setup, dev, build and install need (`scripts/setup.sh --deps`).
# ffmpeg is for on-the-fly transcoding at run time.
DEPS=(git go nodejs pnpm ffmpeg)

# Whether the checkout needs scripts/setup.sh: never set up, or pnpm-lock.yaml changed since.
needs_setup() { outdated "$ROOT/node_modules/.modules.yaml" "$ROOT/pnpm-lock.yaml"; }

# Run a pnpm command in the checkout, its output on stderr.
pnpm_in_root() { (cd "$ROOT" && pnpm "$@" >&2); }
