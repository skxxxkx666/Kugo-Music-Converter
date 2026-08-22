# Proposed release signing policy

Last updated: 2026-08-18

This is a proposed policy for a future release. It is not active for v0.6.0 or v0.6.1, which are intentionally unsigned and distributed with published SHA-256 checksums.

## Proposed scope

If SignPath Foundation is adopted, only Windows executables produced by the repository's trusted release workflow may be submitted. Pull-request artifacts, local development builds and files rebuilt after submission must never be presented as signed releases.

The policy must explicitly cover every release variant intended for publication, including the standard executable and any executable that embeds the WebView2 Fixed Version Runtime. The embedded FFmpeg and WebView2 packages retain their upstream provenance and are recorded in `FFMPEG-SOURCE.md` and `WEBVIEW2-SOURCE.md`.

## Proposed trusted build and approval chain

1. A maintainer synchronizes `VERSION`, release notes, PE metadata and third-party notices.
2. Automated tests and reproducible payload checks run on a GitHub-hosted Windows runner.
3. The exact unsigned candidates are uploaded as a GitHub Actions artifact.
4. The official SignPath GitHub Action submits that artifact to the configured policy.
5. Release signing uses the approvals and multi-factor authentication required by SignPath.
6. Signed artifacts are downloaded without rebuilding.
7. `verify-release.ps1 -RequireSignature` verifies ProductName, ProductVersion, FileVersion, Authenticode validity, trusted timestamp and expected publisher.
8. Only verified executables and their SHA-256 files are attached to a draft GitHub Release.
9. A maintainer reviews and publishes the draft manually.

Fork and pull-request workflows must not be able to request signing. Credentials belong only in protected GitHub secrets or SignPath and must never be committed or printed in logs.

## Incident response

If a signing credential, workflow or signed artifact is suspected to be compromised:

- stop publishing affected releases;
- disable or rotate affected credentials;
- contact the signing provider about revocation when appropriate;
- preserve workflow and signing-request records;
- publish a security notice and a clean replacement after containment.

The current unsigned release status and optional future setup notes are in [`SIGNING.md`](SIGNING.md).
