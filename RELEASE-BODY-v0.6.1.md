## Kugo Music Converter v0.6.1

v0.6.1 在 v0.6.0 桌面版基础上增加 QQ 音乐 `.mflac` / `.mgg` 解密，并修复混合批次和请求级密钥隔离问题。

### 下载

| 场景 | 资产 |
|---|---|
| 推荐安装 | `Kugo-Music-Converter-v0.6.1-windows-amd64-setup.exe` |
| 便携运行 | `Kugo-Music-Converter-v0.6.1-windows-amd64.exe` |
| 无法安装系统 WebView2 | `Kugo-Music-Converter-v0.6.1-windows-amd64-webview2-setup.exe` 或 `-webview2.exe` |

Windows 10 / 11 x64；不支持 Windows 7。正式资产保持未签名，并提供独立 SHA-256 文件。v0.6.1 不接入 SignPath Foundation。

### MFLAC / MGG

- 旧式、尾部内嵌 ekey 的 MFLAC/MGG 继续离线解密；
- QQ 音乐新版 `musicex` 文件从 192 字节尾部读取 `media_mid` 和资源文件名；
- 程序优先从当前用户 QQ 音乐会话文件读取明确的 `authst` 字段；无有效字段时，对同用户的 `QQMusic.exe` / `qmbrowser.exe` 使用只读进程查询与内存读取权限寻找该字段；
- 通过 QQ 音乐运营的 `u.y.qq.com` GetEVkey 兼容端点取得请求级 ekey；该客户端协议未公开，不属于稳定公共 API；
- 音频内容和本地路径不会发送；UIN、会话值和 ekey 不进入 WebView、配置、历史、CSV、日志或磁盘缓存；
- 不使用 DLL 注入、固定函数偏移、辅助 EXE、进程写权限或管理员权限；
- STag 文件可以识别，但当前缺少可靠的资源元数据取钥路径，会返回明确的不支持提示；
- 修复“查找本机音乐”无法识别或错误归类 QQ 音乐：优先扫描 Windows“音乐”已知文件夹下的 `VipSongsDownload`，兼容已知文件夹重定向；当多个客户端目录互相包含时，NCM 与 MFLAC/MGG/QMC 仍按格式归入网易云和 QQ 音乐；不扫描 `downloadproxyNew` 临时缓存；
- MFLAC Copy 会用内嵌 FFmpeg 严格校验。若 raw 解密流仅在末尾带残缺 FLAC 包，则自动无损重建并复验；MGG Copy 保持原始 OGG。

每个包含 `musicex` 文件的转换批次都需要联网取钥。QQ 音乐未运行、未登录、会话过期、账号失去资源权限或网络不可用时，只影响对应的 modern QMC 文件，同批其他格式继续转换。

### 稳定性与错误隔离

- modern QMC 取钥异步运行，NCM/KWM/KGG/传统 QMC 等离线项目不等待网络预取；
- modern QMC 资源按精确 `media_mid + filename` 去重，保留大小写；
- GetEVkey 使用固定 HTTPS 主机、禁止重定向、20 秒单请求超时、60 秒批次预算和 1 MiB 响应上限；
- KGG 数据库失败只影响 KGG 项，不再令混合批次全部失败；
- KGG 显式数据库请求使用本次加载的 immutable key-map 快照，避免并发请求切换全局缓存后串用另一数据库；
- 新增尾部损坏、STag 不支持、未登录、会话过期、账号无权限、网络失败和协议异常等错误码。
- 应用内更新安装器始终优先从官方 GitHub Release 下载，直连失败时使用 `gh.h233.eu.org` 转发同一官方地址；两条路径都要求配套 SHA-256 校验通过后才启动安装器。

### 性能与转换设置

- KGG MAP 解密使用保持原算法边界语义的周期查表，减少逐字节平方和取模；
- KGG RC4 在每个读取块内复用工作区，减少大批量转换的临时内存分配；
- MP3 转码显式使用内嵌 FFmpeg 的 `libmp3lame`；
- 输出格式和 MP3 VBR 档位改用更明确的兼容性、是否重编码和近似码率说明，默认仍为 MP3 V2。

### 隐私提示

v0.6.1 将首次声明版本提升为 v2，升级用户会再次看到 modern QMC 网络取钥及只读会话访问说明。完整边界见 `PRIVACY.md` 和 `SECURITY.md`。

### 发布前验证

<details>
<summary>展开查看完整验证记录</summary>

- QMC 容器、尾部、MAP/RC4 分块、有界流和错误分流单元测试通过；
- GetEVkey 请求结构、敏感值不泄漏、超大响应、重定向、取消和批次时限测试通过；
- mixed batch 离线优先、资源大小写、会话访问短路和 Windows amd64/386 构建边界测试通过；
- 重叠客户端目录按文件格式归类，以及 GitHub 优先、备用地址兜底的更新下载顺序测试通过；
- QQ 音乐 PC 22.52 的 26 个真实 `musicex` MFLAC 与 16 个 MGG 均完成归类和转换验证，42/42 通过；
- Windows 11 上标准版与内置 WebView2 版安装器均通过安装、已安装程序自检、卸载和无残留检查；
- `go test ./...`、release build-tag、`go vet ./...`、前端语法和 Node 测试通过；
- 开发与发布构建使用 Go 1.26.6 和 Wails CLI v2.14.0；应用运行库也按 `go.mod` 固定为 Wails v2.14.0。

</details>

真实音乐文件、UIN、会话和 ekey 未加入仓库或测试夹具。
