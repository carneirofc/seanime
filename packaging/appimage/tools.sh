# shellcheck shell=sh disable=SC2034 # the *_SHA256_* pins are read indirectly via eval
# Pinned third-party tools for the AppImage build, sourced by build.sh and verify.sh.
#
# Every download is checked against the SHA-256 below before it is executed. To bump a
# tool, change its version and hashes together; the digests are listed on each GitHub
# release page (or: gh api repos/<owner>/<repo>/releases/tags/<tag> --jq '.assets[].digest').

APPIMAGETOOL_VERSION=1.9.1
APPIMAGETOOL_SHA256_x86_64=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
APPIMAGETOOL_SHA256_aarch64=f0837e7448a0c1e4e650a93bb3e85802546e60654ef287576f46c71c126a9158

# The AppImage runtime is passed to appimagetool explicitly; otherwise it downloads the
# unpinned "continuous" runtime at build time.
RUNTIME_VERSION=20251108
RUNTIME_SHA256_x86_64=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d
RUNTIME_SHA256_aarch64=00cbdfcf917cc6c0ff6d3347d59e0ca1f7f45a6df1a428a0d6d8a78664d87444

GITLEAKS_VERSION=8.30.1
GITLEAKS_SHA256_x64=551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb
GITLEAKS_SHA256_arm64=e4a487ee7ccd7d3a7f7ec08657610aa3606637dab924210b3aee62570fb4b080

REPO_ROOT=$(cd "$(dirname "$0")/../.." && pwd)
TOOLS_DIR=${APPIMAGE_TOOLS_DIR:-$REPO_ROOT/dist/appimage-tools}

die() {
	printf '%s: %s\n' "$(basename "$0")" "$*" >&2
	exit 1
}

host_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo x86_64 ;;
	aarch64 | arm64) echo aarch64 ;;
	*) die "unsupported host architecture: $(uname -m)" ;;
	esac
}

# fetch URL SHA256 DEST — download once, verify every time.
fetch() {
	url=$1 sum=$2 dest=$3
	mkdir -p "$(dirname "$dest")"
	if [ ! -f "$dest" ]; then
		curl -fsSL --retry 3 -o "$dest.part" "$url" || die "download failed: $url"
		mv "$dest.part" "$dest"
	fi
	if ! printf '%s  %s\n' "$sum" "$dest" | sha256sum -c --status; then
		rm -f "$dest"
		die "checksum mismatch for $url"
	fi
}

appimagetool_path() {
	a=$(host_arch)
	eval "sum=\$APPIMAGETOOL_SHA256_$a"
	dest="$TOOLS_DIR/appimagetool-$APPIMAGETOOL_VERSION-$a.AppImage"
	fetch "https://github.com/AppImage/appimagetool/releases/download/$APPIMAGETOOL_VERSION/appimagetool-$a.AppImage" "$sum" "$dest"
	chmod +x "$dest"
	echo "$dest"
}

# runtime_path TARGET_ARCH
runtime_path() {
	eval "sum=\$RUNTIME_SHA256_$1"
	[ -n "$sum" ] || die "no runtime pinned for $1"
	dest="$TOOLS_DIR/runtime-$RUNTIME_VERSION-$1"
	fetch "https://github.com/AppImage/type2-runtime/releases/download/$RUNTIME_VERSION/runtime-$1" "$sum" "$dest"
	echo "$dest"
}

gitleaks_path() {
	if [ -n "${GITLEAKS:-}" ]; then
		echo "$GITLEAKS"
		return
	fi
	case "$(host_arch)" in
	x86_64) a=x64 ;;
	aarch64) a=arm64 ;;
	esac
	eval "sum=\$GITLEAKS_SHA256_$a"
	tarball="$TOOLS_DIR/gitleaks_${GITLEAKS_VERSION}_linux_$a.tar.gz"
	fetch "https://github.com/gitleaks/gitleaks/releases/download/v$GITLEAKS_VERSION/gitleaks_${GITLEAKS_VERSION}_linux_$a.tar.gz" "$sum" "$tarball"
	bin="$TOOLS_DIR/gitleaks-$GITLEAKS_VERSION"
	if [ ! -x "$bin" ]; then
		tar -xzf "$tarball" -C "$TOOLS_DIR" gitleaks
		mv "$TOOLS_DIR/gitleaks" "$bin"
	fi
	echo "$bin"
}
