# 全站榜单-Rex/Fw/Capy模块（Go + Docker 版）

一个**常驻服务**（Go 实现、容器化部署）：每天定时抓取 **5 个榜单源**
（骨朵 / 豆瓣 / 芒果TV / 剧场平台 / 番剧），带**网页管理面板**（填 TMDB API Key、预览各源数据），
并**自动生成聚合 fw/rex/capy 模块 widget.js**。

> 本仓库为 Go + Docker 重写版：HTTP 接口、数据格式与原 Node.js 版 1:1 兼容；
> 镜像由 GitHub Actions 自动构建并推送到 `ghcr.io/syuim/vps-widget`。

## 抓取的 5 个源

| 源 | 内容 | 数据接口 |
|---|---|---|
| 骨朵热度 `guduo` | 剧集 / 综艺 / 动漫 / 电影 4 分类 | `/data/guduo.json` |
| 豆瓣热榜 `douban` | 9 个区域（大陆/欧美/日/韩/动漫/纪录/综艺…） | `/data/douban.json` |
| 芒果TV `mgtv` | 剧集 + 王牌综艺 | `/data/mgtv.json` |
| 剧场平台 `theater` | 15 个剧场（迷雾/白夜/X/恋恋…） | `/data/theater.json` |
| 番剧 `bangumi` | bgm.tv 排名榜（公开 sort=rank，无需登录） | `/data/bangumi.json` |

每个源都由对应 `internal/sources/<源>.go` 抓取 + TMDB 匹配，结果结构与原版 JSON 保持一致。

## 特性

- **端口 5555**（可用 `PORT` 环境变量改）
- **面板登录密码**：`ADMIN_PASSWORD` 环境变量初始化；未设置则首次启动随机生成并打印到容器日志；面板内可随时改密码
- **每日定时抓取**：默认每天 17:00（北京时间）串行抓 5 源，存到 `data/`
- **网页管理面板**：`http://<VPS>:5555/` —— 登录后填 TMDB Key、看各源状态、选源预览、手动更新、改密码
- **自动生成 widget.js**：一个聚合 fw/rex 模块，含 5 个榜单子模块，每模块带**排序方式**（默认原序/最近更新/最近发布/热度最高/流行趋势/高分优先），数据源指向本机
- **零依赖 + 小镜像**：纯标准库 Go 实现，镜像约 15MB（Alpine），多架构（linux/amd64 + linux/arm64）

> 说明：`/data/*.json` 与 `/widget.js` 对外公开（widget 需加载数据）；管理 API 与面板需登录。

## 部署（Docker）

镜像：`ghcr.io/syuim/vps-widget:latest`（GitHub Actions 自动构建）

### 方式一：docker compose（推荐）

```bash
mkdir -p ~/vps-widget/data && cd ~/vps-widget
curl -sLO https://raw.githubusercontent.com/syuim/vps-widget/main/docker-compose.yml
ADMIN_PASSWORD=你的密码 docker compose up -d
```

### 方式二：docker run

```bash
mkdir -p ~/vps-widget/data
docker run -d --name vps-widget --restart unless-stopped \
  -p 5555:5555 \
  -e ADMIN_PASSWORD=你的密码 \
  -e CONFIG_PATH=/app/data/config.json \
  -v ~/vps-widget/data:/app/data \
  ghcr.io/syuim/vps-widget:latest
```

> 面板密码：未设置 `ADMIN_PASSWORD` 时会在启动日志中随机生成（`docker logs vps-widget` 查看）；
> 首次登录后在面板内修改密码。

### 更新 / 卸载

```bash
# 更新到最新镜像
docker compose pull && docker compose up -d

# 卸载（数据保留在 ./data）
docker compose down
```

### 从旧版（Node / install.sh）迁移

数据与配置格式完全兼容：把旧部署的 `data/` 目录整体拷贝到本机 `./data/`，
再把旧 `config.json` 移到 `./data/config.json`，然后按上面方式启动容器即可
（密码、TMDB Key、已抓数据全部保留）。

## 环境变量

| 变量 | 说明 | 默认 |
|---|---|---|
| `PORT` | 服务监听端口（容器内） | `5555` |
| `ADMIN_PASSWORD` | 首次启动初始化面板密码（`config.json` 已有密码时忽略） | 随机生成并打印日志 |
| `CONFIG_PATH` | 配置文件路径 | `config.json`（容器部署建议 `/app/data/config.json`） |

## 使用流程

1. 浏览器打开 `http://<VPS>:5555/`
2. 「配置」里填 **TMDB API Key** 和 **VPS 对外地址** → 保存
3. 点「更新全部」或各源「更新」抓取（抓一次需数分钟，串行做 TMDB 匹配）
4. 「数据预览」选源查看抓到的片单
5. 「Widget 模块」复制模块地址，在 Forward 里添加即可

## HTTP 接口

| 方法 / 路径 | 说明 |
|---|---|
| `GET /` | 管理面板 |
| `GET /data/<源>.json` | 各源数据（CORS 已开） |
| `GET /widget.js` | 生成的聚合模块 |
| `GET /api/sources` | 各源状态 |
| `GET/POST /api/config` | 读/写配置 |
| `POST /api/update` | 更新全部（带 `{source:xxx}` 单源） |
| `GET /api/preview?source=xxx` | 预览 |
| `GET /api/widget` | widget 地址+代码 |

## 生成模块

聚合模块 `id: makka.vps.aggregator`，5 个子模块：`loadGuduo` / `loadDouban` / `loadMangoTV` /
`loadTheater` / `loadBangumi`。条目为 `VideoItem`（`type:"tmdb"` + 数字 `tmdbId` + `mediaType`），
点击走播放器内置 TMDB 详情页。数据源默认你填的 VPS 地址，也可在模块 `globalParams.baseUrl` 改。

## 本地开发

```bash
go run .          # 启动（默认端口 5555）
go test ./...     # 测试（含 widget 输出与 Node 原版的逐字节对比基线）
go build -o vps-widget .
```

## 防火墙

云厂商安全组放行 `5555/TCP`；主机防火墙：

```bash
sudo ufw allow 5555/tcp
```

## 镜像构建（维护者）

push 到 `main` 分支或打 `v*` tag 后，GitHub Actions 自动构建并推送镜像：

1. `go vet` + `go test`（含黄金基线对比）
2. 多架构构建（linux/amd64, linux/arm64）→ `ghcr.io/syuim/vps-widget`
   - `main` 分支 → `latest`
   - tag `v1.2.3` → `1.2.3` / `1.2`
