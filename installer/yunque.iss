; ╔══════════════════════════════════════════════╗
; ║  云雀 Agent (Yunque Agent) — Inno Setup     ║
; ╚══════════════════════════════════════════════╝
;
; Build with: iscc installer\yunque.iss
; Requires: Inno Setup 6+ (https://jrsoftware.org/isinfo.php)
;
; Before building, ensure:
;   1. dist\yunque-agent.exe exists (run: go build -ldflags "-s -w -H windowsgui" -o dist\yunque-agent.exe .\cmd\agent)
;   2. heroui-web\out\ exists (frontend is embedded, so this is only for reference)
;   3. .env.example exists in project root

#define MyAppName "Yunque Agent"
#define MyAppNameZh "云雀 Agent"
#define MyAppPublisher "Tori Project"
#define MyAppURL "https://github.com/user/yunque-agent"
#define MyAppExeName "yunque-agent.exe"

; Version is injected by build script, fallback to dev
#ifndef MyAppVersion
  #define MyAppVersion "0.1.0-dev"
#endif

[Setup]
AppId={{A7E3D2F1-8B4C-4D6E-9F0A-1B2C3D4E5F6A}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppNameZh}
AllowNoIcons=yes
OutputBaseFilename=YunqueAgent-Setup-{#MyAppVersion}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesInstallIn64BitMode=x64compatible
; Uncomment when icon is available:
; SetupIconFile=assets\yunque.ico
; UninstallDisplayIcon={app}\yunque-agent.exe

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "autostart"; Description: "开机自动启动 / Start on login"; GroupDescription: "系统集成 / System Integration"

[Files]
Source: "..\dist\yunque-agent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\.env.example"; DestDir: "{app}"; DestName: ".env.example"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppNameZh}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\卸载 {#MyAppNameZh}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppNameZh}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Registry]
; Auto-start on login (current user only)
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; \
  ValueType: string; ValueName: "YunqueAgent"; \
  ValueData: """{app}\{#MyAppExeName}"" --background"; \
  Flags: uninsdeletevalue; Tasks: autostart

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "启动 {#MyAppNameZh} / Launch {#MyAppName}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Clean up log files but NOT user data (in %AppData%\YunqueAgent)
Type: files; Name: "{app}\*.log"

[Code]
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    { Create initial .env from example if not exists }
    if not FileExists(ExpandConstant('{app}\.env')) then
    begin
      if FileExists(ExpandConstant('{app}\.env.example')) then
        FileCopy(ExpandConstant('{app}\.env.example'), ExpandConstant('{app}\.env'), False);
    end;
  end;
end;

function InitializeUninstall(): Boolean;
var
  DataDir: String;
  MsgResult: Integer;
begin
  Result := True;
  DataDir := ExpandConstant('{userappdata}\YunqueAgent');
  if DirExists(DataDir) then
  begin
    MsgResult := MsgBox(
      '是否同时删除用户数据？' + #13#10 +
      'Delete user data as well?' + #13#10 + #13#10 +
      '数据位置 / Data location: ' + DataDir,
      mbConfirmation, MB_YESNO);
    if MsgResult = IDYES then
      DelTree(DataDir, True, True, True);
  end;
end;
