# What seanime needs from the system, for scripts/lib.sh. The npm scripts in package.json
# stay the cross-platform interface; these scripts are the Arch one.
# shellcheck shell=bash

NAME=seanime
# The pacman packages setup, dev, build and install need (`scripts/setup.sh --deps`).
# ffmpeg is for on-the-fly transcoding at run time.
DEPS=(git go nodejs npm ffmpeg)

# Whether the checkout needs scripts/setup.sh: never set up, or package-lock.json changed since.
needs_setup() { outdated "$ROOT/node_modules/.package-lock.json" "$ROOT/package-lock.json"; }

# Run an npm command in the checkout, its output on stderr.
npm_in_root() { (cd "$ROOT" && npm "$@" >&2); }
