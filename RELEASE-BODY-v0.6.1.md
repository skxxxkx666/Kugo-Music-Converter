## Kugo Music Converter v0.6.1

Kugo Music Converter 是一款 Windows 本机加密音频解密转换工具，支持酷狗、网易云、酷我和 QQ 音乐的多种加密格式转换为 MP3、FLAC、WAV 或原始音频格式。

v0.6.1 重点新增新版 QQ 音乐 `.mflac` / `.mgg` 支持，同时改进本机音乐发现、混合批次容错、KGG 转换性能和应用内更新安全性。

![Kugo Music Converter v0.6.1 界面](https://raw.githubusercontent.com/skxxxkx666/Kugo-Music-Converter/v0.6.1/assets/screenshot.png)

**亮点速览**：新增新版 MFLAC/MGG 解密 · 修复 QQ 音乐目录发现 · 混合批次互不牵连 · KGG 大批量转换更快 · MP3 音质说明更清楚 · 自动更新校验更严格

### 使用方法

#### Windows（不支持 Windows 7）

| 你的情况 | 下载这个 |
|---|---|
| 大多数用户（推荐） | `Kugo-Music-Converter-v0.6.1-windows-amd64-setup.exe`（安装器，不需要管理员权限） |
| 不想安装、直接运行 | `Kugo-Music-Converter-v0.6.1-windows-amd64.exe`（便携版） |
| 受管控环境或无法安装 WebView2 | `Kugo-Music-Converter-v0.6.1-windows-amd64-webview2-setup.exe` 或 `-webview2.exe`（内置 WebView2，体积明显更大） |

1. 下载适合的 EXE
2. 双击 EXE 启动
3. 拖入文件或文件夹，也可以使用“选择音频文件”、“添加文件夹”或“查找本机音乐”
4. 选择输出格式、音质、并发数和输出目录
5. 点击“开始转换”

首次启动会准备内嵌 FFmpeg，后续启动会直接复用已校验的本机缓存。内置 WebView2 版还会在首次启动时校验并解压 Fixed Runtime（便携版约 339 MiB，解压缓存约 661 MiB），因此第一次打开更慢且会占用额外磁盘空间。

### 系统要求

- Windows 10 / 11（64 位）
- 不支持 Windows 7
- 标准版需要 Microsoft Edge WebView2 Runtime；Windows 10/11 通常已经包含
- 内置 WebView2 版无需另外安装系统 WebView2 Runtime

### 本次更新

#### 新增 QQ 音乐 MFLAC / MGG 支持

- 支持新版 QQ 音乐 `musicex` MFLAC/MGG 文件；
- 继续支持尾部内嵌 ekey 的旧式 MFLAC/MGG，此类文件仍可离线转换；
- 新版文件本身不携带密钥，转换时程序会联网为每首歌实时获取解密密钥；
- 获取密钥需要本机存在有效的 QQ 音乐登录会话、当前账号仍拥有该歌曲权限且网络连接正常；如果无法从会话文件取得有效授权信息，请启动 QQ 音乐 PC 客户端并重新登录；
- 单个文件获取密钥失败只影响对应的 MFLAC/MGG，不会阻塞同一批次中的 KGG、NCM、KWM 或传统 QMC 文件；
- MFLAC 选择“保持原格式”时会校验并在必要时无损修复末尾损坏的 FLAC 数据；MGG 保持原始 OGG 音频；
- 可以识别 STag 变体，但暂时无法转换，转换时会显示明确的不支持提示。

#### 修复“查找本机音乐”中的 QQ 音乐识别

- 支持 Windows“音乐”文件夹被重定向到其他磁盘的情况；
- 优先查找 QQ 音乐的 `VipSongsDownload` 下载目录；
- 网易云与 QQ 音乐目录互相包含时，会根据文件格式正确归类 NCM、MFLAC、MGG 和传统 QMC；
- 不扫描 QQ 音乐临时下载缓存，减少重复项和不可转换文件。

#### 稳定性与批次容错

- QQ 音乐新版文件联网获取密钥不会阻塞同批离线格式；
- 修复混合批次中 KGG 数据库错误影响其他格式的问题；
- 不同转换请求使用各自的 KGG 密钥数据，避免并发任务互相干扰；
- 修复 QQ 音乐后台获取密钥时的意外异常可能导致程序直接退出的问题；
- 为尾部损坏、STag、未登录、会话过期、账号无该歌曲权限、网络失败和协议异常提供更明确的错误提示。

#### 性能与转换设置

- 优化 KGG MAP 解密计算，减少逐字节重复运算；
- 优化 KGG RC4 解密的临时内存分配，改善大批量转换表现；
- MP3 转码明确使用内嵌 FFmpeg 的 `libmp3lame` 编码器；
- 输出格式与 MP3 音质选项增加兼容性、是否重新编码及近似码率说明，默认仍为 MP3 V2。

#### 应用内更新

- 继续通过 GitHub Releases 检查新版本；
- SHA-256 校验文件必须从本项目的官方 GitHub Release 获取，无法取得可信校验文件时会停止自动更新；
- 取得官方校验文件后，安装器优先从 GitHub 下载；直连失败时可通过 `gh.h233.eu.org` 转发同一官方地址；
- 只有安装器通过官方 SHA-256 校验后，程序才会启动安装流程。

### 支持格式

| 来源 | 输入格式 | 输出格式 |
|---|---|---|
| 酷狗音乐 | `.kgg` `.kgm` `.kgma` `.vpr` | MP3、FLAC、WAV、保持原格式 |
| 网易云音乐 | `.ncm` | MP3、FLAC、WAV、保持原格式 |
| 酷我音乐 | `.kwm` | MP3、FLAC、WAV、保持原格式 |
| QQ 音乐 | `.mflac` `.mgg` `.qmc0` `.qmc2` `.qmc3` `.qmc4` `.qmc6` `.qmc8` `.qmcflac` `.qmcogg` `.tkm` | MP3、FLAC、WAV、保持原格式 |

> 新版 `musicex` MFLAC/MGG 转换时需要本机存在有效的 QQ 音乐登录会话并保持联网；如果无法取得有效授权信息，请启动 QQ 音乐 PC 客户端并重新登录。旧式内嵌 ekey 文件和其他格式均可离线转换。

### KGG 使用说明

KGG 文件需要酷狗客户端的 `KGMusicV3.db`。程序会优先自动检测，也可以在转换设置中手动选择或重新检测。

常见路径：

```text
%APPDATA%\KuGou8\KGMusicV3.db
```

如果新下载的 KGG 无法解密，请先在酷狗客户端播放一次歌曲，再重新检测最新数据库。

### 隐私与联网说明

- 音频内容和本地文件路径不会上传，解密与转码仍在本机完成；
- KGG、KGM、KGMA、VPR、NCM、KWM、传统 QMC 以及内嵌 ekey 的旧式 MFLAC/MGG 不需要联网获取密钥；
- 只有新版 QQ 音乐 `musicex` MFLAC/MGG 会在转换时联网获取对应歌曲的解密密钥，依赖本机 QQ 音乐 PC 客户端的登录状态；
- 程序以只读方式获取当前用户的 QQ 音乐会话信息，不修改 QQ 音乐进程，不需要管理员权限；
- QQ 音乐账户信息和解密密钥不会写入配置、历史、CSV、日志或磁盘缓存；
- 检查更新和用户确认后的自动更新也会访问网络。

完整隐私边界请查看 [PRIVACY.md](https://github.com/skxxxkx666/Kugo-Music-Converter/blob/v0.6.1/PRIVACY.md) 和 [SECURITY.md](https://github.com/skxxxkx666/Kugo-Music-Converter/blob/v0.6.1/SECURITY.md)。

### 软件签名与完整性

- v0.6.1 按项目决策保持未签名，Windows 可能显示“未知发布者”或 SmartScreen 提示；
- 每个发布资产都附有独立的 `.sha256` 校验文件，请只从本项目的 GitHub Releases 下载并核对；
- 应用内自动更新同样强制核对官方 SHA-256，校验不通过不会启动安装器。

### 从 v0.6.0 升级

直接运行 v0.6.1 安装器即可覆盖安装，不需要先卸载旧版；便携版直接替换 EXE。转换设置、历史记录和输出目录都会保留，已转换的音频文件不受影响（注意：卸载会一并删除设置和历史记录，覆盖安装则不会）。

升级后首次启动会重新显示一次开源与隐私说明，确认后即可继续使用；转换新版 MFLAC/MGG 前请先阅读上方“隐私与联网说明”。

### 发布前验证

<details>
<summary>展开查看完整验证记录</summary>

- QQ 音乐 PC 22.52 的 26 个新版 MFLAC 和 16 个 MGG 真实样本完成归类与转换验证，42/42 通过；
- QQ 音乐容器解析、MAP/RC4 分块解密、错误隔离、请求去重、超时与取消测试通过；
- KGG、NCM、KWM、传统 QMC 与新版 MFLAC/MGG 混合批次验证通过；
- QQ 音乐与网易云目录重叠、Windows“音乐”文件夹重定向和备用下载顺序测试通过；
- v0.6.1 标准版与内置 WebView2 版均通过发布构建检查，安装器通过干净安装、自检、卸载和无残留验证；
- `go test ./...`、发布标签测试、`go vet ./...`、前端语法和 Node 测试通过；
- 发布构建使用 Go 1.26.6、Wails CLI v2.14.0 和 Wails 应用运行库 v2.14.0。

</details>

### 开源许可

本项目基于 GNU General Public License v3.0 发布。内嵌 FFmpeg 与可选 WebView2 Fixed Runtime 的许可证和来源信息请查看 [THIRD-PARTY-NOTICES.md](https://github.com/skxxxkx666/Kugo-Music-Converter/blob/v0.6.1/THIRD-PARTY-NOTICES.md)。

---

遇到问题？[提交 Issue](https://github.com/skxxxkx666/Kugo-Music-Converter/issues)
