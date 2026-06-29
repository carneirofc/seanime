# Windows installer

[`seanime.iss`](./seanime.iss) is an [Inno Setup](https://jrsoftware.org/isinfo.php) 6 script that
packages the built Seanime server into a double-click `setup.exe` with an uninstaller, Start Menu /
Desktop shortcuts, optional sign-in autostart, and Add/Remove Programs registration.

The Seanime server binary already embeds the web UI, so the installer ships a single self-contained
executable.

## Prerequisites

- **Inno Setup 6** — provides the `ISCC.exe` compiler. Install with:
  ```
  winget install JRSoftware.InnoSetup
  ```
  Make sure `ISCC.exe` is on `PATH` (or call it by full path).
- A **built server binary** at `dist\seanime-windows-amd64.exe` (run `npm run build`).

## Building the installer

From the repository root:

```bash
# Build the app, then package the installer
npm run dist:windows

# Or, if the binary is already built
npm run build:installer
```

Or invoke the compiler directly (lets you pass the real version):

```bash
iscc /DAppVersion=3.8.3 installer\seanime.iss
```

The result is written to `dist\seanime-setup-<version>.exe`.

## Install behavior

- **Per-user install** (no administrator rights). Default location:
  `%LOCALAPPDATA%\Programs\Seanime`.
- Creates a writable data directory at `<install>\seanime_data_dir`; shortcuts launch the server with
  `--datadir` pointing at it.
- Optional tasks: a Desktop icon and "start automatically when I sign in" (a per-user `Run` registry
  entry, removed on uninstall).
- User data in `seanime_data_dir` is intentionally **left in place** on uninstall.

## Silent install / uninstall

```bash
seanime-setup-3.8.3.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART
"%LOCALAPPDATA%\Programs\Seanime\unins000.exe" /VERYSILENT
```
