# Privacy policy

Last updated: 2026-08-19

Kugo Music Converter is a local desktop application. It does not operate an account service and does not collect analytics or telemetry.

## Local data processing

- Selected music files are decrypted and transcoded on the user's computer.
- Music files, `KGMusicV3.db`, encryption keys, conversion history and diagnostic logs are not uploaded by the application.
- Output files are written only to the directory selected by the user.
- Preferences and recent conversion history are stored in the local WebView application data.
- The user-triggered local music discovery reads known music-client configuration and download folders only to list supported files; discovered paths remain on the device and are not uploaded.
- The embedded FFmpeg executable is extracted to the per-user application cache and verified by SHA-256 before use.

Users control and may delete their output files, local application data, exported logs and FFmpeg cache at any time.

## Network access

Conversion does not start a local HTTP server and does not require an Internet connection. The optional update check sends a standard HTTPS request to the GitHub Releases API or a configured release mirror. It transmits the current application version and normal network metadata required for the request, but no music file, key database, conversion history or diagnostic log.

GitHub processes network metadata according to its own privacy statement: <https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement>.

## Diagnostic information

Diagnostic logs remain local until the user explicitly exports them. Users should review exported logs before attaching them to an issue and should not publish music files, key databases, credentials or personal paths unnecessarily.

## Changes and questions

Material privacy changes will be documented in the repository and release notes. Questions can be opened in the project's GitHub issue tracker without attaching private music files or key databases.
