# Windows code signing

## v0.6.0 decision

Kugo Music Converter v0.6.0 is intentionally released without Authenticode signing. Applying to SignPath Foundation and enabling a signing gate are deferred for evaluation in v0.6.1.

The v0.6.0 release workflow therefore:

1. builds the standard and bundled-WebView2 Windows x64 executables;
2. verifies PE product and version metadata;
3. requires both executables to report `NotSigned` rather than an invalid or unexpected signature;
4. publishes a separate SHA-256 file for each executable;
5. stages the assets in a draft GitHub Release for maintainer review.

Windows SmartScreen may display “Unknown publisher” for v0.6.0. Users should download only from this repository's GitHub Releases page and compare the published SHA-256 value before running the file.

Local verification for v0.6.0:

```powershell
./verify-release.ps1 `
  -ExecutablePath ./Kugo-Music-Converter-v0.6.0-windows-amd64.exe `
  -ExpectedVersion v0.6.0
```

## Proposed v0.6.1 SignPath evaluation

No SignPath token, organization or certificate is required for v0.6.0. If the project proceeds with SignPath Foundation in v0.6.1, configure protected GitHub settings only after the open-source project has been accepted:

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
