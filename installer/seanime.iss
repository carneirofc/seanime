; Inno Setup script for Seanime (Windows).
;
; Produces a per-user setup.exe with an uninstaller, Start Menu/Desktop shortcuts,
; optional sign-in autostart, and Add/Remove Programs registration.
;
; Prerequisites:
;   - Inno Setup 6 (ISCC.exe). Install with: winget install JRSoftware.InnoSetup
;   - A built server binary at dist\seanime-windows-amd64.exe (run `npm run build`).
;
; Build:
;   npm run build:installer            (compiles this script)
;   npm run dist:windows               (build the app, then the installer)
;   iscc /DAppVersion=3.8.3 installer\seanime.iss
;
; The output is written to dist\seanime-setup-<version>.exe.

#ifndef AppVersion
  #define AppVersion "3.8.3"
#endif

#define AppName "Seanime"
#define AppPublisher "Seanime"
#define AppURL "https://github.com/5rahim/seanime"
#define AppExeName "seanime.exe"
; Path is relative to this .iss file (installer\).
#define SourceBinary "..\dist\seanime-windows-amd64.exe"

[Setup]
; A stable AppId keeps upgrades/uninstall consistent across versions. Do not change it.
AppId={{A7E5C9D2-3B4F-4A1E-9C6D-2F8B1E7A4C30}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}
AppUpdatesURL={#AppURL}/releases
VersionInfoVersion={#AppVersion}
DefaultDirName={localappdata}\Programs\Seanime
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
; Per-user install: no admin rights required, and the data directory stays writable.
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=seanime-setup-{#AppVersion}
SetupIconFile=..\internal\icon\iconwin.ico
UninstallDisplayIcon={app}\{#AppExeName}
UninstallDisplayName={#AppName} {#AppVersion}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "autostart"; Description: "Start {#AppName} automatically when I sign in"; GroupDescription: "Startup:"; Flags: unchecked

[Dirs]
; Data directory (config.toml, database, logs); shortcuts pass it via --datadir.
Name: "{app}\data"

[Files]
Source: "{#SourceBinary}"; DestDir: "{app}"; DestName: "{#AppExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Parameters: "--datadir ""{app}\data"""; WorkingDir: "{app}"; IconFilename: "{app}\{#AppExeName}"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Parameters: "--datadir ""{app}\data"""; WorkingDir: "{app}"; IconFilename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Registry]
; Optional sign-in autostart (removed on uninstall).
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{#AppName}"; ValueData: """{app}\{#AppExeName}"" --datadir ""{app}\data"""; Flags: uninsdeletevalue; Tasks: autostart

[Run]
Filename: "{app}\{#AppExeName}"; Parameters: "--datadir ""{app}\data"""; WorkingDir: "{app}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent
