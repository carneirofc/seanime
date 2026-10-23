# Packaging

Everything that turns a built Seanime binary into something installed on a machine. Every
format starts from `npm run build` (see [`../DEVELOPMENT_AND_BUILD.md`](../DEVELOPMENT_AND_BUILD.md));
running Seanime as a networked server is covered in [`../deploy/README.md`](../deploy/README.md).

| Directory | Produces | Entry point | Docs |
| --- | --- | --- | --- |
| [`linux/`](linux/) | Per-user or system-wide install: launcher, XDG entry, icons, systemd unit | `npm run install:linux`, or `packaging/linux/install.sh --help` | [`linux/README.md`](linux/README.md) |
| [`windows/`](windows/) | Per-user install script, and an Inno Setup `setup.exe` | `npm run install:windows`, `npm run build:installer` | [`windows/README.md`](windows/README.md) |
| [`arch/`](arch/) | `seanime-git` and `seanime-cloak-backend-git` pacman packages | `scripts/package.sh` | [`arch/AGENTS.md`](arch/AGENTS.md) |
| [`appimage/`](appimage/) | The release AppImages, verified and secret-scanned | `packaging/appimage/build.sh` (CI) | [`appimage/AGENTS.md`](appimage/AGENTS.md) |

The `*.in` templates in `linux/` are shared: `linux/install.sh` and the Arch `PKGBUILD` both
render them, so a change there reaches both.

All scripts here locate the checkout relative to themselves and run from any directory.
