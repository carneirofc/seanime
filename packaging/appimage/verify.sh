#!/bin/sh
# Check a finished AppImage for anything that must not ship: unexpected files and
# secrets (private keys, tokens, credentials).
#
#   packaging/appimage/verify.sh path/to/seanime-<v>_Linux_<arch>.AppImage
#
# Works for any target architecture: the squashfs payload is read with unsquashfs
# rather than by running the image. Exits non-zero on any finding.

set -eu

. "$(dirname "$0")/tools.sh"

IMAGE=${1:?usage: verify.sh IMAGE}
[ -f "$IMAGE" ] || die "not found: $IMAGE"
command -v unsquashfs >/dev/null 2>&1 || die "unsquashfs not found (install squashfs-tools)"
command -v strings >/dev/null 2>&1 || die "strings not found (install binutils)"
GITLEAKS_BIN=$(gitleaks_path)

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM
ROOT="$WORK/root"

# The payload starts where the runtime ELF ends: after its section header table.
OFFSET=$(python3 - "$IMAGE" <<'EOF'
import struct, sys
with open(sys.argv[1], "rb") as f:
    h = f.read(64)
assert h[:4] == b"\x7fELF", "not an ELF file"
if h[4] == 2:
    shoff, = struct.unpack_from("<Q", h, 0x28); shentsize, shnum = struct.unpack_from("<HH", h, 0x3A)
else:
    shoff, = struct.unpack_from("<I", h, 0x20); shentsize, shnum = struct.unpack_from("<HH", h, 0x2E)
print(shoff + shentsize * shnum)
EOF
)
unsquashfs -q -no-xattrs -o "$OFFSET" -d "$ROOT" "$IMAGE" >/dev/null

# 1. Exactly the files build.sh puts in, nothing else.
EXPECTED='.DirIcon
AppRun
seanime.desktop
seanime.png
usr/bin/seanime
usr/share/doc/seanime/LICENSE'
printf '%s\n' "$EXPECTED" | LC_ALL=C sort >"$WORK/expected"
(cd "$ROOT" && find . ! -type d | sed 's|^\./||' | LC_ALL=C sort) >"$WORK/actual"
if ! diff -u "$WORK/expected" "$WORK/actual" >&2; then
	die "AppImage contents differ from the expected file list (- expected, + found)"
fi

# 2. Secret scan. The binary embeds the web bundle, so its printable strings cover both
# the Go side and anything the frontend build inlined from the environment.
SCAN="$WORK/scan"
mkdir -p "$SCAN"
cp "$ROOT/AppRun" "$ROOT/seanime.desktop" "$SCAN/"
strings -n 8 "$ROOT/usr/bin/seanime" >"$SCAN/seanime.strings"
"$GITLEAKS_BIN" dir "$SCAN" --config "$REPO_ROOT/.gitleaks.toml" --no-banner --redact --exit-code 1 ||
	die "secret scan found possible credentials in $IMAGE"

# 3. PEM private-key blocks, which gitleaks also covers; checked by hand so a gitleaks
# rule change can never silently drop it.
if grep -q -- '-----BEGIN [A-Z ]*PRIVATE KEY-----' "$SCAN/seanime.strings" "$ROOT/AppRun"; then
	die "private key block found in $IMAGE"
fi

printf 'verified %s: file list ok, no secrets found\n' "$IMAGE"
