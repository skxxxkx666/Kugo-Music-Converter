[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SampleDirectory,
    [string]$DatabasePath = "",
    [string]$KGGV2Sample = "",
    [string]$FFmpegPath = "",
    [string]$OutputDirectory = "",
    [string]$ReportPath = "",
    [switch]$RequireCompleteCoverage,
    [switch]$InventoryOnly
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($FFmpegPath)) {
    $FFmpegPath = Join-Path $PSScriptRoot "tools\ffmpeg.exe"
}

$sampleRoot = [IO.Path]::GetFullPath($SampleDirectory)
if (-not (Test-Path -LiteralPath $sampleRoot -PathType Container)) {
    throw "样本目录不存在：$sampleRoot"
}

$coverageGroups = [ordered]@{
    KGG = @(".kgg")
    KGM = @(".kgm")
    KGMA = @(".kgma")
    VPR = @(".vpr")
    NCM = @(".ncm")
    KWM = @(".kwm")
    QMC_MFLAC = @(".mflac")
    QMC_MGG = @(".mgg")
    QMC = @(".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg", ".tkm")
}
$supportedExtensions = @($coverageGroups.Values | ForEach-Object { $_ } | ForEach-Object { $_.ToLowerInvariant() })
$sampleFiles = @(Get-ChildItem -LiteralPath $sampleRoot -File -Recurse -ErrorAction Stop | Where-Object {
    $supportedExtensions -contains $_.Extension.ToLowerInvariant()
})
if ($sampleFiles.Count -eq 0) {
    throw "样本目录中没有支持的加密音频：$sampleRoot"
}

$counts = [ordered]@{}
$missing = [Collections.Generic.List[string]]::new()
foreach ($entry in $coverageGroups.GetEnumerator()) {
    $extensions = @($entry.Value | ForEach-Object { $_.ToLowerInvariant() })
    $count = @($sampleFiles | Where-Object { $extensions -contains $_.Extension.ToLowerInvariant() }).Count
    $counts[$entry.Key] = $count
    if ($count -eq 0) {
        $missing.Add($entry.Key)
    }
}

$hasKGGV2 = -not [string]::IsNullOrWhiteSpace($KGGV2Sample) -and (Test-Path -LiteralPath $KGGV2Sample -PathType Leaf)
if (-not $hasKGGV2) {
    $missing.Add("KGG_V2_EKEY")
}

$report = [ordered]@{
    generatedAtUtc = [DateTime]::UtcNow.ToString("o")
    sampleDirectory = $sampleRoot
    totalFiles = $sampleFiles.Count
    counts = $counts
    kggV2SampleProvided = $hasKGGV2
    missingCoverage = @($missing)
    conversionExecuted = $false
    conversionPassed = $false
}

if (-not $InventoryOnly) {
    $resolvedFFmpeg = [IO.Path]::GetFullPath($FFmpegPath)
    if (-not (Test-Path -LiteralPath $resolvedFFmpeg -PathType Leaf)) {
        & (Join-Path $PSScriptRoot "prepare-ffmpeg.ps1")
    }
    if (-not (Test-Path -LiteralPath $resolvedFFmpeg -PathType Leaf)) {
        throw "FFmpeg 不存在：$resolvedFFmpeg"
    }

    if ($counts.KGG -gt 0 -and [string]::IsNullOrWhiteSpace($DatabasePath)) {
        $databaseCandidates = @(
            (Join-Path $env:APPDATA "KuGou\KGMusicV3.db"),
            (Join-Path $env:APPDATA "KuGou8\KGMusicV3.db"),
            (Join-Path $env:LOCALAPPDATA "KuGou\KGMusicV3.db"),
            (Join-Path $env:LOCALAPPDATA "KuGou8\KGMusicV3.db")
        )
        $DatabasePath = $databaseCandidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
    }
    if ($counts.KGG -gt 0 -and ([string]::IsNullOrWhiteSpace($DatabasePath) -or -not (Test-Path -LiteralPath $DatabasePath -PathType Leaf))) {
        throw "样本包含 KGG，但未找到 KGMusicV3.db；请通过 -DatabasePath 提供。"
    }

    $environmentNames = @(
        "KUGO_TEST_INPUT_DIR",
        "KUGO_TEST_FFMPEG",
        "KUGO_TEST_DB",
        "KUGO_TEST_OUTPUT_FORMAT",
        "KUGO_TEST_CONCURRENCY",
        "KUGO_TEST_INPUT",
        "KUGO_TEST_OUTPUT_DIR"
    )
    $previousEnvironment = @{}
    foreach ($name in $environmentNames) {
        $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
    }
    try {
        $env:KUGO_TEST_INPUT_DIR = $sampleRoot
        $env:KUGO_TEST_FFMPEG = $resolvedFFmpeg
        $env:KUGO_TEST_DB = $DatabasePath
        $env:KUGO_TEST_OUTPUT_FORMAT = "copy"
        $env:KUGO_TEST_CONCURRENCY = "1"
        if (-not [string]::IsNullOrWhiteSpace($OutputDirectory)) {
            $resolvedOutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
            New-Item -ItemType Directory -Path $resolvedOutputDirectory -Force | Out-Null
            $env:KUGO_TEST_OUTPUT_DIR = $resolvedOutputDirectory
        } else {
            $env:KUGO_TEST_OUTPUT_DIR = $null
        }

        Push-Location (Join-Path $PSScriptRoot "backend")
        try {
            go test ./internal/handler -run '^TestConvertLocalPathsRealFolder$' -count=1 -v
            if ($LASTEXITCODE -ne 0) {
                throw "真实样本目录转换回归失败。"
            }
            if ($hasKGGV2) {
                $env:KUGO_TEST_INPUT = [IO.Path]::GetFullPath($KGGV2Sample)
                go test ./internal/handler -run '^TestConvertLocalPathsRealFile$' -count=1 -v
                if ($LASTEXITCODE -ne 0) {
                    throw "KGG V2 ekey 样本转换回归失败。"
                }
            }
        } finally {
            Pop-Location
        }
        $report.conversionExecuted = $true
        $report.conversionPassed = $true
    } finally {
        foreach ($name in $environmentNames) {
            [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], "Process")
        }
    }
}

if (-not [string]::IsNullOrWhiteSpace($ReportPath)) {
    $resolvedReportPath = [IO.Path]::GetFullPath($ReportPath)
    $reportJson = $report | ConvertTo-Json -Depth 5
    [IO.File]::WriteAllText($resolvedReportPath, $reportJson, [Text.UTF8Encoding]::new($false))
}

Write-Host "真实样本覆盖：$($report.counts | ConvertTo-Json -Compress)"
if ($missing.Count -gt 0) {
    Write-Warning ("仍缺发布覆盖：" + ($missing -join ", "))
}
if ($RequireCompleteCoverage -and $missing.Count -gt 0) {
    throw "真实样本发布门禁未通过：仍缺 $($missing -join ', ')。"
}
