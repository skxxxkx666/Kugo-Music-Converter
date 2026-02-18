# Kugo Music Converter

酷狗/网易云加密音频批量转换工具（v0.2.2）

支持将 KGG、KGM、KGMA、VPR、NCM 等加密音频文件批量转换为 MP3、FLAC、WAV 格式。

## 功能特性

- 拖拽上传与批量转换
- 并发转换（1~6 线程可调）
- SSE 流式实时进度
- 中途取消转换
- 目录扫描、文件名提取、CSV 导出
- 历史记录与日志导出
- KGG 数据库自动检测 / 手动选择 / 上传

## 支持格式

| 输入格式 | 输出格式 |
|----------|----------|
| `.kgg` `.kgm` `.kgma` `.vpr` `.ncm` | `MP3` `FLAC` `WAV` |

## 快速开始

### 下载

前往 [Releases](../../releases) 页面下载最新版本压缩包，解压即可使用。

### 启动

优先使用（带可视化界面）：

```
双击 start.hta
```

备选方式（命令行）：

```
双击 start.bat
```

启动后浏览器自动打开 `http://localhost:8080`。

### 目录结构

```
Kugo-Music-Converter/
├── backend/
│   └── bin/
│       └── kugo-converter.exe   # 后端主程序
├── public/                      # 前端页面
│   ├── index.html
│   ├── app.js
│   ├── style.css
│   └── modules/
├── tools/
│   └── ffmpeg.exe               # 转码工具
├── output/                      # 默认输出目录
├── start.hta                    # 启动器（推荐）
├── start.bat                    # 启动器（备选）
└── COPYING                      # GPLv3 许可证
```

## 使用说明

### KGG 文件转换

KGG（酷狗 Hi-Res）文件需要 `KGMusicV3.db` 中的密钥才能解密：

- **自动检测**：程序启动时自动搜索系统中的酷狗数据库
- **手动选择**：在页面中点击"选择 DB 文件"按钮
- **上传方式**：通过页面上传 DB 文件

> 如果新下载的歌曲解密失败，通常是密钥映射未包含最新条目。
> 请在酷狗客户端播放一次歌曲，然后重新加载最新的 `KGMusicV3.db`。

Windows 系统中 DB 文件常见路径：

```
%APPDATA%\KuGou8\KGMusicV3.db
```

### 常见问题

**KGG 转换失败**
- 请确认 `KGMusicV3.db` 路径有效
- 可在酷狗客户端播放一次歌曲后重试

**无法转码**
- 请确认 `tools/ffmpeg.exe` 存在且可执行

**端口 8080 被占用**
- 程序会自动尝试释放端口，若仍失败请手动关闭占用进程

## 从源码构建

请参阅 [backend/README.md](backend/README.md) 了解构建步骤和 API 文档。

## 许可证

本项目基于 [GNU General Public License v3.0](COPYING) 发布。

