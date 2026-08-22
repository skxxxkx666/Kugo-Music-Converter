# Backend - Go 后端

## v0.6.1 桌面端架构

v0.6.1 延续 Wails v2 单 EXE 架构，并新增 MFLAC/MGG 与请求级 QQ 音乐取钥。桌面应用支持文件与文件夹拖放、队列管理、设置持久化、批量转换、进度与 ETA、任务取消、失败恢复、试听与定位、历史、目录扫描、诊断、显式自动更新和运行时缓存清理。Windows 原生集成包含单实例、任务栏进度、完成通知和关窗确认。

当前已发布稳定版本为 v0.6.0，本分支版本为 v0.6.1（待发布）。v0.6.1 发布链继续生成标准版和内置 WebView2 版的便携 EXE 与按用户安装器，保持未签名并分别生成 SHA-256；本版本不接入 SignPath Foundation。

```powershell
cd backend

# 开发构建（不嵌入 FFmpeg）
wails build -trimpath -o Kugo-Music-Converter-v0.6.1-dev.exe

# 正式候选构建（从仓库根目录执行）
cd ..
.\build-release.ps1
```

两个正式构建首次启动时都会将 FFmpeg 解压到 `%LOCALAPPDATA%\Kugo Music Converter\runtime\<校验值>`。内置 WebView2 版还会把 Fixed Runtime 解压到 `%LOCALAPPDATA%\Kugo Music Converter\webview2\<版本>-<校验值>` 并设置 AppContainer 读/执行权限。后续启动会校验并复用缓存；生成的载荷、Wails 绑定和构建产物不会提交到 Git。

### 功能等价说明

| v0.5.x 能力 | v0.6.0 桌面实现 |
|---|---|
| 浏览器拖放、目录递归解析 | Wails 原生文件/目录拖放与目录选择 |
| 输出格式、音质、并发、输出目录、KGG 数据库 | 桌面设置区；偏好保存在 WebView 本机存储，数据库支持自动检测、手选和重新检测 |
| SSE 总进度、单文件结果、ETA、取消 | Wails 事件；显示总进度、当前文件/阶段、ETA，取消前使用原生确认窗口 |
| 成功试听、输出定位、失败详情与重试 | 队列结果行和任务结果操作区 |
| 扫描文件名、复制、CSV | 折叠式高级工具，支持多目录、递归和自定义扩展名 |
| 转换日志与历史 | 页面内诊断记录、LOG 导出、最近 50 次历史的恢复/删除/清空 |
| GitHub 更新提示 | 后端检查、更新摘要、忽略版本、官方安装器下载、SHA-256 校验和 Releases 回退 |
| 深色模式 | 跟随 Windows 系统主题，顶栏可手动切换并记忆 |

这里的“产品功能等价”不要求保留 `localhost`、HTTP 上传、SSE 或浏览器标签页；这些传输和启动方式由进程内 Wails 绑定替代。旧链路清理是后续独立阶段，指删除 `start.hta`、`start.bat`、`public/` 和 `cmd/server`；发布脚本已经切换到桌面单 EXE，本阶段仍不删除旧回退链路。

## 1. 项目结构

```
backend/
├── main.go                         # v0.6.0 Wails 桌面入口、单实例启动
├── app.go                          # 原生绑定、启动状态与关窗拦截
├── app_conversion.go               # 转换任务、取消与 Wails 事件
├── app_features.go                 # 扫描、导出、试听、更新等桌面方法
├── app_update.go                   # 官方 Release 自动更新下载、校验与安装器启动
├── app_cache.go                    # 应用运行时缓存统计与安全清理
├── app_music_discovery.go          # 查找本机音乐的分组、去重与受限扫描
├── app_music_discovery_windows.go  # Windows 音乐软件下载目录与四客户端配置检测
├── app_windows_integration.go      # Windows 原生集成（任务栏进度、Toast 通知、单实例）
├── app_integration_other.go        # 非 Windows 平台的空实现
├── frontend/
│   └── src/                        # v0.6.0 桌面界面
│       ├── index.html              # 语义结构与内联 SVG 图标精灵
│       ├── main.css                # 设计令牌、双主题、组件与动效
│       ├── main.js                 # 队列/设置/转换/历史/扫描/快捷键交互
│       ├── fonts/                  # 内嵌 Inter 可变字体
│       └── assets/                 # 应用 Logo 与酷我音乐品牌图标 PNG
├── cmd/
│   └── server/
│       └── main.go                  # v0.5.x HTTP 服务入口
├── internal/
│   ├── algo/
│   │   └── kgg/                     # KGG 纯 Go 解密实现
│   │       ├── decoder.go           # KGG 流式解码器 (Validate/Read)
│   │       ├── ekey.go              # ekey (v1/v2) 解析与 TEA-CBC
│   │       ├── qmc2.go              # QMC2 MAP/RC4 两种算法实现
│   │       ├── database.go          # KGMusicV3.db 解密与密钥映射读取
│   │       └── aes_cbc_std.go       # AES-CBC 封装
│   ├── config/
│   │   └── config.go                # 配置处理 (YAML + 环境变量 + CLI)
│   ├── handler/
│   │   ├── convert.go               # 服务启动、路由注册、静态文件服务
│   │   ├── convert_api.go           # POST /api/convert 同步转换
│   │   ├── convert_db.go            # DB 加载与密钥查询
│   │   ├── convert_paths.go         # 路径解析（baseDir/outputDir/ffmpeg）
│   │   ├── convert_runtime.go       # 运行时工具函数与错误映射
│   │   ├── sse.go                   # POST /api/convert-stream SSE 流式转换
│   │   ├── config_api.go            # GET /api/config 配置查询
│   │   ├── picker.go                # POST /api/pick-directory, /api/pick-db-file
│   │   ├── db_api.go                # POST /api/validate-db-path, /api/redetect-db, /api/upload-db
│   │   ├── scanner.go               # POST /api/scan-folders 目录扫描
│   │   ├── preview.go               # GET /api/preview-file 试听服务
│   │   ├── health.go                # GET /api/health 健康检查
│   │   ├── local_convert.go         # v0.6.0 本地路径转换适配层
│   │   ├── error.go                 # 统一错误码定义
│   │   └── middleware.go            # 请求日志中间件
│   ├── logger/
│   │   └── logger.go                # 分级日志 (DEBUG/INFO/WARN/ERROR)
│   ├── runtimebundle/               # FFmpeg 压缩载荷与 AppData 缓存
│   ├── service/
│   │   ├── decrypt.go               # 解密服务 (KGG/KGM/KGMA/VPR/NCM/KWM/QMC)
│   │   ├── transcode.go             # ffmpeg 转码 (MP3/FLAC/WAV)
│   │   ├── batch.go                 # 并发批量转换引擎
│   │   ├── dbfinder.go              # KGMusicV3.db 自动检测
│   │   ├── filescan.go              # 目录递归扫描
│   │   └── errors.go                # 服务层错误定义
│   └── utils/
│       └── utils.go                 # 通用工具
├── bin/
│   └── kugo-converter.exe           # 编译产物
├── go.mod
├── go.sum
└── config.example.yaml              # 示例配置文件
```

## 2. v0.6.0 桌面开发与验证

### 2.1 环境要求

- Windows 10 / 11 x64；
- Go 1.26；
- Wails CLI v2.14.0；
- NSIS 3；
- Microsoft Edge WebView2 Runtime；
- 正式构建需要联网获取已固定哈希的 FFmpeg 与 WebView2 载荷，已有校验通过的本机载荷会直接复用。

### 2.2 开发构建

```powershell
cd backend
wails build -trimpath -o Kugo-Music-Converter-v0.6.1-dev.exe
```

不带 `runtimebundle` 标签的构建不会嵌入 FFmpeg，应用会明确显示“开发构建未嵌入 FFmpeg”。

### 2.3 正式双版本构建

```powershell
cd ..
.\build-release.ps1
```

输出位于 `dist/release/`：

- `Kugo-Music-Converter-v0.6.0-windows-amd64.exe`：标准版，使用系统 WebView2（推荐）；
- `Kugo-Music-Converter-v0.6.0-windows-amd64-webview2.exe`：内置 Fixed Runtime 版，体积较大。
- `Kugo-Music-Converter-v0.6.0-windows-amd64-setup.exe`：标准版按用户安装器（推荐）；
- `Kugo-Music-Converter-v0.6.0-windows-amd64-webview2-setup.exe`：内置 Fixed Runtime 按用户安装器。

发布脚本强制为两个版本嵌入 FFmpeg，并只为第二个版本启用 `webview2bundle`。脚本验证 PE 元数据、两版体积差、未签名状态、SHA-256 和无界面运行时自检。安装器默认写入 `%LOCALAPPDATA%\Programs\Kugo Music Converter`，不要求管理员权限。`test-clean-install.ps1` 在干净测试机执行安装、自检、卸载和无残留门禁。v0.6.0 与 v0.6.1 均不经过 SignPath；`verify-release.ps1 -RequireSignature` 仅保留给可能启用签名的未来版本。

### 2.4 测试

```powershell
cd backend
node --check frontend/src/main.js
go test ./...
go test unlock-music.dev/cli/algo/qmc
go vet ./...

# 真实样本（按实际路径传参，缺失格式会明确列出）
cd ..
.\test-sample-coverage.ps1 -SampleDirectory C:\path\to\music
```

### 2.5 桌面方法

| 类别 | 方法 |
|---|---|
| 启动状态 | `GetStartupState` |
| 文件与目录 | `SelectAudioFiles`、`SelectOutputDirectory`、`SelectScanDirectory`、`ResolveDroppedPaths` |
| KGG 数据库 | `SelectDatabaseFile`、`RestoreDatabaseFile`、`RedetectDatabase` |
| 转换 | `StartConversion`、`CancelConversion`、`ConfirmConversionCancellation` |
| 输出结果 | `OpenOutputDirectory`、`OpenOutputFile`、`GetPreviewURL` |
| 高级扫描 | `ScanAudioDirectory`、`ScanDirectories` |
| 本机音乐发现 | `FindLocalMusic` |
| 导出和确认 | `SaveTextFile`、`ConfirmHistoryAction` |
| 更新 | `CheckForUpdates`、`DownloadAndInstallUpdate`、`OpenReleasePage` |
| 运行时缓存 | `GetRuntimeCacheInfo`、`ClearRuntimeCache` |

### 2.6 桌面事件

| 事件 | 负载 |
|---|---|
| `conversion:progress` | 当前文件、阶段、总体百分比和字节进度 |
| `conversion:file-done` | 单文件状态、输出路径或结构化错误 |
| `conversion:complete` | 成功数、失败数、耗时、输出目录和全部结果 |
| `conversion:error` | 任务级错误代码、用户提示、建议和详情 |

### 2.7 运行时安全边界

- 转换不监听 TCP 端口；
- 文件、目录和数据库通过原生对话框或受控拖放选择；
- 拖放、扫描和导出拒绝网络共享路径；
- 文本导出只允许 CSV、TXT、LOG，最大 8 MiB；
- 试听只允许本次进程注册的结果文件；
- GitHub 跳转只允许本项目 Releases 页面；
- 扫描超时 30 秒，转换队列上限 500，高级扫描上限 50,000。

## 3. v0.5.x 旧 HTTP 服务（迁移期参考）

以下内容描述 `cmd/server` 和 `../public/`。它们不属于 v0.6.0 正式桌面运行链路，当前仅用于对照、回归和回退。

### 3.1 运行

```bash
# 使用默认配置
./bin/kugo-converter.exe

# 指定配置文件
./bin/kugo-converter.exe --config config.yaml

# 指定监听地址和 ffmpeg 路径
./bin/kugo-converter.exe --addr :9090 --ffmpeg tools/ffmpeg.exe

# 显示帮助
./bin/kugo-converter.exe --help
```

### 3.2 使用说明

- 启动后访问 `http://localhost:8080`，即可看到拖拽/多选上传界面。
- 支持输入格式：KGG、KGM、KGMA、VPR、NCM、KWM，以及 QMC0/QMC2/QMC3/QMC4/QMC6/QMC8/QMCFLAC/QMCOGG/TKM。
- 支持输出格式：MP3 (VBR 质量可选)、FLAC、WAV。
- 默认最大 500 个文件，单文件上限 1 GiB（可通过配置调整）。浏览器单次上传总计上限为 2028 MiB，超过时可分批处理或改用目录扫描。
- 支持并发转换 (1~6 线程)、SSE 流式进度、中途取消。

#### 3.2.1 KGG 密钥加载

KGG (酷狗 Hi-Res) 文件需要 KGMusicV3.db 中的密钥才能解密。

- 自动检测：程序启动时自动搜索以下路径：
  - `%APPDATA%\KuGou8\KGMusicV3.db`
  - `%APPDATA%\KuGou\KGMusicV3.db`
  - `%LOCALAPPDATA%\KuGou8\KGMusicV3.db`
  - `%LOCALAPPDATA%\KuGou\KGMusicV3.db`
- 手动选择：在页面中使用"选择 DB 文件"按钮或手动输入路径。
- 上传方式：通过 `/api/upload-db` 接口上传 DB 文件。

密钥加载后立刻生效，无需重启。如果新下载的歌曲解密失败，通常是密钥映射未包含最新条目，请重新加载最新的 KGMusicV3.db。

### 3.3 HTTP API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 静态文件服务 (前端页面) |
| GET | `/api/config` | 获取运行时配置与 DB 状态 |
| POST | `/api/convert` | 同步批量转换 |
| POST | `/api/convert-stream` | SSE 流式转换 (实时进度) |
| POST | `/api/upload-db` | 上传 KGMusicV3.db 并加载密钥 |
| POST | `/api/validate-db-path` | 验证 DB 路径有效性 |
| POST | `/api/redetect-db` | 重新自动检测 DB |
| POST | `/api/pick-directory` | 打开文件夹选择对话框 |
| POST | `/api/pick-db-file` | 打开 DB 文件选择对话框 |
| POST | `/api/scan-folders` | 递归扫描目录中的加密文件 |
| GET | `/api/preview-file` | 试听已转换的音频文件 |
| GET | `/api/health` | 健康检查与版本信息 |
| POST | `/api/open-folder` | 用资源管理器打开指定目录 |

### 3.4 服务日志

- 格式：`YYYY-MM-DD HH:mm:ss [LEVEL] message`
- 级别：DEBUG / INFO / WARN / ERROR（默认 INFO）
- 控制方式：

```powershell
$env:LOG_LEVEL="DEBUG"; ./bin/kugo-converter.exe
```

### 3.5 旧服务配置项

| 配置键 | 默认值 | 说明 |
|--------|--------|------|
| `addr` | `:8080` | 监听地址 |
| `ffmpeg_bin` | `tools/ffmpeg.exe` | ffmpeg 可执行文件路径 |
| `public_dir` | `public` | 前端静态文件目录 |
| `default_output` | `output` | 默认输出目录 |
| `max_file_size` | 1 GiB | 单文件上传上限 |
| `max_files` | 500 | 最大文件数 |
| `concurrency` | 3 | 默认并发数 |
| `parse_form_memory` | 32 MB | 表单解析内存限制 |

支持 YAML 配置文件、环境变量 (`KGG_ADDR`, `KGG_FFMPEG_BIN` 等) 和 CLI 参数三种方式，优先级：CLI > 环境变量 > YAML > 默认值。

### 3.6 旧浏览器前端 Tailwind 重新编译（离线，可选）

前端样式已预编译为 `public/vendor/tailwind.min.css`，**正常运行无需 Node 环境**。
仅当修改了 `public/**` 的类名需要重建样式时，按以下步骤操作：

```bash
npm install -D tailwindcss @tailwindcss/forms @tailwindcss/typography
npx tailwindcss -c tailwind.config.js -i public/src/input.css -o public/vendor/tailwind.min.css --minify
```

`tailwind.config.js` 中两个插件（`@tailwindcss/forms`、`@tailwindcss/typography`）
已改为**可选加载**：未安装时自动跳过，重编译不会因缺插件而失败（缺插件时
仅少量表单/排版样式不会生成，建议尽量按上面命令一并安装）。

## 4. 解密层说明（v0.5.0 起沿用）

`internal/algo/kgg` 为自研实现，已与 `unlock-music.dev/cli` 的标准实现逐行
对照并以真实文件逐字节回归：

- QMC2 MAP / RC4、ekey(V1/V2 TEA-CBC)、KGMusicV3.db 解密均与 unlock-music 等价；
- KGMusicV3.db **全程内存解密**（不再写明文到 `%TEMP%`），页 1 带完整性校验，
  master key 支持 `KGG_DB_MASTER_KEY`（hex）环境变量覆盖；
- 解库结果按 `dbPath + 大小 + mtime` 进程级缓存（用户重载 DB 后自动失效）。

回归测试（默认跳过，需真实样本）：

```powershell
$env:KGG_DB="...\KGMusicV3.db"; $env:KGG_FILE="...\song.kgg"
go test ./internal/algo/kgg/ -run "TestOracleRealFileRegression|TestLoadKGDatabaseKeyMap" -v
```

`internal/algo/qmcfile` 负责识别 legacy / QTag / STag / `musicex` 尾部，并为 `musicex` 构造只读取加密音频负载的有界 QMC2 流；`internal/qmckey` 只在 Windows amd64 桌面路径读取当前用户 QQ 音乐会话：优先解析 `SetCookie.dat/_SetCookie.dat` 中明确的 `authst` 字段，必要时以只读权限扫描同用户、QQMusic 安装目录下的 `QQMusic.exe/qmbrowser.exe`，再通过 QQ 音乐运营的未公开 GetEVkey 兼容端点获取请求级 ekey。会话和 ekey 不进入 WebView、配置、历史或日志。旧 `cmd/server` 构造器不启用该能力。

新版 QMC 真实文件测试需要 QQ 音乐 PC 客户端已运行并登录下载文件的账号：

```powershell
$env:KUGO_TEST_INPUT="...\sample.mflac" # 或 sample.mgg
$env:KUGO_TEST_FFMPEG="...\ffmpeg.exe"
$env:KUGO_TEST_OUTPUT_FORMAT="copy"
go test ./internal/handler -run '^TestConvertLocalPathsRealFile$' -count=1 -v
```

桌面本地转换核心的其他真实文件测试同样由环境变量门控，默认 Copy；设置 `KUGO_TEST_OUTPUT_FORMAT=wav` 可强制覆盖 FFmpeg 转码路径：

```powershell
$env:KUGO_TEST_INPUT="...\sample.kgma"
$env:KUGO_TEST_FFMPEG="...\ffmpeg.exe"
$env:KUGO_TEST_OUTPUT_FORMAT="wav"
go test ./internal/handler -run '^TestConvertLocalPathsRealFile$' -count=1 -v
```
