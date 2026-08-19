[CmdletBinding()]
param(
    [string]$OutputDirectory = (Join-Path $PSScriptRoot "dist\release"),
    [switch]$RequireCleanTree
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$projectRoot = $PSScriptRoot
$backendDir = Join-Path $projectRoot "backend"
$versionPath = Join-Path $projectRoot "VERSION"
$version = (Get-Content -LiteralPath $versionPath -Raw -Encoding UTF8).Trim()
if ($version -notmatch '^v(\d+)\.(\d+)\.(\d+)$') {
    throw "VERSION 格式无效：$version"
}
$productVersion = $version.TrimStart("v")
$standardArtifactName = "Kugo-Music-Converter-$version-windows-amd64.exe"
$webView2ArtifactName = "Kugo-Music-Converter-$version-windows-amd64-webview2.exe"
$standardInstallerName = "Kugo-Music-Converter-$version-windows-amd64-setup.exe"
$webView2InstallerName = "Kugo-Music-Converter-$version-windows-amd64-webview2-setup.exe"
$outputDirectoryPath = [IO.Path]::GetFullPath($OutputDirectory)
$standardArtifactPath = Join-Path $outputDirectoryPath $standardArtifactName
$webView2ArtifactPath = Join-Path $outputDirectoryPath $webView2ArtifactName
$standardInstallerPath = Join-Path $outputDirectoryPath $standardInstallerName
$webView2InstallerPath = Join-Path $outputDirectoryPath $webView2InstallerName
$releaseBodyPath = Join-Path $projectRoot "RELEASE-BODY-$version.md"
$ffmpegPath = Join-Path $projectRoot "tools\ffmpeg.exe"
$ffmpegPayloadDirectory = Join-Path $backendDir "internal\runtimebundle\payload"
$ffmpegPayloadHashPath = Join-Path $ffmpegPayloadDirectory "ffmpeg.sha256"
$webView2PayloadDirectory = Join-Path $backendDir "internal\webview2bundle\payload"
$webView2PayloadPath = Join-Path $webView2PayloadDirectory "webview2-runtime.cab"
$webView2VersionPath = Join-Path $webView2PayloadDirectory "webview2-runtime.version"
$webView2HashPath = Join-Path $webView2PayloadDirectory "webview2-runtime.sha256"

function Assert-LastExitCode([string]$Action) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Action 失败，退出码：$LASTEXITCODE"
    }
}

function Assert-Command([string]$Name) {
    if ($null -eq (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "缺少发布工具：$Name"
    }
}

function Invoke-ReleaseSelfTest([string]$ExecutablePath, [bool]$ExpectWebView2) {
    $reportPath = Join-Path ([IO.Path]::GetTempPath()) ("kugo-release-self-test-{0}.json" -f [Guid]::NewGuid().ToString("N"))
    try {
        $startInfo = [Diagnostics.ProcessStartInfo]::new()
        $startInfo.FileName = $ExecutablePath
        $startInfo.UseShellExecute = $false
        $startInfo.CreateNoWindow = $true
        $startInfo.ArgumentList.Add("--release-self-test")
        $startInfo.ArgumentList.Add("--output")
        $startInfo.ArgumentList.Add($reportPath)
        $process = [Diagnostics.Process]::Start($startInfo)
        if ($null -eq $process) {
            throw "无法启动正式构建自检：$ExecutablePath"
        }
        $process.WaitForExit()
        if ($process.ExitCode -ne 0) {
            $detail = if (Test-Path -LiteralPath $reportPath) { Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8 } else { "未生成自检报告" }
            throw "正式构建自检失败（退出码 $($process.ExitCode)）：$detail"
        }
        $report = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if (-not $report.success -or -not $report.ffmpegReady) {
            throw "正式构建自检未确认 FFmpeg 可用：$($report | ConvertTo-Json -Compress)"
        }
        if ([bool]$report.webView2Bundled -ne $ExpectWebView2 -or ($ExpectWebView2 -and -not $report.webView2Ready)) {
            throw "正式构建自检的 WebView2 状态不匹配：$($report | ConvertTo-Json -Compress)"
        }
    } finally {
        Remove-Item -LiteralPath $reportPath -Force -ErrorAction SilentlyContinue
    }
}

if ($null -eq (Get-Command "makensis" -ErrorAction SilentlyContinue)) {
    $nsisDirectory = Join-Path ${env:ProgramFiles(x86)} "NSIS"
    if (Test-Path -LiteralPath (Join-Path $nsisDirectory "makensis.exe") -PathType Leaf) {
        $env:Path = "$nsisDirectory;$env:Path"
    }
}

$requiredSources = @(
    (Join-Path $projectRoot "COPYING"),
    (Join-Path $projectRoot "README.md"),
    (Join-Path $projectRoot "THIRD-PARTY-NOTICES.md"),
    (Join-Path $projectRoot "LICENSES\FFmpeg.txt"),
    (Join-Path $projectRoot "LICENSES\WebView2.txt"),
    (Join-Path $projectRoot "LICENSES\Inter.txt"),
    (Join-Path $projectRoot "FFMPEG-SOURCE.md"),
    (Join-Path $projectRoot "WEBVIEW2-SOURCE.md"),
    (Join-Path $projectRoot "PRIVACY.md"),
    (Join-Path $projectRoot "SECURITY.md"),
    (Join-Path $projectRoot "SIGNING.md"),
    (Join-Path $projectRoot "SIGNING-POLICY.md"),
    $releaseBodyPath,
    $versionPath,
    (Join-Path $backendDir "wails.json"),
    (Join-Path $backendDir "build\appicon.png"),
    (Join-Path $backendDir "build\windows\icon.ico"),
    (Join-Path $backendDir "build\windows\info.json"),
    (Join-Path $backendDir "build\windows\wails.exe.manifest"),
    (Join-Path $backendDir "build\windows\installer\project.nsi")
)
foreach ($source in $requiredSources) {
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
        throw "发布所需文件缺失：$source"
    }
}

if ($RequireCleanTree) {
    $workingTreeStatus = (& git -C $projectRoot status --porcelain 2>$null | Out-String).Trim()
    Assert-LastExitCode "Git 状态检查"
    if (-not [string]::IsNullOrWhiteSpace($workingTreeStatus)) {
        throw "正式发布要求干净的 Git 工作区。"
    }
}

$releaseBody = Get-Content -LiteralPath $releaseBodyPath -Raw -Encoding UTF8
if (-not $releaseBody.StartsWith("## Kugo Music Converter $version")) {
    throw "Release 正文标题未同步到 $version"
}
$readme = Get-Content -LiteralPath (Join-Path $projectRoot "README.md") -Raw -Encoding UTF8
if (-not $readme.Contains($standardArtifactName) -or
    -not $readme.Contains($webView2ArtifactName) -or
    -not $readme.Contains($standardInstallerName) -or
    -not $readme.Contains($webView2InstallerName)) {
    throw "README 未同时列出标准版、内置 WebView2 版及对应安装器资产名。"
}

$wailsConfig = Get-Content -LiteralPath (Join-Path $backendDir "wails.json") -Raw -Encoding UTF8 | ConvertFrom-Json
if ($wailsConfig.info.productVersion -ne $productVersion) {
    throw "wails.json 产品版本不匹配：期望 $productVersion，实际 $($wailsConfig.info.productVersion)"
}
if ($wailsConfig.info.productName -ne "Kugo Music Converter") {
    throw "wails.json 缺少正确的产品名称。"
}

Assert-Command "go"
Assert-Command "node"
Assert-Command "wails"
Assert-Command "expand.exe"
Assert-Command "makensis"

& (Join-Path $projectRoot "prepare-ffmpeg.ps1")
$ffmpegOutput = & $ffmpegPath -version 2>&1
Assert-LastExitCode "FFmpeg 运行时检查"
$ffmpegVersion = $ffmpegOutput | Select-Object -First 1
if ([string]::IsNullOrWhiteSpace($ffmpegVersion) -or $ffmpegVersion -notmatch '^ffmpeg version ') {
    throw "FFmpeg 版本输出无效：$ffmpegVersion"
}
$ffmpegBuildID = ([string]$ffmpegVersion -split '\s+')[2]

& (Join-Path $backendDir "prepare-runtime.ps1") -FFmpegPath $ffmpegPath
Assert-LastExitCode "FFmpeg 内嵌载荷准备"
$ffmpegSourceHash = (Get-FileHash -LiteralPath $ffmpegPath -Algorithm SHA256).Hash.ToLowerInvariant()
$ffmpegPayloadHash = (Get-Content -LiteralPath $ffmpegPayloadHashPath -Raw -Encoding UTF8).Trim().ToLowerInvariant()
if ($ffmpegSourceHash -ne $ffmpegPayloadHash) {
    throw "内嵌 FFmpeg 哈希与源文件不一致。"
}
$ffmpegSourceDocument = Get-Content -LiteralPath (Join-Path $projectRoot "FFMPEG-SOURCE.md") -Raw -Encoding UTF8
if (-not $ffmpegSourceDocument.Contains($ffmpegSourceHash) -or -not $ffmpegSourceDocument.Contains($ffmpegBuildID)) {
    throw "FFMPEG-SOURCE.md 未同步当前 FFmpeg 版本或 SHA-256。"
}

if (Test-Path -LiteralPath $webView2PayloadPath -PathType Leaf) {
    & (Join-Path $backendDir "prepare-webview2-runtime.ps1") -SourceCAB $webView2PayloadPath
} else {
    & (Join-Path $backendDir "prepare-webview2-runtime.ps1")
}
$webView2Version = (Get-Content -LiteralPath $webView2VersionPath -Raw -Encoding UTF8).Trim()
$webView2Hash = (Get-Content -LiteralPath $webView2HashPath -Raw -Encoding UTF8).Trim().ToLowerInvariant()
$webView2ActualHash = (Get-FileHash -LiteralPath $webView2PayloadPath -Algorithm SHA256).Hash.ToLowerInvariant()
if ($webView2Hash -ne $webView2ActualHash) {
    throw "内嵌 WebView2 CAB 哈希与载荷不一致。"
}
$webView2SourceDocument = Get-Content -LiteralPath (Join-Path $projectRoot "WEBVIEW2-SOURCE.md") -Raw -Encoding UTF8
if (-not $webView2SourceDocument.Contains($webView2Hash) -or -not $webView2SourceDocument.Contains($webView2Version)) {
    throw "WEBVIEW2-SOURCE.md 未同步当前 WebView2 版本或 SHA-256。"
}

node --check (Join-Path $backendDir "frontend\src\main.js")
Assert-LastExitCode "桌面前端 JavaScript 语法检查"
$desktopUserText = @(
    (Get-Content -LiteralPath (Join-Path $backendDir "frontend\src\index.html") -Raw -Encoding UTF8),
    (Get-Content -LiteralPath (Join-Path $backendDir "frontend\src\main.js") -Raw -Encoding UTF8)
) -join "`n"
foreach ($legacyWording in @("上传", "localhost", "start.hta", "start.bat", "本地服务")) {
    if ($desktopUserText.Contains($legacyWording)) {
        throw "桌面界面仍包含旧链路表述：$legacyWording"
    }
}
node --test (Join-Path $projectRoot "tests\frontend\queue.test.mjs")
Assert-LastExitCode "桌面前端队列测试"

New-Item -ItemType Directory -Path $outputDirectoryPath -Force | Out-Null
foreach ($path in @(
    $standardArtifactPath,
    $webView2ArtifactPath,
    $standardInstallerPath,
    $webView2InstallerPath,
    "$standardArtifactPath.sha256",
    "$webView2ArtifactPath.sha256",
    "$standardInstallerPath.sha256",
    "$webView2InstallerPath.sha256",
    (Join-Path $outputDirectoryPath "build-metadata.json")
)) {
    if (Test-Path -LiteralPath $path -PathType Leaf) {
        Remove-Item -LiteralPath $path -Force
    }
}

Push-Location $backendDir
try {
    go test ./...
    Assert-LastExitCode "Go 测试"
    go test unlock-music.dev/cli/algo/qmc
    Assert-LastExitCode "QMC 上游解码器回归测试"
    go vet ./...
    Assert-LastExitCode "go vet"

    $buildDate = [DateTime]::UtcNow.ToString("o")
    $commitHash = (& git -C $projectRoot rev-parse --short=12 HEAD 2>$null | Select-Object -First 1)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($commitHash)) {
        throw "无法读取 Git 提交哈希。"
    }
    $ldflags = "-s -w -X main.version=$version -X main.buildDate=$buildDate -X main.commitHash=$commitHash"

    wails build -clean -trimpath -platform windows/amd64 -webview2 download -tags "runtimebundle,release" -nsis -o $standardArtifactName -ldflags $ldflags
    Assert-LastExitCode "Wails Windows amd64 标准版构建"
    $standardWailsOutput = Join-Path $backendDir "build\bin\$standardArtifactName"
    if (-not (Test-Path -LiteralPath $standardWailsOutput -PathType Leaf)) {
        throw "Wails 标准版构建产物缺失：$standardWailsOutput"
    }
    Copy-Item -LiteralPath $standardWailsOutput -Destination $standardArtifactPath -Force
    $standardWailsInstaller = Join-Path $backendDir "build\bin\Kugo-Music-Converter-amd64-setup.exe"
    if (-not (Test-Path -LiteralPath $standardWailsInstaller -PathType Leaf)) {
        throw "Wails 标准版安装器缺失：$standardWailsInstaller"
    }
    Copy-Item -LiteralPath $standardWailsInstaller -Destination $standardInstallerPath -Force

    wails build -clean -trimpath -platform windows/amd64 -webview2 error -tags "runtimebundle,webview2bundle,release" -nsis -o $webView2ArtifactName -ldflags $ldflags
    Assert-LastExitCode "Wails Windows amd64 内置 WebView2 版构建"
    $webView2WailsOutput = Join-Path $backendDir "build\bin\$webView2ArtifactName"
    if (-not (Test-Path -LiteralPath $webView2WailsOutput -PathType Leaf)) {
        throw "Wails 内置 WebView2 版构建产物缺失：$webView2WailsOutput"
    }
    Copy-Item -LiteralPath $webView2WailsOutput -Destination $webView2ArtifactPath -Force
    $webView2WailsInstaller = Join-Path $backendDir "build\bin\Kugo-Music-Converter-amd64-setup.exe"
    if (-not (Test-Path -LiteralPath $webView2WailsInstaller -PathType Leaf)) {
        throw "Wails 内置 WebView2 版安装器缺失：$webView2WailsInstaller"
    }
    Copy-Item -LiteralPath $webView2WailsInstaller -Destination $webView2InstallerPath -Force
} finally {
    Pop-Location
}

& (Join-Path $projectRoot "verify-release.ps1") -ExecutablePath $standardArtifactPath -ExpectedVersion $version
Assert-LastExitCode "标准版发布验证"
& (Join-Path $projectRoot "verify-release.ps1") -ExecutablePath $webView2ArtifactPath -ExpectedVersion $version
Assert-LastExitCode "内置 WebView2 版发布验证"
& (Join-Path $projectRoot "verify-release.ps1") -ExecutablePath $standardInstallerPath -ExpectedVersion $version
Assert-LastExitCode "标准版安装器发布验证"
& (Join-Path $projectRoot "verify-release.ps1") -ExecutablePath $webView2InstallerPath -ExpectedVersion $version
Assert-LastExitCode "内置 WebView2 版安装器发布验证"

Invoke-ReleaseSelfTest -ExecutablePath $standardArtifactPath -ExpectWebView2 $false
Invoke-ReleaseSelfTest -ExecutablePath $webView2ArtifactPath -ExpectWebView2 $true

$standardFile = Get-Item -LiteralPath $standardArtifactPath
$webView2File = Get-Item -LiteralPath $webView2ArtifactPath
$standardInstallerFile = Get-Item -LiteralPath $standardInstallerPath
$webView2InstallerFile = Get-Item -LiteralPath $webView2InstallerPath
if (($webView2File.Length - $standardFile.Length) -lt 250MB) {
    throw "内置 WebView2 版体积差异不足，可能没有嵌入 Fixed Runtime。"
}
foreach ($artifactPath in @($standardArtifactPath, $webView2ArtifactPath, $standardInstallerPath, $webView2InstallerPath)) {
    $signature = Get-AuthenticodeSignature -LiteralPath $artifactPath
    if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::NotSigned) {
        throw "v0.6.0 产物应明确保持未签名，实际状态：$($signature.Status)"
    }
    $artifactName = Split-Path -Leaf $artifactPath
    $artifactHash = (Get-FileHash -LiteralPath $artifactPath -Algorithm SHA256).Hash.ToLowerInvariant()
    "$artifactHash  $artifactName" | Set-Content -LiteralPath "$artifactPath.sha256" -Encoding ascii
}

$metadata = [ordered]@{
    version = $version
    productVersion = $productVersion
    architecture = "windows-amd64"
    commit = $commitHash
    buildDateUtc = $buildDate
    signatureStage = "unsigned-v0.6.0-release"
    ffmpeg = [ordered]@{
        version = [string]$ffmpegVersion
        sha256 = $ffmpegSourceHash
    }
    webView2FixedRuntime = [ordered]@{
        version = $webView2Version
        sha256 = $webView2Hash
    }
    artifacts = @(
        [ordered]@{
            name = $standardArtifactName
            variant = "standard-recommended"
            size = $standardFile.Length
            sha256 = (Get-FileHash -LiteralPath $standardArtifactPath -Algorithm SHA256).Hash.ToLowerInvariant()
        },
        [ordered]@{
            name = $webView2ArtifactName
            variant = "webview2-fixed-runtime"
            size = $webView2File.Length
            sha256 = (Get-FileHash -LiteralPath $webView2ArtifactPath -Algorithm SHA256).Hash.ToLowerInvariant()
        },
        [ordered]@{
            name = $standardInstallerName
            variant = "standard-recommended-installer-per-user"
            size = $standardInstallerFile.Length
            sha256 = (Get-FileHash -LiteralPath $standardInstallerPath -Algorithm SHA256).Hash.ToLowerInvariant()
        },
        [ordered]@{
            name = $webView2InstallerName
            variant = "webview2-fixed-runtime-installer-per-user"
            size = $webView2InstallerFile.Length
            sha256 = (Get-FileHash -LiteralPath $webView2InstallerPath -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    )
}
$metadataPath = Join-Path $outputDirectoryPath "build-metadata.json"
$metadata | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $metadataPath -Encoding utf8NoBOM

Write-Host "v0.6.0 发布资产已生成："
Write-Host "  标准版：$standardArtifactPath"
Write-Host "  内置 WebView2 版：$webView2ArtifactPath"
Write-Host "  标准版安装器：$standardInstallerPath"
Write-Host "  内置 WebView2 版安装器：$webView2InstallerPath"
Write-Host "两个便携 EXE 与两个按用户安装器均内嵌 FFmpeg，并按 v0.6.0 决策保持未签名；SHA-256 已生成。"
