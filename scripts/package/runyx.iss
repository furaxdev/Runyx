; Inno Setup script for Runyx -- a real installer (double-click, click
; through a wizard, Runyx is installed and ready) that places Runyx.exe
; under Program Files (or the user's own profile -- see
; PrivilegesRequired below), adds a Start Menu shortcut, an optional
; desktop shortcut, and offers to launch Runyx when the wizard finishes.
;
; Built for real by .github/workflows/ci.yml's windows-installer job,
; which runs natively on a GitHub-hosted Windows runner (no Wine, no
; cross-compilation): installs Inno Setup via Chocolatey, builds
; Runyx.exe with the native WebView2 window, then compiles this script
; with ISCC.exe into Runyx-<version>-Installer.exe, a real, runnable
; Windows installer -- uploaded as a CI artifact on every push.
;
; To build locally (on Windows, with Inno Setup installed and
; Runyx.exe already built next to this file, or elsewhere -- pass
; /DSourceExe=path\to\Runyx.exe to override):
;   iscc /DMyAppVersion=0.1.0 scripts\package\runyx.iss
;
; See scripts/package/README.md for this packaging target's verified/
; not-verified status.

#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif
#ifndef SourceExe
  #define SourceExe "Runyx.exe"
#endif

#define MyAppName "Runyx"
#define MyAppPublisher "Runyx"
#define MyAppURL "https://github.com/furaxdev/Runyx"
#define MyAppExeName "Runyx.exe"

[Setup]
AppId={{B8F2B6B0-7B6D-4B7E-9A3A-6B6F3E5B4A1C}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
; Portable-by-default app (config/models live in the user's data dir,
; not under Program Files), so no admin rights are strictly required;
; "lowest" lets a non-admin user install to their own profile instead.
PrivilegesRequired=lowest
OutputBaseFilename=Runyx-{#MyAppVersion}-Installer
SetupIconFile=..\..\cmd\runyx\runyx.ico
Compression=lzma
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
