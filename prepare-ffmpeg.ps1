[CmdletBinding()]
param(
    [string]$SourceURL = "https://github.com/skxxxkx666/Kugo-Music-Converter/releases/download/v0.5.1/Kugo-Music-Converter-v0.5.1-windows-amd64.zip",
    [string]$ExpectedSHA256 = "128cdaa01cfd6a72d961ccb6777adb2c32278091a203f2b8ac83f7b5a181dd7f"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$targetDirectory = Join-Path $PSScriptRoot "tools"
$targetPath = Join-Path $targetDirectory "ffmpeg.exe"
if (Test-Path -LiteralPath $targetPath -PathType Leaf) {
    $existingHash = (Get-FileHash -LiteralPath $targetPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($existingHash -eq $ExpectedSHA256.ToLowerInvariant()) {
        Write-Host "已复用校验通过的 FFmpeg：$targetPath"
        return
    }
}

$temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) ("kugo-ffmpeg-" + [Guid]::NewGuid().ToString("N"))
$archivePath = Join-Path $temporaryDirectory "release.zip"
$extractPath = Join-Path $temporaryDirectory "extracted"
try {
    New-Item -ItemType Directory -Path $temporaryDirectory -Force | Out-Null
    Invoke-WebRequest -Uri $SourceURL -OutFile $archivePath -UseBasicParsing
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractPath -Force

    $candidates = @(Get-ChildItem -LiteralPath $extractPath -Recurse -File -Filter "ffmpeg.exe")
    if ($candidates.Count -ne 1) {
        throw "FFmpeg 获取包中应只有一个 ffmpeg.exe，实际找到 $($candidates.Count) 个。"
    }
    $actualHash = (Get-FileHash -LiteralPath $candidates[0].FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $ExpectedSHA256.ToLowerInvariant()) {
        throw "FFmpeg SHA-256 不匹配：期望 $ExpectedSHA256，实际 $actualHash"
    }

    New-Item -ItemType Directory -Path $targetDirectory -Force | Out-Null
    Copy-Item -LiteralPath $candidates[0].FullName -Destination $targetPath -Force
    Write-Host "FFmpeg 已准备：$targetPath"
    Write-Host "SHA256：$actualHash"
} finally {
    if (Test-Path -LiteralPath $temporaryDirectory -PathType Container) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
