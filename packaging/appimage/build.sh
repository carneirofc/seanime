#!/bin/sh
# Package a built Seanime server binary as an AppImage, then verify it.
#
#   packaging/appimage/build.sh --binary PATH --arch x86_64|aarch64 [--version V] [--outdir DIR]
#
# The binary must already embed the web UI (npm run build does both). Use the systray
# build (no -tags=nosystray): AppRun relies on its tray icon, and it is the variant the
# release ships as the standalone Linux download.
#
# The AppDir is assembled from an explicit list of files, never by copying a directory,
# so nothing from the working tree (config.toml, .env files, a dev data dir) can ride
# along. verify.sh then re-checks the finished image from the outside.

set -eu

. "$(dirname "$0")/tools.sh"
HERE="$REPO_ROOT/packaging/appimage"

BINARY='' ARCH='' VERSION='' OUTDIR="$REPO_ROOT/dist"
while [ "$#" -gt 0 ]; do
	case "$1" in
	--binary) BINARY=$2; shift 2 ;;
	--arch) ARCH=$2; shift 2 ;;
	--version) VERSION=$2; shift 2 ;;
	--outdir) OUTDIR=$2; shift 2 ;;
	-h | --help) sed -n '2,12p' "$0"; exit 0 ;;
	*) die "unknown argument: $1" ;;
	esac
done

[ -n "$BINARY" ] || die "--binary is required"
[ -f "$BINARY" ] || die "binary not found: $BINARY"
case "$ARCH" in
x86_64) RELEASE_ARCH=x86_64 ;;
aarch64) RELEASE_ARCH=arm64 ;;
*) die "--arch must be x86_64 or aarch64" ;;
esac
if [ -z "$VERSION" ]; then
	VERSION=$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$REPO_ROOT/internal/constants/constants.go" | head -n 1)
	[ -n "$VERSION" ] || die "could not read Version from internal/constants/constants.go"
fi

# Release asset naming matches the other Linux downloads (seanime-<v>_Linux_<arch>.tar.gz).
OUT="$OUTDIR/seanime-${VERSION}_Linux_${RELEASE_ARCH}.AppImage"

APPIMAGETOOL=$(appimagetool_path)
RUNTIME=$(runtime_path "$ARCH")

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM
APPDIR="$WORK/Seanime.AppDir"

install -Dm755 "$BINARY" "$APPDIR/usr/bin/seanime"
install -Dm755 "$HERE/AppRun" "$APPDIR/AppRun"
install -Dm644 "$HERE/seanime.desktop" "$APPDIR/seanime.desktop"
install -Dm644 "$REPO_ROOT/internal/icon/logo.png" "$APPDIR/seanime.png"
ln -s seanime.png "$APPDIR/.DirIcon"
install -Dm644 "$REPO_ROOT/LICENSE" "$APPDIR/usr/share/doc/seanime/LICENSE"
printf 'X-AppImage-Version=%s\n' "$VERSION" >>"$APPDIR/seanime.desktop"

# Reproducible squashfs timestamps: the commit time when building from git.
if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
	SOURCE_DATE_EPOCH=$(git -C "$REPO_ROOT" log -1 --format=%ct 2>/dev/null || date +%s)
fi
export SOURCE_DATE_EPOCH

mkdir -p "$OUTDIR"
rm -f "$OUT"
# APPIMAGE_EXTRACT_AND_RUN: appimagetool is itself an AppImage, and CI runners have no FUSE.
APPIMAGE_EXTRACT_AND_RUN=1 ARCH="$ARCH" "$APPIMAGETOOL" --no-appstream \
	--runtime-file "$RUNTIME" "$APPDIR" "$OUT"

# An image that fails verification is deleted, so it cannot be uploaded by mistake.
"$HERE/verify.sh" "$OUT" || {
	rm -f "$OUT"
	die "verification failed; $OUT was removed"
}

(cd "$OUTDIR" && sha256sum "$(basename "$OUT")" >"$(basename "$OUT").sha256")
printf 'built %s\n' "$OUT"
