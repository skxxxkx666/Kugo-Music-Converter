[CmdletBinding()]
param(
    [string]$FFmpegPath = (Join-Path $PSScriptRoot "..\tools\ffmpeg.exe")
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$sourcePath = [IO.Path]::GetFullPath($FFmpegPath)
if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
    throw "FFmpeg 文件不存在：$sourcePath"
}

$versionOutput = & $sourcePath -version 2>&1
if ($LASTEXITCODE -ne 0 -or -not (($versionOutput | Select-Object -First 1) -match '^ffmpeg version ')) {
    throw "指定文件不是可用的 FFmpeg：$sourcePath"
}

$payloadDirectory = Join-Path $PSScriptRoot "internal\runtimebundle\payload"
$payloadPath = Join-Path $payloadDirectory "ffmpeg.exe.gz"
$hashPath = Join-Path $payloadDirectory "ffmpeg.sha256"
$tempPath = "$payloadPath.tmp"

try {
    New-Item -ItemType Directory -Path $payloadDirectory -Force | Out-Null

    $input = [IO.File]::OpenRead($sourcePath)
    try {
        $output = [IO.File]::Create($tempPath)
        try {
            $gzip = [IO.Compression.GZipStream]::new($output, [IO.Compression.CompressionLevel]::Optimal, $true)
            try {
                $input.CopyTo($gzip)
            } finally {
                $gzip.Dispose()
            }
        } finally {
            $output.Dispose()
        }
    } finally {
        $input.Dispose()
    }

    Move-Item -LiteralPath $tempPath -Destination $payloadPath -Force
    $hash = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText($hashPath, "$hash`n", [Text.UTF8Encoding]::new($false))

    $payloadSize = (Get-Item -LiteralPath $payloadPath).Length
    Write-Host "FFmpeg 运行时载荷已生成：$payloadPath"
    Write-Host "压缩大小：$payloadSize bytes"
    Write-Host "SHA256：$hash"
} finally {
    if (Test-Path -LiteralPath $tempPath) {
        Remove-Item -LiteralPath $tempPath -Force
    }
}
