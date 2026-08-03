[CmdletBinding()]
param(
    [string]$OutputDirectory = $PSScriptRoot
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$projectRoot = $PSScriptRoot
$backendDir = Join-Path $projectRoot "backend"
$versionPath = Join-Path $projectRoot "VERSION"
$version = (Get-Content -LiteralPath $versionPath -Raw -Encoding UTF8).Trim()
if ($version -notmatch '^v\d+\.\d+\.\d+$') {
    throw "VERSION 格式无效：$version"
}

$outputDirectoryPath = [IO.Path]::GetFullPath($OutputDirectory)
$archiveName = "Kugo-Music-Converter-$version-windows-amd64.zip"
$archivePath = Join-Path $outputDirectoryPath $archiveName
$releaseBodyPath = Join-Path $projectRoot "RELEASE-BODY-$version.md"
$stageRoot = Join-Path ([IO.Path]::GetTempPath()) ("kugo-release-" + [Guid]::NewGuid().ToString("N"))
$packageRoot = Join-Path $stageRoot "Kugo-Music-Converter"
$exePath = Join-Path $backendDir "bin\kugo-converter.exe"

$previousEnvironment = @{
    CGO_ENABLED = $env:CGO_ENABLED
    GOOS = $env:GOOS
    GOARCH = $env:GOARCH
}
$backendLocationPushed = $false

function Assert-LastExitCode([string]$Action) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Action 失败，退出码：$LASTEXITCODE"
    }
}

try {
    $requiredSources = @(
        (Join-Path $projectRoot "COPYING"),
        (Join-Path $projectRoot "README.md"),
        $releaseBodyPath,
        $versionPath,
        (Join-Path $projectRoot "start.bat"),
        (Join-Path $projectRoot "start.hta"),
        (Join-Path $projectRoot "public"),
        (Join-Path $projectRoot "tools\ffmpeg.exe")
    )
    foreach ($source in $requiredSources) {
        if (-not (Test-Path -LiteralPath $source)) {
            throw "发布所需文件缺失：$source"
        }
    }

    $readme = Get-Content -LiteralPath (Join-Path $projectRoot "README.md") -Raw -Encoding UTF8
    if (-not $readme.Contains("**最新版本：$version**") -or -not $readme.Contains($archiveName)) {
        throw "README 中的最新版本或发布包名称未同步到 $version"
    }
    $releaseBody = Get-Content -LiteralPath $releaseBodyPath -Raw -Encoding UTF8
    if (-not $releaseBody.StartsWith("## Kugo Music Converter $version")) {
        throw "Release 正文标题未同步到 $version"
    }

    $ffmpegOutput = & (Join-Path $projectRoot "tools\ffmpeg.exe") -version 2>&1
    $ffmpegExitCode = $LASTEXITCODE
    if ($ffmpegExitCode -ne 0) {
        throw "FFmpeg 运行时检查失败，退出码：$ffmpegExitCode"
    }
    $ffmpegVersion = $ffmpegOutput | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($ffmpegVersion) -or $ffmpegVersion -notmatch '^ffmpeg version ') {
        throw "FFmpeg 版本输出无效：$ffmpegVersion"
    }

    node --test (Join-Path $projectRoot "tests\frontend\queue.test.mjs")
    Assert-LastExitCode "前端回归测试"
    Get-ChildItem -LiteralPath (Join-Path $projectRoot "public") -Recurse -Filter "*.js" | ForEach-Object {
        node --check $_.FullName
        Assert-LastExitCode "JavaScript 语法检查：$($_.FullName)"
    }

    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    Push-Location $backendDir
    $backendLocationPushed = $true

    go test ./...
    Assert-LastExitCode "Go 测试"
    go vet ./...
    Assert-LastExitCode "go vet"

    New-Item -ItemType Directory -Path (Split-Path -Parent $exePath) -Force | Out-Null
    $buildDate = [DateTime]::UtcNow.ToString("o")
    $commitHash = (& git -C $projectRoot rev-parse --short HEAD 2>$null | Select-Object -First 1)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($commitHash)) {
        $commitHash = "unknown"
    } else {
        $workingTreeStatus = (& git -C $projectRoot status --porcelain 2>$null | Out-String).Trim()
        if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($workingTreeStatus)) {
            $commitHash = "$commitHash-dirty"
        }
    }
    $ldflags = "-s -w -X main.version=$version -X main.buildDate=$buildDate -X main.commitHash=$commitHash -X main.appEnv=release"
    go build -trimpath -ldflags $ldflags -o $exePath ./cmd/server
    Assert-LastExitCode "Windows amd64 后端构建"

    $versionOutput = (& $exePath --version 2>&1 | Out-String).Trim()
    Assert-LastExitCode "后端版本检查"
    if ($versionOutput -notmatch [Regex]::Escape($version)) {
        throw "后端版本不匹配：期望 $version，实际输出 $versionOutput"
    }

    Pop-Location
    $backendLocationPushed = $false

    New-Item -ItemType Directory -Path $packageRoot -Force | Out-Null
    foreach ($name in @("COPYING", "README.md", "VERSION", "start.bat", "start.hta")) {
        Copy-Item -LiteralPath (Join-Path $projectRoot $name) -Destination $packageRoot
    }

    $packageBackendBin = Join-Path $packageRoot "backend\bin"
    New-Item -ItemType Directory -Path $packageBackendBin -Force | Out-Null
    Copy-Item -LiteralPath $exePath -Destination $packageBackendBin

    $packagePublic = Join-Path $packageRoot "public"
    New-Item -ItemType Directory -Path $packagePublic -Force | Out-Null
    Copy-Item -Path (Join-Path $projectRoot "public\*") -Destination $packagePublic -Recurse

    $packageTools = Join-Path $packageRoot "tools"
    New-Item -ItemType Directory -Path $packageTools -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $projectRoot "tools\ffmpeg.exe") -Destination $packageTools

    $packageOutput = Join-Path $packageRoot "output"
    New-Item -ItemType Directory -Path $packageOutput -Force | Out-Null
    New-Item -ItemType File -Path (Join-Path $packageOutput ".gitkeep") -Force | Out-Null

    New-Item -ItemType Directory -Path $outputDirectoryPath -Force | Out-Null
    if (Test-Path -LiteralPath $archivePath) {
        Remove-Item -LiteralPath $archivePath -Force
    }
    Compress-Archive -LiteralPath $packageRoot -DestinationPath $archivePath -CompressionLevel Optimal

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::OpenRead($archivePath)
    try {
        $entries = @($zip.Entries | ForEach-Object { $_.FullName.Replace('\', '/') })
        $requiredEntries = @(
            "Kugo-Music-Converter/VERSION",
            "Kugo-Music-Converter/backend/bin/kugo-converter.exe",
            "Kugo-Music-Converter/public/index.html",
            "Kugo-Music-Converter/tools/ffmpeg.exe"
        )
        foreach ($entry in $requiredEntries) {
            if ($entries -notcontains $entry) {
                throw "发布包缺少条目：$entry"
            }
        }

        $versionEntry = $zip.Entries | Where-Object { $_.FullName.Replace('\', '/') -eq "Kugo-Music-Converter/VERSION" } | Select-Object -First 1
        $reader = [IO.StreamReader]::new($versionEntry.Open(), [Text.Encoding]::UTF8, $true)
        try {
            $packagedVersion = $reader.ReadToEnd().Trim()
        } finally {
            $reader.Dispose()
        }
        if ($packagedVersion -ne $version) {
            throw "发布包 VERSION 不匹配：期望 $version，实际 $packagedVersion"
        }
    } finally {
        $zip.Dispose()
    }

    $hash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash
    Write-Host "发布包已生成：$archivePath"
    Write-Host "后端版本：$versionOutput"
    Write-Host "SHA256：$hash"
} finally {
    if ($backendLocationPushed) {
        Pop-Location
    }
    foreach ($name in $previousEnvironment.Keys) {
        $value = $previousEnvironment[$name]
        if ($null -eq $value) {
            Remove-Item -LiteralPath "Env:$name" -ErrorAction SilentlyContinue
        } else {
            Set-Item -LiteralPath "Env:$name" -Value $value
        }
    }
    if (Test-Path -LiteralPath $stageRoot) {
        Remove-Item -LiteralPath $stageRoot -Recurse -Force
    }
}
