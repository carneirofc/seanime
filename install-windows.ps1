<#
.SYNOPSIS
    Installs a built Seanime server binary on Windows.

.DESCRIPTION
    Copies the Seanime server binary into an install directory (default:
    %LOCALAPPDATA%\Programs\Seanime), creates the data directory, and (optionally)
    creates a Start Menu / Desktop shortcut and adds the install directory to the
    user PATH.

    The binary is expected to already exist. Run `npm run build` first (or pass
    -BuildFirst) to produce it. By default it looks for the artifact produced by
    `build-api.ps1` / `npm run build:api` at dist\seanime-windows-amd64.exe, then
    falls back to the systray binary at the repository root.

.EXAMPLE
    npm run build:install

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1 -AddToPath -DesktopShortcut

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1 -BuildFirst -InstallDir "D:\Apps\Seanime"
#>
[CmdletBinding()]
param(
    # Path to the built server binary. Relative paths are resolved against the repo root.
    [string]$BinaryPath,
    # Where to install Seanime. Defaults to a per-user, non-admin location.
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "Programs\Seanime"),
    # Data directory passed to the binary via --datadir. Defaults to <InstallDir>\seanime_data_dir.
    [string]$DataDir,
    # Build the web interface and server first (runs `npm run build`).
    [switch]$BuildFirst,
    # Add the install directory to the current user's PATH.
    [switch]$AddToPath,
    # Also create a shortcut on the Desktop.
    [switch]$DesktopShortcut,
    # Do not create a Start Menu shortcut.
    [switch]$NoShortcut
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$rootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$installedExeName = "seanime.exe"

# Candidate locations for the built binary, in order of preference.
$binaryCandidates = @(
    (Join-Path $rootDir "dist\seanime-windows-amd64.exe"),
    (Join-Path $rootDir "seanime-server-systray-windows.exe"),
    (Join-Path $rootDir "seanime.exe")
)

function Write-Log {
    param(
        [string]$Message,
        [ValidateSet("INFO", "WARN", "ERROR")]
        [string]$Level = "INFO"
    )

    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $line = "[$ts] [$Level] $Message"
    switch ($Level) {
        "WARN" { Write-Host $line -ForegroundColor Yellow }
        "ERROR" { Write-Host $line -ForegroundColor Red }
        default { Write-Host $line -ForegroundColor Cyan }
    }
}

function Resolve-BinaryPath {
    if ($BinaryPath) {
        $candidate = if ([System.IO.Path]::IsPathRooted($BinaryPath)) { $BinaryPath } else { Join-Path $rootDir $BinaryPath }
        if (-not (Test-Path $candidate)) {
            throw "Binary not found at -BinaryPath: $candidate"
        }
        return (Resolve-Path $candidate).Path
    }

    foreach ($candidate in $binaryCandidates) {
        if (Test-Path $candidate) {
            return (Resolve-Path $candidate).Path
        }
    }

    throw "No Seanime binary found. Looked in:`n  $($binaryCandidates -join "`n  ")`nBuild it first with 'npm run build' or pass -BuildFirst."
}

function New-Shortcut {
    param(
        [string]$ShortcutPath,
        [string]$TargetPath,
        [string]$Arguments,
        [string]$WorkingDirectory
    )

    $shell = New-Object -ComObject WScript.Shell
    try {
        $shortcut = $shell.CreateShortcut($ShortcutPath)
        $shortcut.TargetPath = $TargetPath
        $shortcut.Arguments = $Arguments
        $shortcut.WorkingDirectory = $WorkingDirectory
        $shortcut.IconLocation = $TargetPath
        $shortcut.Description = "Seanime media server"
        $shortcut.Save()
    }
    finally {
        [System.Runtime.InteropServices.Marshal]::ReleaseComObject($shell) | Out-Null
    }
}

if (-not $DataDir) {
    $DataDir = Join-Path $InstallDir "seanime_data_dir"
}

if ($BuildFirst) {
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
        throw "npm not found on PATH; cannot -BuildFirst."
    }
    Write-Log "Building Seanime (npm run build)..."
    Push-Location $rootDir
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) {
            throw "npm run build failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}

$sourceBinary = Resolve-BinaryPath
Write-Log "Source binary: $sourceBinary"

# Create install + data directories.
foreach ($dir in @($InstallDir, $DataDir)) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Write-Log "Created directory: $dir"
    }
}

# Copy the binary into place.
$installedExe = Join-Path $InstallDir $installedExeName
Copy-Item -Path $sourceBinary -Destination $installedExe -Force
$sizeMb = [Math]::Round(((Get-Item $installedExe).Length / 1MB), 2)
Write-Log "Installed binary: $installedExe ($sizeMb MB)"

$launchArgs = "--datadir `"$DataDir`""

# Start Menu shortcut.
if (-not $NoShortcut) {
    $startMenuDir = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
    $startMenuShortcut = Join-Path $startMenuDir "Seanime.lnk"
    New-Shortcut -ShortcutPath $startMenuShortcut -TargetPath $installedExe -Arguments $launchArgs -WorkingDirectory $InstallDir
    Write-Log "Created Start Menu shortcut: $startMenuShortcut"
}

# Desktop shortcut.
if ($DesktopShortcut) {
    $desktopShortcut = Join-Path ([Environment]::GetFolderPath("Desktop")) "Seanime.lnk"
    New-Shortcut -ShortcutPath $desktopShortcut -TargetPath $installedExe -Arguments $launchArgs -WorkingDirectory $InstallDir
    Write-Log "Created Desktop shortcut: $desktopShortcut"
}

# Add install dir to user PATH (idempotent).
if ($AddToPath) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = @()
    if ($userPath) {
        $pathEntries = $userPath.Split(";") | Where-Object { $_ -ne "" }
    }
    if ($pathEntries -notcontains $InstallDir) {
        $newPath = (@($pathEntries) + $InstallDir) -join ";"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Log "Added to user PATH: $InstallDir (restart your shell to pick it up)"
    }
    else {
        Write-Log "Install directory already on user PATH: $InstallDir"
    }
}

Write-Log "Seanime installed."
Write-Log "  Binary    : $installedExe"
Write-Log "  Data dir  : $DataDir"
Write-Log "Launch it from the Start Menu or run: `"$installedExe`" $launchArgs"
