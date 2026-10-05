# AppImage packaging

## Purpose

Builds the Linux AppImages attached to every release (`seanime-<v>_Linux_x86_64.AppImage`,
`seanime-<v>_Linux_arm64.AppImage`) and proves that no secret ships in them.

## Ownership

- `build.sh` — assembles the AppDir from an explicit file list, runs `appimagetool` with a
  pinned runtime, runs `verify.sh`, writes `<image>.sha256`. Deletes an image that fails
  verification.
- `verify.sh` — reads the squashfs payload (any target arch, no FUSE) and fails unless the file
  list is exactly the expected one and gitleaks finds nothing in `AppRun`, the desktop entry and
  the `strings` of the binary (which covers the embedded web bundle).
- `scan-source.sh` — gitleaks over the checked-out tree, plus `BASE..HEAD` history when given a
  base ref.
- `tools.sh` — pinned versions and SHA-256s of appimagetool, the type2 runtime and gitleaks;
  downloads into `dist/appimage-tools/` and verifies every use.
- `AppRun`, `seanime.desktop` — AppImage entrypoint and desktop entry.
- Repo-root `.gitleaks.toml` — the scan config shared by both scans.
- CI: jobs `secret-scan` and `build-appimage` in `.github/workflows/release-draft-new.yml`.

## Local Contracts

- Package the **systray** binary (no `-tags=nosystray`). AppRun `exec`s it in the foreground:
  the AppImage is unmounted when AppRun's process exits, so the server must never be
  backgrounded the way `seanime-launch` does it.
- Nothing configurable ships inside the image. Config, tokens and the data dir are created at
  runtime in `~/.config/Seanime`. Adding a file to the AppDir means adding it to `EXPECTED` in
  `verify.sh` too.
- `.gitleaks.toml` exceptions are an exact value or line shape only. Never use `paths`: gitleaks
  skips a matching file entirely, ignoring `condition = "AND"`.
- Build jobs reference no secrets and run with `contents: read`. The only secrets are the
  optional `APPIMAGE_GPG_PRIVATE_KEY` / `APPIMAGE_GPG_PASSPHRASE`, used in the `release` job
  after verification to write detached `.asc` signatures. Unset means unsigned (sha256 only).
- Self-update does not apply to an AppImage (its executable is read-only); users replace the file.

## Work Guidance

- To bump a pinned tool, change version and hashes together in `tools.sh`
  (`gh api repos/<owner>/<repo>/releases/tags/<tag> --jq '.assets[].digest'`).
- A new gitleaks false positive: check it really is public, then add the narrowest exception
  with a comment saying why.

## Verification

Needs `squashfs-tools`, `binutils`, `file`, `python3`, `curl` (CI installs them on Ubuntu).

```bash
npm run build && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/seanime-systray .
packaging/appimage/build.sh --binary dist/seanime-systray --arch x86_64   # builds + verifies
packaging/appimage/scan-source.sh upstream/main
```

## Child Index

None.
