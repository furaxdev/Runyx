; Inno Setup script for Runyx -- a simple installer that places
; Runyx.exe in Program Files, and adds a Start Menu shortcut plus an
; optional desktop shortcut. Not built or run here: this sandbox has
; no Windows and no Inno Setup compiler (iscc.exe) available. Build it
; on Windows (or via Wine) with:
;   iscc scripts\package\runyx.iss
; after first building Runyx.exe with scripts/package/windows-release.sh
; (or any other means) and placing it next to this .iss file, or by
; adjusting SourceExe below to point at it.
;
; See scripts/package/README.md for this packaging target's verified/
; not-verified status.

#define MyAppName "Runyx"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "Runyx"
#define MyAppURL "https://github.com/furaxdev/Runyx"
#define MyAppExeName "Runyx.exe"
#define SourceExe "Runyx.exe"

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
OutputBaseFilename=runyx-setup
SetupIconFile=..\..\cmd\runyx\runyx.ico
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
