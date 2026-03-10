# Kugo Music Converter

[![Release](https://img.shields.io/github/v/release/skxxxkx666/Kugo-Music-Converter?label=Release&color=blue)](https://github.com/skxxxkx666/Kugo-Music-Converter/releases/latest)
[![License](https://img.shields.io/github/license/skxxxkx666/Kugo-Music-Converter?label=License&color=green)](COPYING)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows%20x64-0078D4?logo=windows&logoColor=white)](#)
[![Downloads](https://img.shields.io/github/downloads/skxxxkx666/Kugo-Music-Converter/total?label=Downloads&color=orange)](https://github.com/skxxxkx666/Kugo-Music-Converter/releases)

酷狗 / 网易云加密音频批量转换工具，支持将 KGG、KGM、KGMA、VPR、NCM 等加密音频文件批量转换为 MP3、FLAC、WAV 格式。

---

## 下载安装

**最新版本：v0.4.0**

| 文件 | 平台 | 说明 |
|------|------|------|
| [Kugo-Music-Converter-v0.4.0-windows-amd64.zip](https://github.com/skxxxkx666/Kugo-Music-Converter/releases/download/v0.4.0/Kugo-Music-Converter-v0.4.0-windows-amd64.zip) | Windows x64 | 解压即用，内含 ffmpeg |

> 更多版本请前往 [Releases](https://github.com/skxxxkx666/Kugo-Music-Converter/releases) 页面。

## 功能特性

- 拖拽上传与批量转换（支持文件夹递归解析）
- 全离线运行，无需联网（所有前端资源本地化）
- 全新 UI（Tailwind CSS + Inter 字体，纯色风格，深色模式）
- 并发转换（1 ~ 6 线程可调）
- SSE 流式实时进度与 ETA 预估
- 中途取消转换
- 音频试听（转换完成后直接播放）
- 目录扫描、文件名提取、CSV 导出
- 历史记录管理与日志导出
- KGG 数据库自动检测 / 手动选择 / 上传
- OGG CRC 容错转码（自动重试 + 原格式保底输出）
- 扩展音频格式检测（AAC / M4A / WMA / DFF）
- HEX 头部诊断（无法识别时输出前 16 字节）
- 更新检测后端代理（国内网络可用）

## 支持格式

| 输入格式 | 输出格式 |
|----------|----------|
| `.kgg` `.kgm` `.kgma` `.vpr` `.ncm` | `MP3` `FLAC` `WAV` `原格式（Copy）` |

## 快速开始

1. 下载并解压压缩包
2. 双击 `start.hta` 启动（推荐），或双击 `start.bat`（备选）
3. 浏览器自动打开 `http://localhost:8080`

### 目录结构

```
Kugo-Music-Converter/
├── backend/bin/kugo-converter.exe   # 后端主程序
├── public/                          # 前端页面
│   ├── index.html
│   ├── app.js / style.css
│   ├── modules/                     # JS 模块（14 个）
│   └── vendor/                      # 本地化资源
│       ├── tailwind.min.css         # Tailwind CSS（预编译）
│       ├── lucide.min.js            # Lucide Icons
│       ├── gsap.min.js              # GSAP 动画库
│       └── fonts/inter-*.woff2      # Inter 字体
├── tools/ffmpeg.exe                 # 转码工具
├── output/                          # 默认输出目录
├── start.hta                        # 启动器（推荐）
├── start.bat                        # 启动器（备选）
└── COPYING                          # GPLv3 许可证
```

## 使用说明

### KGG 文件转换

KGG（酷狗 Hi-Res）文件需要 `KGMusicV3.db` 中的密钥才能解密：

- **自动检测** -- 程序启动时自动搜索系统中的酷狗数据库
- **手动选择** -- 在页面中点击"选择 DB 文件"按钮
- **上传方式** -- 通过页面上传 DB 文件

> 如果新下载的歌曲解密失败，通常是密钥映射未包含最新条目。
> 请在酷狗客户端播放一次歌曲，然后重新加载最新的 `KGMusicV3.db`。

Windows 系统中 DB 文件常见路径：

```
%APPDATA%\KuGou8\KGMusicV3.db
```

### 常见问题

| 问题 | 解决方法 |
|------|----------|
| KGG 转换失败 | 确认 `KGMusicV3.db` 路径有效，或在酷狗客户端播放一次歌曲后重试 |
| 无法转码 | 确认 `tools/ffmpeg.exe` 存在且可执行 |
| 端口 8080 被占用 | 程序会自动尝试释放端口，若仍失败请手动关闭占用进程 |
| 页面无样式/图标 | v0.4.0 已全部本地化，如仍异常请重新下载最新版 |

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.26, net/http, goroutine 并发 |
| 前端 | Vanilla JS (ES Modules), Tailwind CSS, Lucide Icons, GSAP |
| 解密 | QMC2 (AES-CBC / MAP / RC4), KGM, NCM |
| 转码 | FFmpeg (外部进程调用) |

## 从源码构建

请参阅 [backend/README.md](backend/README.md) 了解构建步骤和 API 文档。

## 更新日志

详见 [Releases](https://github.com/skxxxkx666/Kugo-Music-Converter/releases) 页面。

## 许可证

本项目基于 [GNU General Public License v3.0](COPYING) 发布。

