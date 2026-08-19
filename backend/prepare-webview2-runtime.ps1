[CmdletBinding()]
param(
    [string]$Version = "151.0.4129.93",
    [string]$Architecture = "x64",
    [string]$DownloadURL = "https://msedge.sf.dl.delivery.mp.microsoft.com/filestreamingservice/files/1424552f-1033-46d3-a1ea-26c879f4262b/Microsoft.WebView2.FixedVersionRuntime.151.0.4129.93.x64.cab",
    [string]$ExpectedSHA256 = "1cb7106545f5aee92ee16496347a0e775a351cb5a3816d072f04323695899bde",
    [string]$SourceCAB = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ($Version -notmatch '^\d+\.\d+\.\d+\.\d+$') {
    throw "WebView2 Fixed Runtime 版本格式无效：$Version"
}
if ($Architecture -ne "x64") {
    throw "v0.6.0 当前仅构建 x64，WebView2 架构必须为 x64。"
}
if ($ExpectedSHA256 -notmatch '^[0-9a-fA-F]{64}$') {
    throw "WebView2 Fixed Runtime SHA-256 格式无效。"
}

$payloadDirectory = Join-Path $PSScriptRoot "internal\webview2bundle\payload"
$payloadPath = Join-Path $payloadDirectory "webview2-runtime.cab"
$versionPath = Join-Path $payloadDirectory "webview2-runtime.version"
$hashPath = Join-Path $payloadDirectory "webview2-runtime.sha256"
$temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) ("kugo-webview2-" + [Guid]::NewGuid().ToString("N"))
$downloadPath = Join-Path $temporaryDirectory "webview2-runtime.cab"

try {
    New-Item -ItemType Directory -Path $temporaryDirectory -Force | Out-Null
    if ([string]::IsNullOrWhiteSpace($SourceCAB)) {
        Write-Host "正在从 Microsoft 下载 WebView2 Fixed Runtime $Version ($Architecture)…"
        Invoke-WebRequest -Uri $DownloadURL -OutFile $downloadPath -UseBasicParsing
        $sourcePath = $downloadPath
    } else {
        $sourcePath = [IO.Path]::GetFullPath($SourceCAB)
        if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
            throw "WebView2 CAB 不存在：$sourcePath"
        }
    }

    $actualHash = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $ExpectedSHA256.ToLowerInvariant()) {
        throw "WebView2 CAB SHA-256 不匹配：期望 $ExpectedSHA256，实际 $actualHash"
    }

    $listing = (& expand.exe -D $sourcePath 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0 -or $listing -notmatch 'msedgewebview2\.exe') {
        throw "WebView2 CAB 内容检查失败，未找到 msedgewebview2.exe。"
    }

    New-Item -ItemType Directory -Path $payloadDirectory -Force | Out-Null
    if ([IO.Path]::GetFullPath($sourcePath) -ne [IO.Path]::GetFullPath($payloadPath)) {
        Copy-Item -LiteralPath $sourcePath -Destination $payloadPath -Force
    }
    [IO.File]::WriteAllText($versionPath, "$Version`n", [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText($hashPath, "$actualHash`n", [Text.UTF8Encoding]::new($false))

    $payloadSize = (Get-Item -LiteralPath $payloadPath).Length
    Write-Host "WebView2 内嵌载荷已准备：$payloadPath"
    Write-Host "版本：$Version ($Architecture)"
    Write-Host "大小：$payloadSize bytes"
    Write-Host "SHA256：$actualHash"
} finally {
    if (Test-Path -LiteralPath $temporaryDirectory -PathType Container) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
