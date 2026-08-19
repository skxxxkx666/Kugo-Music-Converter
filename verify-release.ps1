[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedVersion,
    [switch]$RequireSignature,
    [string]$ExpectedPublisher = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$resolvedPath = [IO.Path]::GetFullPath($ExecutablePath)
if (-not (Test-Path -LiteralPath $resolvedPath -PathType Leaf)) {
    throw "发布文件不存在：$resolvedPath"
}
if ($ExpectedVersion -notmatch '^v\d+\.\d+\.\d+$') {
    throw "期望版本格式无效：$ExpectedVersion"
}

$productVersion = $ExpectedVersion.TrimStart("v")
$file = Get-Item -LiteralPath $resolvedPath
if ($file.Length -lt 1MB) {
    throw "发布文件尺寸异常：$($file.Length) bytes"
}

$versionInfo = $file.VersionInfo
if ($versionInfo.ProductName -ne "Kugo Music Converter") {
    throw "PE ProductName 无效：$($versionInfo.ProductName)"
}
if (-not $versionInfo.ProductVersion.StartsWith($productVersion, [StringComparison]::Ordinal)) {
    throw "PE ProductVersion 无效：期望 $productVersion，实际 $($versionInfo.ProductVersion)"
}
if (-not $versionInfo.FileVersion.StartsWith($productVersion, [StringComparison]::Ordinal)) {
    throw "PE FileVersion 无效：期望 $productVersion，实际 $($versionInfo.FileVersion)"
}
if ([string]::IsNullOrWhiteSpace($versionInfo.CompanyName)) {
    throw "PE CompanyName 不能为空。"
}

$signature = Get-AuthenticodeSignature -LiteralPath $resolvedPath
if ($RequireSignature) {
    if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid) {
        throw "Authenticode 签名无效：$($signature.Status) $($signature.StatusMessage)"
    }
    if ($null -eq $signature.SignerCertificate) {
        throw "签名有效但缺少签名证书信息。"
    }
    if ($null -eq $signature.TimeStamperCertificate) {
        throw "正式发布签名缺少可信时间戳。"
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedPublisher) -and
        $signature.SignerCertificate.Subject -notlike "*$ExpectedPublisher*") {
        throw "签名发布者不匹配：$($signature.SignerCertificate.Subject)"
    }
} elseif ($signature.Status -notin @(
    [System.Management.Automation.SignatureStatus]::NotSigned,
    [System.Management.Automation.SignatureStatus]::Valid
)) {
    throw "候选文件包含无效签名：$($signature.Status) $($signature.StatusMessage)"
}

$hash = (Get-FileHash -LiteralPath $resolvedPath -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Host "发布文件验证通过：$resolvedPath"
Write-Host "ProductVersion：$($versionInfo.ProductVersion)"
Write-Host "签名状态：$($signature.Status)"
Write-Host "SHA256：$hash"
