Unicode true

!define REQUEST_EXECUTION_LEVEL "user"
!define PRODUCT_EXECUTABLE "Kugo Music Converter.exe"
!define UNINST_KEY_NAME "KugoMusicConverter"

!include "wails_tools.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion "${INFO_PRODUCTVERSION}.0"
VIAddVersionKey "CompanyName" "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright" "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName" "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\Kugo-Music-Converter-${ARCH}-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\Kugo Music Converter"
InstallDirRegKey HKCU "${UNINST_KEY}" "InstallLocation"
ShowInstDetails show
ShowUninstDetails show

Function .onInit
    !insertmacro wails.checkArchitecture
FunctionEnd

Section "安装"
    SetShellVarContext current

    !insertmacro wails.webview2runtime

    SetOutPath "$INSTDIR"
    !insertmacro wails.files

    CreateDirectory "$SMPROGRAMS\Kugo Music Converter"
    CreateShortcut "$SMPROGRAMS\Kugo Music Converter\Kugo Music Converter.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortcut "$DESKTOP\Kugo Music Converter.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    WriteUninstaller "$INSTDIR\uninstall.exe"

    SetRegView 64
    WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${INFO_COMPANYNAME}"
    WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
    WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
    WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1

    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

Section "uninstall"
    SetShellVarContext current

    Delete "$SMPROGRAMS\Kugo Music Converter\Kugo Music Converter.lnk"
    RMDir "$SMPROGRAMS\Kugo Music Converter"
    Delete "$DESKTOP\Kugo Music Converter.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
    SetOutPath "$TEMP"
    Delete "$INSTDIR\uninstall.exe"

    RMDir /r "$APPDATA\${PRODUCT_EXECUTABLE}"
    RMDir /r "$LOCALAPPDATA\Kugo Music Converter"

    SetRegView 64
    DeleteRegKey HKCU "${UNINST_KEY}"

    RMDir /r "$INSTDIR"
SectionEnd
