; Inno Setup script for Runyx -- a real, branded installer (double-click,
; click through a wizard with Runyx's own mascot artwork, Runyx is
; installed and configured) that places Runyx.exe under Program Files
; (or the user's own profile -- see PrivilegesRequired below), adds a
; Start Menu shortcut, and offers three configuration choices during
; setup: a desktop shortcut, launching Runyx automatically at sign-in,
; and adding Runyx to PATH for command-line use (runyx doctor, runyx
; pull, ...). The wizard finishes by offering to launch Runyx.
;
; Built for real by .github/workflows/ci.yml's windows-installer job,
; which runs natively on a GitHub-hosted Windows runner (no Wine, no
; cross-compilation): installs Inno Setup via Chocolatey, builds
; Runyx.exe with the native WebView2 window, then compiles this script
; with ISCC.exe into Runyx-<version>-Installer.exe, a real, runnable
; Windows installer -- uploaded as a CI artifact on every push, which
; also silently installs it with every task enabled and confirms the
; PATH entry and startup shortcut were actually created, not just that
; the installer compiled.
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
; Branded wizard artwork -- the same mascot the taskbar/Explorer icon
; uses, rasterized onto Runyx's own dark navy (#1B2233), so the
; installer looks like it belongs to the app it's installing instead
; of Inno Setup's generic default banner. Regenerate with (from a
; checkout that has assets/mascot/, which this one -- the actual PR
; branch -- doesn't; see installer-assets/README.md):
;   rsvg-convert + ImageMagick, see docs/DECISIONS.md for the exact
;   commands used to produce these.
WizardImageFile=installer-assets\WizardImage.bmp
WizardSmallImageFile=installer-assets\WizardSmallImage.bmp
Compression=lzma
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Messages]
WelcomeLabel1=Welcome to the [name] Setup Wizard
WelcomeLabel2=This installs [name/ver] -- a fully local LLM runner.%nNo account, no telemetry, nothing leaves your machine.%n%nClick Next to choose how Runyx is set up on this computer.
FinishedLabel=Setup has finished installing [name] on this computer. Runyx runs entirely offline once you've downloaded a model -- no account needed.

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked
Name: "startupicon"; Description: "Start Runyx automatically when I sign in"; GroupDescription: "Startup:"; Flags: unchecked
Name: "addtopath"; Description: "Add Runyx to PATH (use ""runyx"" from any terminal)"; GroupDescription: "Command line:"; Flags: unchecked

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon
Name: "{userstartup}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: startupicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
// Standard Inno Setup idiom for a per-user PATH entry: Path lives in
// HKCU\Environment as a semicolon-joined REG_EXPAND_SZ, so adding/
// removing this app's install dir means read-modify-write the whole
// value, guarding against double-adding on reinstall/repair and
// leaving a stray entry on uninstall.
const
  EnvironmentKey = 'Environment';

procedure EnvAddPath(Dir: string);
var
  Paths: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    Paths := '';
  if Paths = '' then
    Paths := Dir
  else if Pos(';' + Uppercase(Dir) + ';', ';' + Uppercase(Paths) + ';') = 0 then
    Paths := Paths + ';' + Dir
  else
    exit; // already present
  if not RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    Log('EnvAddPath: failed to write HKCU\Environment\Path');
end;

procedure EnvRemovePath(Dir: string);
var
  Paths: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    exit;
  P := Pos(';' + Uppercase(Dir) + ';', ';' + Uppercase(Paths) + ';');
  if P = 0 then
    exit;
  Delete(Paths, P - 1, Length(Dir) + 1);
  RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssPostInstall) and WizardIsTaskSelected('addtopath') then
    EnvAddPath(ExpandConstant('{app}'));
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    EnvRemovePath(ExpandConstant('{app}'));
end;
