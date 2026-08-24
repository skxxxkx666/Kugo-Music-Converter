# Bundled FFmpeg source and build provenance

Kugo Music Converter v0.6.0 and v0.6.1 embed the following Windows x64 FFmpeg executable:

| Field | Value |
|---|---|
| Distributor | Gyan.dev Windows builds |
| Binary build | `2025-03-31-git-35c091f4b7-essentials_build-www.gyan.dev` |
| FFmpeg commit | `35c091f4b7fb19aee9dfcc6c60ace0be92907ce5` |
| Compiler | GCC 14.2.0, MSYS2 |
| SHA-256 | `128cdaa01cfd6a72d961ccb6777adb2c32278091a203f2b8ac83f7b5a181dd7f` |
| License | GPL-3.0-or-later for this enabled configuration |

## Corresponding source

- Exact FFmpeg source archive: <https://github.com/FFmpeg/FFmpeg/archive/35c091f4b7fb19aee9dfcc6c60ace0be92907ce5.tar.gz>
- Exact FFmpeg commit: <https://github.com/FFmpeg/FFmpeg/commit/35c091f4b7fb19aee9dfcc6c60ace0be92907ce5>
- Binary distributor and build resources: <https://www.gyan.dev/ffmpeg/builds/>
- Distributor support repository: <https://github.com/GyanD/codexffmpeg>
- Release-build acquisition mirror: <https://github.com/skxxxkx666/Kugo-Music-Converter/releases/download/v0.5.1/Kugo-Music-Converter-v0.5.1-windows-amd64.zip>

`prepare-ffmpeg.ps1` retrieves the already-published v0.5.1 package and accepts only the exact FFmpeg executable recorded above. The application source tag contains this provenance document, the GPL text and the release scripts; each Windows asset is published with its own SHA-256 file. If the embedded FFmpeg executable is replaced, update the commit, SHA-256, version output and build configuration in this file before release.

## Build configuration

```text
--enable-gpl --enable-version3 --enable-static --disable-w32threads
--disable-autodetect --enable-fontconfig --enable-iconv --enable-gnutls
--enable-libxml2 --enable-gmp --enable-bzlib --enable-lzma --enable-zlib
--enable-libsrt --enable-libssh --enable-libzmq --enable-avisynth
--enable-sdl2 --enable-libwebp --enable-libx264 --enable-libx265
--enable-libxvid --enable-libaom --enable-libopenjpeg --enable-libvpx
--enable-mediafoundation --enable-libass --enable-libfreetype
--enable-libfribidi --enable-libharfbuzz --enable-libvidstab
--enable-libvmaf --enable-libzimg --enable-amf --enable-cuda-llvm
--enable-cuvid --enable-dxva2 --enable-d3d11va --enable-d3d12va
--enable-ffnvcodec --enable-libvpl --enable-nvdec --enable-nvenc
--enable-vaapi --enable-libgme --enable-libopenmpt
--enable-libopencore-amrwb --enable-libmp3lame --enable-libtheora
--enable-libvo-amrwbenc --enable-libgsm --enable-libopencore-amrnb
--enable-libopus --enable-libspeex --enable-libvorbis
--enable-librubberband
```

The authoritative configuration is also printed by the bundled executable:

```powershell
ffmpeg.exe -buildconf
```
