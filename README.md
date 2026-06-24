# PCBox

基于 Wails v2 的桌面端 TVBox 播放器，配合 TV-K（Android TV 盒子应用）使用。Go 后端 + React/TypeScript 前端。

## 功能特性

- 支持 HLS (m3u8)、MP4 等多种视频格式
- 通过 WebSocket 与 TV-K 配对，获取视频源
- 多源聚合搜索
- 历史记录同步
- 亮色/暗色/跟随系统主题
- 视频播放快捷键（空格播放/暂停、方向键快进快退、音量调节等）
- 窗口全屏 / 系统全屏
- 系统托盘支持
- 内置本地代理服务器，自动处理 CORS 和 M3U8 播放列表重写

## TV-K 下载

TV-K 是配套的 Android TV 盒子应用，用于解析视频源并推送到 PCBox 播放。

**蓝奏云下载：** https://wwblv.lanzoul.com/b0139erm1e
**密码：** 2i4s

**备用下载：** https://github.com/zaaack/PCBoxWails/releases/tag/v0.0.1

## 使用教程

### 1. 启动 PCBox

运行构建后的可执行文件，或通过 `wails dev` 启动开发版本。

### 2. 连接 TV-K

1. 确保 PC 和 Android 盒子在同一局域网
2. PCBox 启动后会自动开启 WebSocket 服务（默认端口 9898）
3. 在 Android 盒子上打开 TV-K 应用
4. 进入 TV-K 设置 → FreeBox 配对
5. 输入 PC 的 IP 地址和端口号，点击连接

### 3. 浏览与播放

1. 连接成功后，左侧会显示可用的视频源
2. 点击视频源加载首页内容
3. 点击视频封面查看详情
4. 选择集数开始播放

### 4. 搜索功能

1. 点击左侧 Search 图标
2. 输入关键词搜索
3. 可选择搜索的视频源

### 5. 快捷键

| 按键 | 功能 |
|------|------|
| 空格 | 播放/暂停 |
| ← → | 快退/快进 5 秒 |
| ↑ ↓ | 音量 +/- 10% |
| F / F11 | 窗口全屏 |
| Ctrl+F | 系统全屏 |
| M | 静音 |
| E | 显示/隐藏集数面板 |
| 0-9 | 跳转到 0%-90% |

### 6. 设置

点击左侧 Settings 图标可配置：

- **主题**：深色 / 浅色 / 跟随系统
- **WebSocket 服务**：启动/停止/修改端口

## 开发

### 环境要求

- Go 1.23+
- Node.js 18+
- npm / pnpm

### 安装依赖

```bash
npm install
```

### 启动开发环境

```bash
wails dev
```

### 构建

```bash
wails build
```

### 仅构建前端

```bash
# 在 frontend/ 目录下
npm install
npm run build
```

## 架构

```
Go 后端 (main.go, app.go, ws-server.go, proxy-server.go)
  ├── WebSocket 服务 (默认端口 9898)
  ├── 本地代理服务 (随机端口, 127.0.0.1)
  └── 通过 Wails IPC 暴露方法给前端

前端 (frontend/src/)
  ├── App.tsx — 根组件，初始化 WS 服务
  ├── store/index.ts — Zustand 状态管理，消息分发
  ├── lib/api.ts — IPC 封装 (window.go.main.App.*)
  ├── components/ — Home, Search, Player, VideoDetail, History, Settings, Sidebar
  └── wailsjs/go/main/ — 自动生成的 Go 绑定（请勿手动修改）
```

### WebSocket 消息协议

| Code | 方向 | 说明 |
|------|------|------|
| 100 | TV-K → PCBox | 注册设备 |
| 201 | PCBox → TV-K | 请求视频源列表 |
| 203 | PCBox → TV-K | 请求首页内容 |
| 205 | PCBox → TV-K | 请求分类内容 |
| 207 | PCBox → TV-K | 请求详情 |
| 209 | PCBox → TV-K | 请求播放地址 |
| 211 | PCBox → TV-K | 请求播放历史 |
| 213 | PCBox → TV-K | 搜索 |
| 215 | PCBox → TV-K | 保存播放历史 |

## 致谢

本项目参考了以下开源项目：

- [FreeBox](https://github.com/kknifer7/FreeBox) - 感谢 kknifer7 提供的 FreeBox 项目
- [TV-K](https://github.com/kknifer7/TV-K) - 感谢 kknifer7 提供的 TV-K 项目

## 技术栈

- Wails v2
- Go 1.23
- React 18
- TypeScript
- Vite
- Zustand（状态管理）
- video.js（视频播放）
- WebSocket（设备通信）
- gorilla/websocket

## 许可证

[MIT License](LICENSE)
