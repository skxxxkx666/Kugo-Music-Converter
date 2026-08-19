# Third-party notices

Kugo Music Converter is distributed under GPL-3.0. The complete license text is in [`COPYING`](COPYING).

The Windows executable also contains or uses the following third-party components. Copyright remains with the respective authors.

| Component | Version | License | Project |
|---|---:|---|---|
| FFmpeg | git `35c091f4b7fb19aee9dfcc6c60ace0be92907ce5` | GPL-3.0-or-later for the bundled configuration | <https://ffmpeg.org/> |
| Microsoft Edge WebView2 Fixed Runtime (optional build) | 151.0.4129.93 x64 | Microsoft software license terms | <https://developer.microsoft.com/microsoft-edge/webview2/> |
| Inter | bundled WOFF2 files | SIL Open Font License 1.1 | <https://github.com/rsms/inter> |
| Wails | 2.12.0 | MIT | <https://github.com/wailsapp/wails> |
| Unlock Music CLI algorithms | 0.2.12 | MIT | <https://git.unlock-music.dev/um/cli> |
| go-toast | 2.0.3 | Unlicense OR MIT | <https://git.sr.ht/~jackmordaunt/go-toast> |
| go-ole | 1.3.0 | MIT | <https://github.com/go-ole/go-ole> |
| zap | 1.27.0 | MIT | <https://github.com/uber-go/zap> |
| modernc SQLite | 1.37.0 | BSD-3-Clause | <https://pkg.go.dev/modernc.org/sqlite> |
| Go supplementary libraries | versions recorded in `backend/go.sum` | BSD-3-Clause | <https://go.googlesource.com/> |

The complete Go module dependency graph and immutable checksums are recorded in [`backend/go.mod`](backend/go.mod) and [`backend/go.sum`](backend/go.sum). Source code for the application, including the build scripts and dependency versions used for each release, is published in the corresponding Git tag.

FFmpeg is compressed into the Windows executable and extracted to a per-user cache at runtime. Its exact binary hash, build configuration and corresponding source location are documented in [`FFMPEG-SOURCE.md`](FFMPEG-SOURCE.md). The FFmpeg notice is in [`LICENSES/FFmpeg.txt`](LICENSES/FFmpeg.txt).

The optional WebView2 build embeds Microsoft's Fixed Version Runtime CAB and extracts it to a per-user cache on first launch. Its exact version, package hash and official source are documented in [`WEBVIEW2-SOURCE.md`](WEBVIEW2-SOURCE.md). The redistribution notice is in [`LICENSES/WebView2.txt`](LICENSES/WebView2.txt).

The desktop frontend embeds Inter WOFF2 font files. Inter is licensed under the SIL Open Font License 1.1; the copyright notice and complete license are in [`LICENSES/Inter.txt`](LICENSES/Inter.txt).
