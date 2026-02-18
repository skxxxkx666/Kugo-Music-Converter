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
│   │   ├── convert.go               # 服务启动、路由注册、路径解析
│   │   ├── convert_api.go           # POST /api/convert 同步转换
│   │   ├── sse.go                   # POST /api/convert-stream SSE 流式转换
│   │   ├── config_api.go            # GET /api/config 配置查询
│   │   ├── picker.go                # POST /api/pick-directory, /api/pick-db-file
│   │   ├── db_api.go                # POST /api/validate-db-path, /api/redetect-db, /api/upload-db
│   │   ├── scanner.go               # POST /api/scan-folders 目录扫描
│   │   ├── error.go                 # 统一错误码定义 (15 个错误码)
│   │   └── middleware.go            # 请求日志中间件
│   ├── logger/
│   │   └── logger.go                # 分级日志 (DEBUG/INFO/WARN/ERROR)
│   ├── service/
│   │   ├── decrypt.go               # 解密服务 (KGM/KGMA/VPR/KGG/NCM)
│   │   ├── transcode.go             # ffmpeg 转码 (MP3/FLAC/WAV)
│   │   ├── batch.go                 # 并发批量转换引擎
│   │   ├── dbfinder.go              # KGMusicV3.db 自动检测
│   │   └── filescan.go              # 目录递归扫描
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
- 默认最大 500 个文件，单文件上限 80 MB（可通过配置调整）。
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
| `max_file_size` | 80 MB | 单文件上传上限 |
| `max_files` | 500 | 最大文件数 |
| `concurrency` | 3 | 默认并发数 |
| `parse_form_memory` | 32 MB | 表单解析内存限制 |

支持 YAML 配置文件、环境变量 (`KGG_ADDR`, `KGG_FFMPEG_BIN` 等) 和 CLI 参数三种方式，优先级：CLI > 环境变量 > YAML > 默认值。