#!/bin/sh
# Secret scan of the source the release is built from. The release pipeline runs this
# before building anything; it is also safe to run locally.
#
#   packaging/appimage/scan-source.sh [BASE_REF]
#
# Always scans the checked-out tree. With BASE_REF (CI passes upstream/main), it also
# scans every commit in BASE_REF..HEAD — this fork's own history — because a secret
# that was committed and later deleted is still published with the repository.

set -eu

. "$(dirname "$0")/tools.sh"
GITLEAKS_BIN=$(gitleaks_path)
CONFIG="$REPO_ROOT/.gitleaks.toml"

# CI runs this on a fresh checkout. Locally, untracked files (node_modules, dist/, a dev
# data dir) are scanned too, which can report secrets that are not in the repository.
"$GITLEAKS_BIN" dir "$REPO_ROOT" --config "$CONFIG" --no-banner --redact --exit-code 1 ||
	die "secret scan of the working tree found possible credentials"

if [ -n "${1:-}" ]; then
	"$GITLEAKS_BIN" git "$REPO_ROOT" --config "$CONFIG" --no-banner --redact --exit-code 1 \
		--log-opts "$1..HEAD" ||
		die "secret scan of commits $1..HEAD found possible credentials"
fi

printf 'source secret scan: no findings\n'
