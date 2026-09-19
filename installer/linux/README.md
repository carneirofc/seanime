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
- Optional: `imagemagick` for crisp icons at every size (see [Icons](#icons)),
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
| `--add-to-path` | | see [PATH](#path) |
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

The menu entry does not run `seanime` directly. It runs an installed
`seanime-launch` script, because the server is headless: it serves the web interface
and prints to stdout, and opens no window. A bare `Exec=seanime` would look, to the
person who clicked it, like nothing happened.

`seanime-launch` with no arguments:

1. checks whether something is already answering — a second click opens the tab
   rather than starting a second server;
2. if not, starts the server. With `--systemd` that is `systemctl --user start`;
   otherwise the server is spawned under `setsid` in its own session, so it survives
   the launcher, the browser and the terminal that started it;
3. polls `GET /api/v1/status` — the same endpoint `server.Dockerfile` uses as its
   healthcheck — until it answers, for up to `SEANIME_LAUNCH_TIMEOUT` seconds
   (default 90; a first run builds the database and caches before it listens);
4. opens the URL with `xdg-open`.

Other modes: `--terminal`, `--remote`, `--stop`, `--status`, `--url`, `--help`.

**Closing the browser does not stop the server, and that is correct — it is a
server.** Three ways to stop it: the "Stop the server" action on the menu entry,
`seanime-launch --stop`, or `systemctl --user stop seanime` under `--systemd`. If you
want the server's lifetime managed for you, use `--systemd`.

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

Note what is **not** in that file: `SEANIME_SERVER_HOST` and `SEANIME_SERVER_PORT`.
The server rewrites `config.toml` whenever either differs from the stored value
(`internal/core/config.go`), so a launcher or a systemd unit that helpfully repeated
them would overwrite a hand-edited config on every single start. The launcher instead
*reads* `[server] host` and `port` out of `<datadir>/config.toml` and passes only
`--datadir`.

That is why `--host`/`--port` are seeded into `config.toml` at install time rather
than exported: to change the address afterwards, edit `config.toml` — the launcher
will follow it.

### The terminal action

Right-click the menu entry in the launcher and you get **Start in a terminal**, which
runs the server in the foreground so you can watch the logs, plus **Stop the server**
and **Open data directory**.

It does not work by setting `Terminal=true`. That key is not valid inside a
`[Desktop Action]` group — the Desktop Entry specification allows only `Name`, `Icon`
and `Exec` there, and `desktop-file-validate` rejects it outright:

```
error: file contains key "Terminal" in group "Desktop Action terminal",
but keys extending the format should start with "X-"
```

So the action calls `seanime-launch --terminal`, which finds a terminal emulator
itself. This is also more reliable than `Terminal=true` would have been: the argument
that introduces a command differs per emulator (`konsole -e`, `wezterm start --`,
`gnome-terminal --`, `kitty` with no flag at all), and the emulator your desktop is
configured to use is not necessarily one that accepts a bare `-e`. Set
`SEANIME_TERMINAL` if the detection picks wrong.

## Icons

The source is `internal/icon/logo.png`, a 439×439 raster. The installer downscales it
to 16, 22, 24, 32, 48, 64, 128 and 256 px under `icons/hicolor/<n>x<n>/apps/` — 22 and
24 are the Plasma panel sizes, 256 is what KRunner and the overview use. Nothing is
upscaled, and `scalable/` is never used, because that directory means SVG.

Without ImageMagick (`magick` or `convert`) the unmodified PNG goes to
`share/pixmaps/seanime.png` instead, which both KDE and GTK still search by icon name.
It works; the sizes are just not tuned.

Cache refresh (`update-desktop-database`, `gtk-update-icon-cache`, `kbuildsycoca6`) is
best-effort — every one of them is skipped if absent and ignored if it fails.
`gtk-update-icon-cache` is additionally skipped when the theme directory has no
`index.theme`, which is the normal state of a per-user `hicolor` and would otherwise
make it hard-fail.

## Desktop icon

`--desktop-icon` copies the entry to `xdg-user-dir DESKTOP` (falling back to
`~/Desktop`) and marks it executable. Plasma refuses to run a non-executable
`.desktop` file placed on the desktop, so the exec bit is not optional. It is skipped
with a warning if you have no desktop directory.

## PATH

`--add-to-path` checks first and usually does nothing: `~/.local/bin` is already on
`PATH` on stock Arch/CachyOS, Fedora and Debian.

When it genuinely is not, the installer writes
`~/.config/environment.d/10-seanime.conf` rather than editing a shell rc file. One
file, shell-agnostic, and `--uninstall` removes it with one `rm`. Its two limits are
printed at install time and worth repeating: it applies **at your next login**, not to
the current shell, and it does not reach TTY logins or `ssh`. For those, add the
directory to your shell's own path the usual way.

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
