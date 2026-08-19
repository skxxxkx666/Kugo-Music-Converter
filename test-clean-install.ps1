[CmdletBinding()]
param(
    [string]$ReleaseDirectory = (Join-Path $PSScriptRoot "dist\release"),
    [string]$ExpectedVersion = ((Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw -Encoding UTF8).Trim()),
    [switch]$ConfirmCleanMachine
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if (-not $ConfirmCleanMachine) {
    throw "此脚本会安装并卸载候选版本，只允许在干净测试机或一次性 CI 中运行；请显式传入 -ConfirmCleanMachine。"
}
if ($ExpectedVersion -notmatch '^v\d+\.\d+\.\d+$') {
    throw "期望版本格式无效：$ExpectedVersion"
}

$releaseRoot = [IO.Path]::GetFullPath($ReleaseDirectory)
$installDirectory = Join-Path $env:LOCALAPPDATA "Programs\Kugo Music Converter"
$installedExecutable = Join-Path $installDirectory "Kugo Music Converter.exe"
$uninstaller = Join-Path $installDirectory "uninstall.exe"
$uninstallRegistryPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\KugoMusicConverter"
$version = $ExpectedVersion
$installers = @(
    [ordered]@{
        Name = "Kugo-Music-Converter-$version-windows-amd64-setup.exe"
        WebView2 = $false
    },
    [ordered]@{
        Name = "Kugo-Music-Converter-$version-windows-amd64-webview2-setup.exe"
        WebView2 = $true
    }
)

function Start-HiddenAndWait([string]$FilePath, [string[]]$Arguments) {
    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $FilePath
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    foreach ($argument in $Arguments) {
        $startInfo.ArgumentList.Add($argument)
    }
    $process = [Diagnostics.Process]::Start($startInfo)
    if ($null -eq $process) {
        throw "无法启动进程：$FilePath"
    }
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) {
        throw "进程退出码异常（$($process.ExitCode)）：$FilePath"
    }
}

function Assert-AssetHash([string]$Path) {
    $hashPath = "$Path.sha256"
    if (-not (Test-Path -LiteralPath $hashPath -PathType Leaf)) {
        throw "缺少 SHA-256 文件：$hashPath"
    }
    $expected = ((Get-Content -LiteralPath $hashPath -Raw -Encoding ascii).Trim() -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expected -ne $actual) {
        throw "安装器 SHA-256 不匹配：$Path"
    }
}

function Wait-CleanInstallState {
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    while ((Test-Path -LiteralPath $installDirectory) -or (Test-Path $uninstallRegistryPath)) {
        if ([DateTime]::UtcNow -ge $deadline) {
            return
        }
        Start-Sleep -Milliseconds 250
    }
}

function Invoke-InstalledSelfTest([bool]$ExpectWebView2) {
    $reportPath = Join-Path ([IO.Path]::GetTempPath()) ("kugo-installed-self-test-{0}.json" -f [Guid]::NewGuid().ToString("N"))
    try {
        Start-HiddenAndWait -FilePath $installedExecutable -Arguments @("--release-self-test", "--output", $reportPath)
        $report = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if (-not $report.success -or -not $report.ffmpegReady) {
            throw "已安装程序的 FFmpeg 自检失败：$($report | ConvertTo-Json -Compress)"
        }
        if ([bool]$report.webView2Bundled -ne $ExpectWebView2 -or ($ExpectWebView2 -and -not $report.webView2Ready)) {
            throw "已安装程序的 WebView2 自检不匹配：$($report | ConvertTo-Json -Compress)"
        }
    } finally {
        Remove-Item -LiteralPath $reportPath -Force -ErrorAction SilentlyContinue
    }
}

if ((Test-Path -LiteralPath $installDirectory) -or (Test-Path $uninstallRegistryPath)) {
    throw "测试机不是干净状态，已存在 Kugo Music Converter 安装目录或卸载项。"
}

foreach ($installer in $installers) {
    $installerPath = Join-Path $releaseRoot $installer.Name
    if (-not (Test-Path -LiteralPath $installerPath -PathType Leaf)) {
        throw "候选安装器不存在：$installerPath"
    }
    Assert-AssetHash -Path $installerPath

    try {
        Start-HiddenAndWait -FilePath $installerPath -Arguments @("/S")
        if (-not (Test-Path -LiteralPath $installedExecutable -PathType Leaf) -or
            -not (Test-Path -LiteralPath $uninstaller -PathType Leaf) -or
            -not (Test-Path $uninstallRegistryPath)) {
            throw "安装器未生成预期的程序、卸载器或当前用户卸载项：$($installer.Name)"
        }
        & (Join-Path $PSScriptRoot "verify-release.ps1") -ExecutablePath $installedExecutable -ExpectedVersion $ExpectedVersion
        Invoke-InstalledSelfTest -ExpectWebView2 ([bool]$installer.WebView2)
    } finally {
        if (Test-Path -LiteralPath $uninstaller -PathType Leaf) {
            Start-HiddenAndWait -FilePath $uninstaller -Arguments @("/S")
        }
    }

    Wait-CleanInstallState
    if ((Test-Path -LiteralPath $installDirectory) -or (Test-Path $uninstallRegistryPath)) {
        throw "卸载后仍残留程序目录或当前用户卸载项：$($installer.Name)"
    }
    Write-Host "干净系统安装验收通过：$($installer.Name)"
}
