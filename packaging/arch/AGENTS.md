# Arch Linux packaging

## Purpose

Native Arch install of Seanime and its stealth gateway through one split `PKGBUILD`.

## Ownership

- `PKGBUILD` — builds two packages from the fork's git repositories:
  - `seanime-git`: `/usr/bin/seanime` (web UI embedded), the `seanime-launch` launcher, desktop
    entry, hicolor icons, `seanime.service` (systemd user unit).
  - `seanime-cloak-backend-git`: a relocatable uv venv in `/opt/seanime-cloak-backend`
    (from `seanime-extensions/cloak-backend/uv.lock`), `/usr/bin/cloak-backend`,
    `seanime-cloak-backend.service`.
- `seanime-cloak-backend.service`, `cloak-backend.sh`, `seanime.install` — local sources of the
  above. Their checksums are pinned in `PKGBUILD`.

## Local Contracts

- The launcher, desktop entry and user unit are rendered from `../../installer/linux/*.in` by
  `_render`. They are not copied. A new `@PLACEHOLDER@` there needs a substitution in `_render`.
- Package paths: the data dir is `~/.config/Seanime` and launcher settings are in
  `~/.config/seanime/launcher.env`, matching `install-linux.sh` user mode. Gateway settings are
  in `~/.config/seanime/cloak-backend.env` (`CLOAK_*`).
- `pkgver` comes from `internal/constants/constants.go`, not from tags. The fork's releases are
  not tagged on `main`.
- The Camoufox browser is not packaged. The gateway unit's `ExecStartPre` downloads it into
  `~/.cache/camoufox` on first start, only when it is missing.
- The venv is tied to the system Python minor version. Bump `pkgrel` after a Python upgrade.

## Work Guidance

- After editing a local source file, run `updpkgsums`, then regenerate `.SRCINFO` with
  `makepkg --printsrcinfo > .SRCINFO` before publishing to the AUR.
- Keep `depends` of the gateway in step with what Camoufox's Firefox links against.

## Verification

```bash
cd packaging/arch
makepkg -f            # builds both packages, runs check()
namcap PKGBUILD *.pkg.tar.zst
```

## Child Index

None.
