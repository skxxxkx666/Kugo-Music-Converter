# Security policy

## Supported versions

Security fixes are provided for the latest published release. Older versions may be referenced for migration but are not guaranteed to receive fixes.

## Reporting a vulnerability

Use GitHub Private Vulnerability Reporting when it is available for this repository. If it is not available, open a minimal GitHub issue asking the maintainers for a private reporting channel.

Do not attach music files, `KGMusicV3.db`, encryption keys, credentials, personal paths or exploit material to a public issue. Include only the minimum non-sensitive information needed to identify the affected version and component.

Maintainers should acknowledge a valid report, investigate it before public disclosure, prepare a replacement release when necessary and document the remediation after users have a reasonable opportunity to update.

## Release integrity

v0.6.0 Windows executables are intentionally unsigned. Each formal asset must be built by [`.github/workflows/release-windows-v0.6.0.yml`](.github/workflows/release-windows-v0.6.0.yml), pass PE metadata and payload checks, and include a published SHA-256 file. Users should download only from this repository's GitHub Releases page and verify the checksum.

Authenticode signing through SignPath Foundation is deferred for evaluation in v0.6.1. The proposed future policy is documented in [`SIGNING-POLICY.md`](SIGNING-POLICY.md) and does not apply retroactively to v0.6.0.
