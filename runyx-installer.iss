; Runyx - script d'installation (Inno Setup)
; Installation par utilisateur : aucun droit administrateur, aucun UAC.

#define Version "0.1.0"
#define Editeur "FuraxDev"

[Setup]
AppId={{8F3C1A62-9B4D-4E51-9C0A-3D7B2A15E6F1}
AppName=Runyx
AppVersion={#Version}
AppVerName=Runyx {#Version}
AppPublisher={#Editeur}
AppPublisherURL=https://tryrunyx.vercel.app
AppSupportURL=https://github.com/furaxdev/Runyx/issues
AppUpdatesURL=https://github.com/furaxdev/Runyx/releases
DefaultDirName={localappdata}\Runyx
DefaultGroupName=Runyx
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=dist
OutputBaseFilename=RunyxSetup-{#Version}
SetupIconFile=runyx.ico
UninstallDisplayIcon={app}\runyx.exe
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
LicenseFile=LICENSE
VersionInfoVersion={#Version}
VersionInfoProductName=Runyx
VersionInfoCompany={#Editeur}
VersionInfoDescription=Runyx {#Version} - local LLM runner

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; le binaire et son moteur : rien n'est telecharge a l'installation
Source: "runyx.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "backend\*"; DestDir: "{app}\backend"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Runyx"; Filename: "{app}\runyx.exe"
Name: "{group}\Desinstaller Runyx"; Filename: "{uninstallexe}"
Name: "{autodesktop}\Runyx"; Filename: "{app}\runyx.exe"; Tasks: desktopicon

[Run]
; on ouvre l'interface une fois installe
Filename: "{app}\runyx.exe"; Description: "Lancer Runyx"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; on ne supprime QUE les fichiers de l'application.
; Les modeles vivent dans {localappdata}\Runyx\models et survivent a la
; desinstallation : les retélécharger coute des gigaoctets.
Type: filesandordirs; Name: "{app}\backend"
