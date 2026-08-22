# Security policy

## Supported versions

Security fixes are provided for the latest published release. Older versions may be referenced for migration but are not guaranteed to receive fixes.

## Reporting a vulnerability

Use GitHub Private Vulnerability Reporting when it is available for this repository. If it is not available, open a minimal GitHub issue asking the maintainers for a private reporting channel.

Do not attach music files, `KGMusicV3.db`, encryption keys, QQ Music session values, account identifiers, credentials, personal paths or exploit material to a public issue. Include only the minimum non-sensitive information needed to identify the affected version and component.

Maintainers should acknowledge a valid report, investigate it before public disclosure, prepare a replacement release when necessary and document the remediation after users have a reasonable opportunity to update.

## QQ Music session handling

Modern `.mflac` and `.mgg` conversion reads explicit `authst` fields from the current user's QQ Music cookie files and, when necessary, scans readable memory in same-user `QQMusic.exe` / `qmbrowser.exe` processes using query and memory-read permissions only. It then requests the resource ekey from the QQ-operated `u.y.qq.com` endpoint through an undocumented client compatibility protocol. The application does not inject code, load QQ Music DLLs, request process write access or store session values and ekeys. Reports involving this path must contain only redacted error categories and version information, never raw request bodies, memory contents, session values, UINs or ekeys.

## Release integrity

v0.6.0 published Windows executables are intentionally unsigned and remain governed by the immutable v0.6.0 workflow. The v0.6.1 release branch also remains unsigned because SignPath has not been enabled; its dedicated workflow must build the exact tagged source, pass PE metadata and payload checks, and publish a SHA-256 file for every asset. Users should download only from this repository's GitHub Releases page and verify the checksum.

The proposed SignPath policy remains inactive. Enabling Authenticode requires a separate policy decision, protected credentials and workflow changes documented in [`SIGNING-POLICY.md`](SIGNING-POLICY.md).
