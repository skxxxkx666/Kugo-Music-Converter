# Windows code signing

## Release signing policy

Kugo Music Converter v0.6.0 and v0.6.1 are intentionally released without Authenticode signing. v0.6.1 does not apply to or integrate SignPath Foundation.

Each release workflow:

1. builds two Windows x64 portable executables and two per-user installers;
2. verifies PE product and version metadata;
3. requires all four PE assets to report `NotSigned` rather than an invalid or unexpected signature;
4. publishes a separate SHA-256 file for every portable executable and installer;
5. stages the assets in a draft GitHub Release for maintainer review.

Windows SmartScreen may display “Unknown publisher”. Users should download only from this repository's GitHub Releases page and compare the published SHA-256 value before running the file.

Local verification example:

```powershell
./verify-release.ps1 `
  -ExecutablePath ./Kugo-Music-Converter-v0.6.1-windows-amd64.exe `
  -ExpectedVersion v0.6.1
```

## v0.6.1 status

SignPath is not enabled for v0.6.1. The release continues the unsigned + per-asset SHA-256 model. This does not activate the proposed signing policy.

## Optional future signing

No SignPath application or integration is planned for v0.6.1. If a later release adopts SignPath Foundation through a separate project decision, configure protected GitHub settings only after the open-source project has been accepted:

| Type | Name | Purpose |
|---|---|---|
| Secret | `SIGNPATH_API_TOKEN` | authenticates the trusted GitHub connector |
| Variable | `SIGNPATH_ORGANIZATION_ID` | identifies the SignPath organization |
| Variable | `SIGNPATH_PROJECT_SLUG` | identifies the project |
| Variable | `SIGNPATH_SIGNING_POLICY_SLUG` | selects the approved release policy |
| Variable | `SIGNPATH_ARTIFACT_CONFIGURATION_SLUG` | selects which EXE files may be signed |
| Variable, optional | `SIGNPATH_EXPECTED_PUBLISHER` | expected Authenticode certificate subject |

Before enabling signing, update the release workflow and version-specific documentation, require timestamp verification, verify the expected publisher and confirm whether both Windows variants are covered.

Official references:

- <https://signpath.org/terms.html>
- <https://docs.signpath.io/trusted-build-systems/github>
- <https://github.com/SignPath/github-action-submit-signing-request>
