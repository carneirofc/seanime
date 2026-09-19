#!/bin/sh
#
# Installs a built Seanime server binary on Linux — the counterpart of
# install-windows.ps1.
#
# Copies the binary into an install prefix, installs a launcher, an XDG desktop
# entry with hicolor icons (so it shows up in the KDE/GNOME application menu), and
# optionally a desktop icon, a PATH entry and a systemd service.
#
# The binary is expected to already exist. Run `npm run build` first (or pass
# --build-first) to produce it at dist/seanime.
#
# Three shapes, one script:
#
#   ./install-linux.sh                                  per-user desktop install
#   sudo ./install-linux.sh --system --systemd          host service install
#   ./install-linux.sh --client-only --server-url URL   menu entry for a remote server
#
# See installer/linux/README.md for what lands where.

set -eu

SCRIPT_PATH=$0
case "$SCRIPT_PATH" in
*/*) ROOT_DIR=$(CDPATH= cd -- "${SCRIPT_PATH%/*}" && pwd) ;;
*) ROOT_DIR=$(pwd) ;;
esac
TEMPLATE_DIR="${ROOT_DIR}/installer/linux"
ICON_SOURCE="${ROOT_DIR}/internal/icon/logo.png"
CONSTANTS_FILE="${ROOT_DIR}/internal/constants/constants.go"

APP_ID="seanime"
UNIT_NAME="seanime.service"
SERVICE_USER="seanime"
ICON_SIZES="16 22 24 32 48 64 128 256"

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	C_INFO=$(printf '\033[36m')
	C_WARN=$(printf '\033[33m')
	C_ERR=$(printf '\033[31m')
	C_OFF=$(printf '\033[0m')
else
	C_INFO='' C_WARN='' C_ERR='' C_OFF=''
fi

_log() {
	printf '%s[%s] [%s] %s%s\n' "$2" "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$3" "$C_OFF"
}
info() { _log INFO "$C_INFO" "$*"; }
warn() { _log WARN "$C_WARN" "$*" >&2; }
die() {
	_log ERROR "$C_ERR" "$*" >&2
	exit 1
}

usage() {
	cat <<'EOF'
Install Seanime on Linux.

Usage: install-linux.sh [flags]

Flags:
  --binary PATH          server binary to install (default: dist/seanime, then
                         dist/seanime-linux-*, then ./seanime)
  --prefix DIR           install prefix (default: ~/.local, or /usr/local with
                         --system). Binary goes to <prefix>/bin, desktop entry and
                         icons to <prefix>/share
  --datadir DIR          data directory the launcher passes to --datadir
                         (default: ~/.config/Seanime, or /var/lib/seanime with
                         --system). Must be an absolute path
  --host ADDR            bind address recorded in the launcher env file (default 127.0.0.1)
  --port N               port recorded in the launcher env file (default 43211)
  --build-first          run `npm run build` before installing
  --add-to-path          make <prefix>/bin available on PATH if it is not already
  --desktop-icon         also put a launcher on the desktop
  --no-menu-entry        do not install the application menu entry
  --server-url URL       also install a "Seanime (remote)" entry that opens URL
  --client-only          install no server binary; requires --server-url
  --system               install system-wide with a dedicated service user (needs root)
  --systemd              install a systemd service (a --user unit; implied by --system)
  --uninstall            remove everything this script installed, keeping user data
  -h, --help             show this message
EOF
}

# ---------------------------------------------------------------------------
# Options
# ---------------------------------------------------------------------------

OPT_BINARY=''
OPT_PREFIX=''
OPT_DATADIR=''
OPT_HOST=''
OPT_PORT=''
OPT_SERVER_URL=''
DO_BUILD=0
DO_PATH=0
DO_DESKTOP_ICON=0
DO_MENU_ENTRY=1
DO_CLIENT_ONLY=0
DO_SYSTEM=0
DO_SYSTEMD=0
DO_UNINSTALL=0

need_value() {
	[ "$2" -gt 0 ] || die "$1 requires a value."
}

while [ $# -gt 0 ]; do
	case "$1" in
	--binary) need_value "$1" $(($# - 1)); OPT_BINARY=$2; shift 2 ;;
	--binary=*) OPT_BINARY=${1#*=}; shift ;;
	--prefix) need_value "$1" $(($# - 1)); OPT_PREFIX=$2; shift 2 ;;
	--prefix=*) OPT_PREFIX=${1#*=}; shift ;;
	--datadir) need_value "$1" $(($# - 1)); OPT_DATADIR=$2; shift 2 ;;
	--datadir=*) OPT_DATADIR=${1#*=}; shift ;;
	--host) need_value "$1" $(($# - 1)); OPT_HOST=$2; shift 2 ;;
	--host=*) OPT_HOST=${1#*=}; shift ;;
	--port) need_value "$1" $(($# - 1)); OPT_PORT=$2; shift 2 ;;
	--port=*) OPT_PORT=${1#*=}; shift ;;
	--server-url) need_value "$1" $(($# - 1)); OPT_SERVER_URL=$2; shift 2 ;;
	--server-url=*) OPT_SERVER_URL=${1#*=}; shift ;;
	--build-first) DO_BUILD=1; shift ;;
	--add-to-path) DO_PATH=1; shift ;;
	--desktop-icon) DO_DESKTOP_ICON=1; shift ;;
	--no-menu-entry) DO_MENU_ENTRY=0; shift ;;
	--client-only) DO_CLIENT_ONLY=1; shift ;;
	--system) DO_SYSTEM=1; shift ;;
	--systemd) DO_SYSTEMD=1; shift ;;
	--uninstall) DO_UNINSTALL=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "Unknown option: $1" ;;
	esac
done

[ "$DO_CLIENT_ONLY" -eq 0 ] || [ -n "$OPT_SERVER_URL" ] ||
	die "--client-only needs --server-url: without a binary there is nothing else to open."
[ "$DO_SYSTEM" -eq 0 ] || [ "$(id -u)" -eq 0 ] ||
	die "--system installs outside your home directory; re-run it with sudo."
[ "$DO_SYSTEM" -eq 0 ] || [ "$DO_CLIENT_ONLY" -eq 0 ] ||
	die "--client-only is a per-user install; it has nothing to do system-wide."

[ "$DO_SYSTEM" -eq 0 ] || DO_SYSTEMD=1

case "${OPT_PORT:-43211}" in
'' | *[!0-9]*) die "--port must be a number: ${OPT_PORT}" ;;
esac

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

XDG_CONFIG_DIR=${XDG_CONFIG_HOME:-$HOME/.config}

if [ "$DO_SYSTEM" -eq 1 ]; then
	PREFIX=${OPT_PREFIX:-/usr/local}
	DATA_DIR=${OPT_DATADIR:-/var/lib/seanime}
	ENV_FILE=/etc/seanime/seanime.env
	UNIT_DIR=/etc/systemd/system
	UNIT_TEMPLATE="${TEMPLATE_DIR}/seanime.system.service.in"
	LAUNCH_MODE=system
else
	PREFIX=${OPT_PREFIX:-$HOME/.local}
	# The server's own default when --datadir is omitted (os.UserConfigDir()/Seanime).
	DATA_DIR=${OPT_DATADIR:-$XDG_CONFIG_DIR/Seanime}
	ENV_FILE="${XDG_CONFIG_DIR}/seanime/launcher.env"
	UNIT_DIR="${XDG_CONFIG_DIR}/systemd/user"
	UNIT_TEMPLATE="${TEMPLATE_DIR}/seanime.user.service.in"
	LAUNCH_MODE=user
fi
if [ "$DO_CLIENT_ONLY" -eq 1 ]; then
	LAUNCH_MODE=remote
fi

case "$PREFIX" in /*) ;; *) die "--prefix must be an absolute path: ${PREFIX}" ;; esac
# The server rejects a data directory that names no directory of its own, so catching
# a relative value here beats failing at first launch (internal/core/config.go).
case "$DATA_DIR" in /*) ;; *) die "--datadir must be an absolute path: ${DATA_DIR}" ;; esac

BIN_DIR="${PREFIX}/bin"
SHARE_DIR="${PREFIX}/share"
APP_DIR="${SHARE_DIR}/applications"
ICON_DIR="${SHARE_DIR}/icons/hicolor"
PIXMAP_DIR="${SHARE_DIR}/pixmaps"
DOC_DIR="${SHARE_DIR}/doc/seanime"

BIN_DEST="${BIN_DIR}/seanime"
LAUNCHER_DEST="${BIN_DIR}/seanime-launch"
DESKTOP_DEST="${APP_DIR}/${APP_ID}.desktop"
REMOTE_DESKTOP_DEST="${APP_DIR}/${APP_ID}-remote.desktop"
UNIT_DEST="${UNIT_DIR}/${UNIT_NAME}"
PATH_CONF="${XDG_CONFIG_DIR}/environment.d/10-seanime.conf"

HOST=${OPT_HOST:-127.0.0.1}
PORT=${OPT_PORT:-43211}
REMOTE_URL=$OPT_SERVER_URL

VERSION=unknown
if [ -r "$CONSTANTS_FILE" ]; then
	# Same single source of truth installer/seanime.iss reads at compile time.
	parsed=$(sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$CONSTANTS_FILE" | head -n 1)
	if [ -n "$parsed" ]; then VERSION=$parsed; fi
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

have() { command -v "$1" >/dev/null 2>&1; }

render() {
	# render <template> <destination> <mode>
	src=$1 dest=$2 mode=$3
	[ -r "$src" ] || die "Missing installer template: ${src}"
	mkdir -p "$(dirname "$dest")"
	sed \
		-e "s|@BIN@|${BIN_DEST}|g" \
		-e "s|@LAUNCHER@|${LAUNCHER_DEST}|g" \
		-e "s|@DATADIR@|${DATA_DIR}|g" \
		-e "s|@ENVFILE@|${ENV_FILE}|g" \
		-e "s|@VERSION@|${VERSION}|g" \
		-e "s|@URL@|${REMOTE_URL}|g" \
		-e "s|@USER@|${SERVICE_USER}|g" \
		-e "s|@MODE@|${LAUNCH_MODE}|g" \
		-e "s|@DOCDIR@|${DOC_DIR}|g" \
		-e "s|@HOST@|${HOST}|g" \
		-e "s|@PORT@|${PORT}|g" \
		"$src" >"${dest}.new"
	mv -f "${dest}.new" "$dest"
	chmod "$mode" "$dest"
}

refresh_caches() {
	if have update-desktop-database; then
		update-desktop-database "$APP_DIR" >/dev/null 2>&1 || true
	fi
	# gtk-update-icon-cache hard-fails on a theme directory with no index.theme, and a
	# per-user ~/.local/share/icons/hicolor usually has none. It is only needed by GTK
	# apps — Qt/KDE does not read the cache — so skipping it is harmless.
	if have gtk-update-icon-cache && [ -f "${ICON_DIR}/index.theme" ]; then
		gtk-update-icon-cache -qtf "$ICON_DIR" >/dev/null 2>&1 || true
	fi
	# Plasma rebuilds ksycoca on its own, but this makes the entry appear now rather
	# than after the next login. It needs a session bus, so never under --system.
	if [ "$DO_SYSTEM" -eq 0 ]; then
		for kb in kbuildsycoca6 kbuildsycoca5; do
			if have "$kb"; then
				"$kb" >/dev/null 2>&1 || true
				break
			fi
		done
	fi
}

desktop_dir() {
	if have xdg-user-dir; then
		d=$(xdg-user-dir DESKTOP 2>/dev/null || true)
		[ -n "$d" ] && [ "$d" != "$HOME" ] && { printf '%s\n' "$d"; return 0; }
	fi
	[ -d "$HOME/Desktop" ] && { printf '%s\n' "$HOME/Desktop"; return 0; }
	return 1
}

# ---------------------------------------------------------------------------
# Uninstall
# ---------------------------------------------------------------------------

do_uninstall() {
	info "Uninstalling Seanime (prefix: ${PREFIX})"

	if [ -f "$UNIT_DEST" ] && have systemctl; then
		if [ "$DO_SYSTEM" -eq 1 ]; then
			systemctl disable --now "$UNIT_NAME" >/dev/null 2>&1 || true
		else
			systemctl --user disable --now "$UNIT_NAME" >/dev/null 2>&1 || true
		fi
	fi
	rm -f "$UNIT_DEST"
	if have systemctl; then
		if [ "$DO_SYSTEM" -eq 1 ]; then
			systemctl daemon-reload >/dev/null 2>&1 || true
		else
			systemctl --user daemon-reload >/dev/null 2>&1 || true
		fi
	fi

	rm -f "$BIN_DEST" "$LAUNCHER_DEST" "$DESKTOP_DEST" "$REMOTE_DESKTOP_DEST" "$PATH_CONF"
	rm -f "${PIXMAP_DIR}/${APP_ID}.png"
	for size in $ICON_SIZES; do
		rm -f "${ICON_DIR}/${size}x${size}/apps/${APP_ID}.png"
	done
	if dd=$(desktop_dir); then
		rm -f "${dd}/${APP_ID}.desktop" "${dd}/${APP_ID}-remote.desktop"
	fi
	refresh_caches

	info "Removed the binary, launcher, menu entries, icons and service unit."
	info "Kept on purpose:"
	info "  Data directory : ${DATA_DIR}"
	info "  Settings       : ${ENV_FILE}"
	if [ "$DO_SYSTEM" -eq 1 ] && id -u "$SERVICE_USER" >/dev/null 2>&1; then
		info "  Service user   : ${SERVICE_USER} (it owns files; remove it yourself if you want it gone)"
	fi
}

if [ "$DO_UNINSTALL" -eq 1 ]; then
	do_uninstall
	exit 0
fi

# ---------------------------------------------------------------------------
# Resolve the binary
# ---------------------------------------------------------------------------

if [ "$DO_BUILD" -eq 1 ]; then
	have npm || die "npm was not found on PATH; cannot --build-first."
	info "Building Seanime (npm run build)..."
	(cd "$ROOT_DIR" && npm run build) || die "npm run build failed."
fi

SOURCE_BINARY=''
if [ "$DO_CLIENT_ONLY" -eq 0 ]; then
	if [ -n "$OPT_BINARY" ]; then
		case "$OPT_BINARY" in
		/*) SOURCE_BINARY=$OPT_BINARY ;;
		*) SOURCE_BINARY="${ROOT_DIR}/${OPT_BINARY}" ;;
		esac
		[ -f "$SOURCE_BINARY" ] || die "Binary not found at --binary: ${SOURCE_BINARY}"
	else
		for candidate in \
			"${ROOT_DIR}/dist/seanime" \
			"${ROOT_DIR}/dist/seanime-linux-amd64" \
			"${ROOT_DIR}/dist/seanime-linux-arm64" \
			"${ROOT_DIR}/seanime"; do
			if [ -f "$candidate" ]; then
				SOURCE_BINARY=$candidate
				break
			fi
		done
		[ -n "$SOURCE_BINARY" ] ||
			die "No Seanime binary found under ${ROOT_DIR}/dist. Build it with 'npm run build' or pass --build-first."
	fi

	# Cheap guard against pointing at a script, an archive or the wrong file entirely.
	magic=$(od -An -tx1 -N4 "$SOURCE_BINARY" 2>/dev/null | tr -d ' \n')
	[ "$magic" = "7f454c46" ] || die "Not an ELF executable: ${SOURCE_BINARY}"

	info "Source binary: ${SOURCE_BINARY}"
fi

[ -d "$TEMPLATE_DIR" ] || die "Installer templates not found at ${TEMPLATE_DIR}; run this script from a Seanime checkout."

# ---------------------------------------------------------------------------
# Service user (system installs only)
# ---------------------------------------------------------------------------

if [ "$DO_SYSTEM" -eq 1 ] && ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
	have useradd || die "useradd not found; create the '${SERVICE_USER}' system user yourself and re-run."
	nologin_shell=/bin/false
	for shell in /usr/sbin/nologin /sbin/nologin /usr/bin/nologin; do
		if [ -x "$shell" ]; then
			nologin_shell=$shell
			break
		fi
	done
	useradd --system --home-dir "$DATA_DIR" --shell "$nologin_shell" \
		--comment "Seanime media server" "$SERVICE_USER"
	info "Created system user: ${SERVICE_USER}"
fi

# ---------------------------------------------------------------------------
# Install
# ---------------------------------------------------------------------------

mkdir -p "$BIN_DIR" "$APP_DIR" "$(dirname "$ENV_FILE")"
# A client-only install runs no server, so it has no state to keep.
if [ "$DO_CLIENT_ONLY" -eq 0 ]; then
	mkdir -p "$DATA_DIR"
fi

if [ "$DO_CLIENT_ONLY" -eq 0 ]; then
	cp -f "$SOURCE_BINARY" "${BIN_DEST}.new"
	mv -f "${BIN_DEST}.new" "$BIN_DEST"
	chmod 0755 "$BIN_DEST"
	size=$(( $(wc -c <"$BIN_DEST") / 1048576 ))
	info "Installed binary: ${BIN_DEST} (${size} MB)"
else
	BIN_DEST=''
fi

render "${TEMPLATE_DIR}/seanime-launch.in" "$LAUNCHER_DEST" 0755
info "Installed launcher: ${LAUNCHER_DEST}"

# Settings file. Never clobbered: a reinstall must not reset a hand-edited URL.
#
# Note what is deliberately NOT in here: SEANIME_SERVER_HOST and SEANIME_SERVER_PORT.
# The server rewrites config.toml whenever either differs from the stored value
# (internal/core/config.go:211), so putting them in a file that both the launcher and
# the systemd unit read would overwrite a hand-edited config on every start. The
# SEANIME_LAUNCH_* names below are read only by the launcher, and only as a fallback
# for the first run, before config.toml exists.
if [ -e "$ENV_FILE" ]; then
	info "Kept existing settings: ${ENV_FILE}"
	if [ -n "$REMOTE_URL" ] && ! grep -q "^SEANIME_REMOTE_URL=${REMOTE_URL}\$" "$ENV_FILE" 2>/dev/null; then
		warn "SEANIME_REMOTE_URL in ${ENV_FILE} was left as-is; edit it by hand to use ${REMOTE_URL}."
	fi
else
	cat >"$ENV_FILE" <<EOF
# Seanime launcher settings, read by seanime-launch and by the systemd unit.
# Written once at install time; the installer never overwrites this file.

SEANIME_DATA_DIR=${DATA_DIR}

# Where the launcher looks for the web interface before <datadir>/config.toml exists.
# Once it does, config.toml is authoritative and these are ignored — change the
# address there, not here.
SEANIME_LAUNCH_HOST=${HOST}
SEANIME_LAUNCH_PORT=${PORT}

# How long to wait for the server to answer /api/v1/status before giving up. A first
# run builds the database and caches before it starts listening.
SEANIME_LAUNCH_TIMEOUT=90

# Remote server opened by the "Seanime (remote)" menu entry.
SEANIME_REMOTE_URL=${REMOTE_URL}

# How the web interface is opened. Default: xdg-open. A "%u" is replaced by the URL,
# which is how you get a chromeless app window instead of a browser tab:
# SEANIME_BROWSER_CMD=chromium --app=%u

# Terminal emulator for the "Start in a terminal" action, if detection picks wrong.
# SEANIME_TERMINAL=
EOF
	chmod 0644 "$ENV_FILE"
	info "Wrote settings: ${ENV_FILE}"
fi

# A non-default address has to reach the server through config.toml, for the same
# reason: passing it as a flag or an env var makes the server rewrite that file on
# every launch. Seeding it once, only when there is no config yet, avoids the fight.
if [ "$DO_SYSTEM" -eq 0 ] && [ -n "${OPT_HOST}${OPT_PORT}" ] && [ ! -e "${DATA_DIR}/config.toml" ]; then
	cat >"${DATA_DIR}/config.toml" <<EOF
[server]
host = "${HOST}"
port = ${PORT}
EOF
	chmod 0600 "${DATA_DIR}/config.toml"
	info "Seeded configuration: ${DATA_DIR}/config.toml (${HOST}:${PORT})"
fi

# --- Icons -----------------------------------------------------------------

install_icons() {
	[ -r "$ICON_SOURCE" ] || {
		warn "Icon source not found at ${ICON_SOURCE}; the menu entry will use a generic icon."
		return 0
	}

	converter=''
	if have magick; then
		converter=magick
	elif have convert; then
		converter=convert
	fi

	if [ -z "$converter" ]; then
		# No ImageMagick: hicolor directories are size-specific and the source is
		# 439x439, so it goes to the legacy unsized location instead, which both
		# KDE and GTK still search by icon name.
		mkdir -p "$PIXMAP_DIR"
		cp -f "$ICON_SOURCE" "${PIXMAP_DIR}/${APP_ID}.png"
		chmod 0644 "${PIXMAP_DIR}/${APP_ID}.png"
		info "Installed icon: ${PIXMAP_DIR}/${APP_ID}.png (install ImageMagick for proper hicolor sizes)"
		return 0
	fi

	for size in $ICON_SIZES; do
		dir="${ICON_DIR}/${size}x${size}/apps"
		mkdir -p "$dir"
		"$converter" "$ICON_SOURCE" -resize "${size}x${size}" "${dir}/${APP_ID}.png"
		chmod 0644 "${dir}/${APP_ID}.png"
	done
	sizes_label=$(for s in $ICON_SIZES; do printf '%sx%s,' "$s" "$s"; done)
	info "Installed icons: ${ICON_DIR}/{${sizes_label%,}}/apps/${APP_ID}.png"
}
install_icons

# --- Desktop entries -------------------------------------------------------

if [ "$DO_MENU_ENTRY" -eq 1 ] && [ "$DO_CLIENT_ONLY" -eq 0 ]; then
	render "${TEMPLATE_DIR}/seanime.desktop.in" "$DESKTOP_DEST" 0644
	info "Installed menu entry: ${DESKTOP_DEST}"
fi

if [ -n "$REMOTE_URL" ]; then
	render "${TEMPLATE_DIR}/seanime-remote.desktop.in" "$REMOTE_DESKTOP_DEST" 0644
	info "Installed remote entry: ${REMOTE_DESKTOP_DEST} -> ${REMOTE_URL}"
fi

if [ -n "${XDG_DATA_HOME:-}" ] && [ "$DO_SYSTEM" -eq 0 ] && [ "${XDG_DATA_HOME}" != "$SHARE_DIR" ]; then
	warn "XDG_DATA_HOME is ${XDG_DATA_HOME} but the entry went to ${SHARE_DIR}."
	warn "Re-run with --prefix \"\$(dirname \"\$XDG_DATA_HOME\")\" if it does not appear in your menu."
fi

refresh_caches

# --- Desktop icon ----------------------------------------------------------

if [ "$DO_DESKTOP_ICON" -eq 1 ]; then
	if dd=$(desktop_dir); then
		src=$DESKTOP_DEST
		[ -f "$src" ] || src=$REMOTE_DESKTOP_DEST
		if [ -f "$src" ]; then
			cp -f "$src" "${dd}/$(basename "$src")"
			# Plasma refuses to run a non-executable .desktop dropped on the desktop
			# and shows an "untrusted" prompt; the exec bit is what clears it.
			chmod 0755 "${dd}/$(basename "$src")"
			info "Created desktop icon: ${dd}/$(basename "$src")"
		fi
	else
		warn "No desktop directory found; skipping --desktop-icon."
	fi
fi

# --- PATH ------------------------------------------------------------------

if [ "$DO_PATH" -eq 1 ]; then
	case ":${PATH}:" in
	*":${BIN_DIR}:"*)
		info "Already on PATH: ${BIN_DIR}"
		;;
	*)
		mkdir -p "$(dirname "$PATH_CONF")"
		printf 'PATH=%s:${PATH}\n' "$BIN_DIR" >"$PATH_CONF"
		info "Wrote ${PATH_CONF}"
		warn "environment.d applies to the graphical session and systemd user services at"
		warn "your next login — not to this shell. For now: export PATH=\"${BIN_DIR}:\$PATH\""
		;;
	esac
fi

# --- System-only setup (must precede starting the service) -----------------

if [ "$DO_SYSTEM" -eq 1 ]; then
	mkdir -p "$DOC_DIR"
	for doc in WEB_DEPLOYMENT.md config.example.toml; do
		if [ -r "${ROOT_DIR}/${doc}" ]; then cp -f "${ROOT_DIR}/${doc}" "${DOC_DIR}/${doc}"; fi
	done

	config_dest="${DATA_DIR}/config.toml"
	if [ -e "$config_dest" ]; then
		info "Kept existing configuration: ${config_dest}"
	else
		render "${TEMPLATE_DIR}/server-config.toml.in" "$config_dest" 0640
		info "Seeded configuration: ${config_dest} (capabilities = [], loopback only)"
	fi
	chown -R "${SERVICE_USER}:${SERVICE_USER}" "$DATA_DIR"
	chown "${SERVICE_USER}:${SERVICE_USER}" "$ENV_FILE" 2>/dev/null || true
	# 0700, owned by the service user — the "restrict data-dir permissions" item in
	# WEB_DEPLOYMENT.md. It holds the database, sessions, logs and extensions.
	chmod 0700 "$DATA_DIR"
fi

# --- systemd ---------------------------------------------------------------

if [ "$DO_SYSTEMD" -eq 1 ]; then
	if [ "$DO_CLIENT_ONLY" -eq 1 ]; then
		warn "--client-only installs no server; skipping the systemd unit."
	elif ! have systemctl; then
		warn "systemctl not found; skipping the systemd unit."
	else
		mkdir -p "$UNIT_DIR"
		render "$UNIT_TEMPLATE" "$UNIT_DEST" 0644
		info "Installed unit: ${UNIT_DEST}"
		if [ "$DO_SYSTEM" -eq 1 ]; then
			systemctl daemon-reload
			systemctl enable --now "$UNIT_NAME"
			info "Enabled and started ${UNIT_NAME}"
		else
			systemctl --user daemon-reload
			systemctl --user enable --now "$UNIT_NAME"
			info "Enabled and started ${UNIT_NAME} (user)"
			info "For it to run when you are not logged in: loginctl enable-linger \"\$USER\""
		fi
	fi
fi

# --- Runtime dependency check ----------------------------------------------

have ffmpeg || warn "ffmpeg was not found on PATH. On-the-fly transcoding needs it (package: ffmpeg)."

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

info "Seanime ${VERSION} installed."
if [ -n "$BIN_DEST" ]; then info "  Binary    : ${BIN_DEST}"; fi
info "  Launcher  : ${LAUNCHER_DEST}"
info "  Data dir  : ${DATA_DIR}"
info "  Settings  : ${ENV_FILE}"
if [ -n "$REMOTE_URL" ]; then info "  Remote    : ${REMOTE_URL}"; fi

if [ "$DO_SYSTEM" -eq 1 ]; then
	info "Manage it with: systemctl status ${UNIT_NAME}"
	info "Before exposing it beyond this host, read ${DOC_DIR}/WEB_DEPLOYMENT.md."
	info "Grant a media root to the unit with a ReadWritePaths= line in ${UNIT_DEST}."
elif [ "$DO_CLIENT_ONLY" -eq 1 ]; then
	info "Launch \"Seanime (remote)\" from the application menu."
else
	info "Launch Seanime from the application menu, or run: ${LAUNCHER_DEST}"
	info "To watch the logs live, use the entry's \"Start in a terminal\" action."
fi
