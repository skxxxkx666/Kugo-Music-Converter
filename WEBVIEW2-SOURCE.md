# Bundled WebView2 source and provenance

Kugo Music Converter v0.6.0 provides an optional Windows x64 build that embeds the Microsoft Edge WebView2 Fixed Version Runtime.

| Field | Value |
|---|---|
| Product | Microsoft Edge WebView2 Fixed Version Runtime |
| Version | `151.0.4129.93` |
| Architecture | `x64` |
| Package | `Microsoft.WebView2.FixedVersionRuntime.151.0.4129.93.x64.cab` |
| Package size | `307214523` bytes |
| SHA-256 | `1cb7106545f5aee92ee16496347a0e775a351cb5a3816d072f04323695899bde` |
| Publisher | Microsoft Corporation |

## Official source

- Fixed Version Runtime download page: <https://developer.microsoft.com/microsoft-edge/webview2/>
- Pinned x64 package: <https://msedge.sf.dl.delivery.mp.microsoft.com/filestreamingservice/files/1424552f-1033-46d3-a1ea-26c879f4262b/Microsoft.WebView2.FixedVersionRuntime.151.0.4129.93.x64.cab>
- Microsoft distribution guidance: <https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution>

`backend/prepare-webview2-runtime.ps1` downloads or accepts this exact CAB, verifies the pinned SHA-256 and checks that the package contains `msedgewebview2.exe`. The ignored CAB payload is generated locally or in the trusted release workflow and is not committed to Git.

At first launch, the WebView2 build verifies the embedded CAB again, extracts it into `%LOCALAPPDATA%\Kugo Music Converter\webview2\<version>-<hash>`, grants the Microsoft-documented AppContainer read/execute permissions and directs Wails to the extracted Fixed Runtime. Later launches reuse the validated cache.

The verified v0.6.0 x64 payload expands to 259 files totaling `693133593` bytes (about 661 MiB). The release executable is about 339 MiB because it contains the CAB plus the application and embedded FFmpeg.

The Fixed Version Runtime does not update itself. A newer application release must update the pinned version, URL, size and SHA-256 in this document and in the preparation script.
