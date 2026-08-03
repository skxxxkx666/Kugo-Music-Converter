# Backend - Go 后端

## 1. 项目结构

```
backend/
├── cmd/
│   └── server/
│       └── main.go                  # 程序入口点
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
│   │   ├── error.go                 # 统一错误码定义
│   │   └── middleware.go            # 请求日志中间件
│   ├── logger/
│   │   └── logger.go                # 分级日志 (DEBUG/INFO/WARN/ERROR)
│   ├── service/
│   │   ├── decrypt.go               # 解密服务 (KGM/KGMA/VPR/KGG/NCM)
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

## 2. 构建

```bash
cd backend
go mod tidy

# Windows 64 位 (PowerShell)
$env:CGO_ENABLED="0"; $env:GOOS="windows"; $env:GOARCH="amd64"; go build -o bin/kugo-converter.exe ./cmd/server

# Windows 64 位 (Linux shell 交叉编译)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/kugo-converter.exe ./cmd/server

# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/kugo-converter-linux-amd64 ./cmd/server

# Linux arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/kugo-converter-linux-arm64 ./cmd/server

# macOS Intel
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/kugo-converter-darwin-amd64 ./cmd/server

# macOS Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/kugo-converter-darwin-arm64 ./cmd/server
```

## 3. 运行

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

## 4. 使用说明

- 启动后访问 `http://localhost:8080`，即可看到拖拽/多选上传界面。
- 支持输入格式：KGG、KGM、KGMA、VPR、NCM。
- 支持输出格式：MP3 (VBR 质量可选)、FLAC、WAV。
- 默认最大 500 个文件，单文件上限 1 GiB（可通过配置调整）。浏览器单次上传总计上限为 2028 MiB，超过时可分批处理或改用目录扫描。
- 支持并发转换 (1~6 线程)、SSE 流式进度、中途取消。

### 4.1 KGG 密钥加载

KGG (酷狗 Hi-Res) 文件需要 KGMusicV3.db 中的密钥才能解密。

- 自动检测：程序启动时自动搜索以下路径：
  - `%APPDATA%\KuGou8\KGMusicV3.db`
  - `%APPDATA%\KuGou\KGMusicV3.db`
  - `%LOCALAPPDATA%\KuGou8\KGMusicV3.db`
  - `%LOCALAPPDATA%\KuGou\KGMusicV3.db`
- 手动选择：在页面中使用"选择 DB 文件"按钮或手动输入路径。
- 上传方式：通过 `/api/upload-db` 接口上传 DB 文件。

密钥加载后立刻生效，无需重启。如果新下载的歌曲解密失败，通常是密钥映射未包含最新条目，请重新加载最新的 KGMusicV3.db。

## 5. API

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

## 6. 日志

- 格式：`YYYY-MM-DD HH:mm:ss [LEVEL] message`
- 级别：DEBUG / INFO / WARN / ERROR（默认 INFO）
- 控制方式：

```powershell
$env:LOG_LEVEL="DEBUG"; ./bin/kugo-converter.exe
```

## 7. 配置项

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

## 8. Tailwind 重新编译（离线，可选）

前端样式已预编译为 `public/vendor/tailwind.min.css`，**正常运行无需 Node 环境**。
仅当修改了 `public/**` 的类名需要重建样式时，按以下步骤操作：

```bash
npm install -D tailwindcss @tailwindcss/forms @tailwindcss/typography
npx tailwindcss -c tailwind.config.js -i public/src/input.css -o public/vendor/tailwind.min.css --minify
```

`tailwind.config.js` 中两个插件（`@tailwindcss/forms`、`@tailwindcss/typography`）
已改为**可选加载**：未安装时自动跳过，重编译不会因缺插件而失败（缺插件时
仅少量表单/排版样式不会生成，建议尽量按上面命令一并安装）。

## 9. 解密层说明（v0.5.0）

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
