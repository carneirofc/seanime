# Linux installer

[`../../install-linux.sh`](../../install-linux.sh) is the Linux counterpart of
[`install-windows.ps1`](../../install-windows.ps1): it takes a built server binary and
turns it into an installed application — an entry in the KDE/GNOME application menu,
hicolor icons, an optional desktop icon and PATH entry, and an optional systemd
service.

The Seanime server binary already embeds the web UI, so there is nothing to install
besides the executable and the desktop integration around it.

The files in this directory are templates. `@PLACEHOLDER@` tokens are substituted at
install time with the paths you chose.

## Prerequisites

- A **built server binary** at `dist/seanime` — run `npm run build`, or pass
  `--build-first`.
- `ffmpeg` on `PATH` for on-the-fly transcoding. The installer warns if it is missing.
- Optional: `imagemagick` for crisp icons at every size (see [Icons](#icons-desktop-icon-path)),
  `curl` or `wget` so the launcher can tell when the server is ready.

## Installing

From the repository root:

```bash
# Build the app, then install it for the current user
npm run build:install

# Or, if the binary is already built
npm run install:linux
```

Those two run the installer with no options. **Flags cannot be passed through `npm
run`** — `run-script-os` re-invokes `npm run` to dispatch on the platform, and npm
rejects any flag it does not recognise before it ever reaches the script (`--prefix`
is worse: npm consumes it as its own). `npm run install:windows` has the same
limitation. To pass options, call the script directly:

```bash
# Per-user, no root, with a desktop icon
./install-linux.sh --desktop-icon

# Somewhere else entirely, building first
./install-linux.sh --build-first --prefix ~/apps/seanime --datadir /data/seanime

# A host service: dedicated user, hardened unit, everything denied by default
sudo ./install-linux.sh --system

# A machine that only consumes a server elsewhere
./install-linux.sh --client-only --server-url https://seanime.example.com

# Remove it again (your library and settings are kept)
./install-linux.sh --uninstall
```

## Flags

| Flag | Default | |
| --- | --- | --- |
| `--binary PATH` | `dist/seanime`, then `dist/seanime-linux-*`, then `./seanime` | source binary; relative paths resolve against the repo root |
| `--prefix DIR` | `~/.local` (`/usr/local` with `--system`) | binary in `<prefix>/bin`, entry and icons in `<prefix>/share` |
| `--datadir DIR` | `~/.config/Seanime` (`/var/lib/seanime` with `--system`) | must be absolute |
| `--host ADDR` / `--port N` | `127.0.0.1` / `43211` | seeded into `config.toml` on a first install; see [Address](#address) |
| `--build-first` | | run `npm run build` first |
| `--add-to-path` | | see [PATH](#icons-desktop-icon-path) |
| `--desktop-icon` | | also place a launcher on the desktop |
| `--no-menu-entry` | | skip the application menu entry |
| `--server-url URL` | | also install a "Seanime (remote)" entry |
| `--client-only` | | install no binary; requires `--server-url` |
| `--system` | | system-wide, with a service user and a systemd unit (needs root) |
| `--systemd` | implied by `--system` | per-user: install and enable a `--user` unit |
| `--uninstall` | | honours `--prefix` and `--system` |

## What gets installed where

| | per-user (default) | `--system` |
| --- | --- | --- |
| Binary | `~/.local/bin/seanime` | `/usr/local/bin/seanime` |
| Launcher | `~/.local/bin/seanime-launch` | `/usr/local/bin/seanime-launch` |
| Menu entry | `~/.local/share/applications/seanime.desktop` | `/usr/local/share/applications/…` |
| Icons | `~/.local/share/icons/hicolor/<n>x<n>/apps/seanime.png` | `/usr/local/share/icons/hicolor/…` |
| Data | `~/.config/Seanime` | `/var/lib/seanime`, owned by `seanime`, mode 0700 |
| Settings | `~/.config/seanime/launcher.env` | `/etc/seanime/seanime.env` |
| Service | `~/.config/systemd/user/seanime.service` | `/etc/systemd/system/seanime.service` |

Everything derives from `--prefix`, so `--prefix /tmp/x` gives you a fully
self-contained install you can delete in one `rm -rf`.

**This differs from Windows on purpose.** `install-windows.ps1` puts the data
directory next to the binary (`<install dir>\seanime_data_dir`). On Linux the default
is `~/.config/Seanime`, which is where the server itself would put it with no
`--datadir` at all (`os.UserConfigDir()/Seanime`, `internal/core/config.go`). Keeping
the server's own default means the install location and your library are independent
— you can reinstall to a different prefix without moving a byte of data.

## The launcher

The menu entry runs `seanime-launch`, not `seanime`: the server is headless, so a bare
`Exec=seanime` would look like nothing happened. With no arguments it:

1. opens the existing tab if a server already answers;
2. otherwise starts one (`systemctl --user start` under `--systemd`, else `setsid`, so it
   outlives the launcher and browser);
3. polls `GET /api/v1/status` for up to `SEANIME_LAUNCH_TIMEOUT` seconds (default 90);
4. opens the URL with `xdg-open`.

Other modes: `--terminal`, `--remote`, `--stop`, `--status`, `--url`, `--help`.
Closing the browser does not stop the server; use the menu's "Stop the server" action,
`seanime-launch --stop`, or `systemctl --user stop seanime`.

### Settings — `launcher.env`

Written once at install time and **never overwritten by a reinstall**, so a
hand-edited value survives.

```sh
SEANIME_DATA_DIR=/home/you/.config/Seanime
SEANIME_LAUNCH_HOST=127.0.0.1     # fallback only; config.toml wins once it exists
SEANIME_LAUNCH_PORT=43211
SEANIME_LAUNCH_TIMEOUT=90
SEANIME_REMOTE_URL=               # set by --server-url
# SEANIME_BROWSER_CMD=chromium --app=%u    # "%u" is replaced by the URL
# SEANIME_TERMINAL=konsole                 # if terminal detection picks wrong
```

### Address

`launcher.env` deliberately omits `SEANIME_SERVER_HOST`/`PORT`: the server rewrites
`config.toml` whenever those differ (`internal/core/config.go`). The launcher reads
`[server] host`/`port` from `<datadir>/config.toml` instead, so change the address there.

### Menu actions

Right-click the entry for **Start in a terminal**, **Stop the server** and **Open data
directory**. The terminal action calls `seanime-launch --terminal`, which detects an
emulator itself (`Terminal=true` is invalid in a `[Desktop Action]` group); set
`SEANIME_TERMINAL` if it picks wrong.

## Icons, desktop icon, PATH

- Icons: `internal/icon/logo.png` is downscaled to 16–256 px under `icons/hicolor/`. Without
  ImageMagick the raw PNG goes to `share/pixmaps/` instead. Cache refreshes are best-effort.
- `--desktop-icon` copies the entry to `xdg-user-dir DESKTOP` and marks it executable
  (Plasma requires that); skipped if there is no desktop directory.
- `--add-to-path` does nothing when `~/.local/bin` is already on `PATH`; otherwise it writes
  `~/.config/environment.d/10-seanime.conf`, which applies at next login and not to TTY or
  `ssh` sessions.

## systemd

### Per-user (`--systemd`)

Installed at `~/.config/systemd/user/seanime.service` and enabled immediately. It runs
when you log in; for it to run when you are not logged in:

```bash
loginctl enable-linger "$USER"
```

The unit is **deliberately unsandboxed**. A local desktop install resolves to *all*
privileged capabilities, because that is what the features you installed it for need:
spawning mpv/VLC, browsing the filesystem, opening a file manager. `ProtectHome`,
`PrivateTmp` or a restrictive `SystemCallFilter` would break exactly those, and the
failure would surface as "the media player does nothing" with no error anywhere.

### System-wide (`--system`)

Installed at `/etc/systemd/system/seanime.service`, running as a dedicated `seanime`
system user, and hardened to match the posture `deploy/k8s/deployment.yaml` and
`server.Dockerfile` already set: non-root, no ambient capabilities, a read-only system
with the data directory as the only writable path.

That lockdown is coherent **because** `--system` also seeds
`/var/lib/seanime/config.toml` with `capabilities = []`. Without that line, a
bare-metal server with no OIDC, no `externalurl` and no `trustedproxies` configured yet
is treated as a *local desktop install* by `ResolveDefaultCapabilities`
(`internal/security/capability.go`) and starts with `exec`, `filesystem`,
`extensions`, `selfupdate` and `nakama-host` all granted.

Two things this costs you, both intentional:

- **`selfupdate` cannot work.** `ProtectSystem=strict` makes `/usr` read-only. Update
  the binary the way you installed it.
- **A media root outside the data directory is invisible** until you add a
  `ReadWritePaths=` (or `ReadOnlyPaths=`) line for it to the unit. If you grant
  `filesystem` or `exec` in `config.toml`, relax the matching unit directive in the
  same change — otherwise the failure looks like a Seanime bug.

Before exposing the server beyond the host, read
[`WEB_DEPLOYMENT.md`](../../WEB_DEPLOYMENT.md): reverse proxy, TLS, OIDC and the
capability model. `--system` copies it and `config.example.toml` to
`/usr/local/share/doc/seanime/`.

## Uninstalling

```bash
./install-linux.sh --uninstall              # add --prefix/--system to match the install
```

Removes the binary, the launcher, both desktop entries (menu and desktop), every
installed icon, the `environment.d` PATH file, and the systemd unit (stopped and
disabled first).

Keeps, on purpose — the same choice the Windows installer makes:

- your **data directory** (`~/.config/Seanime` or `/var/lib/seanime`): library,
  database, settings, extensions;
- `launcher.env`, so a reinstall does not lose your configuration;
- the `seanime` system user, which owns files. Remove it yourself with
  `userdel seanime` once you have dealt with `/var/lib/seanime`.
