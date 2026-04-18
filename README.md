# CXCDN

一个类jsDelivr的开源CDN服务，支持从npm和GitHub获取文件并进行全球分发加速。
本项目使用AI开发,有一些Bug,下一个版本会修复,欢迎提交Issue或Pull Request。

## 功能特性

- **NPM包文件分发**: 直接访问npm包中的任意文件
- **GitHub仓库文件分发**: 从GitHub仓库获取文件，支持分支、标签和commit
- **高性能**: 基于Cloudwego Hertz框架，使用epoll和连接池
- **智能缓存**: 内存缓存+磁盘持久化，减少重复请求
- **目录浏览**: 提供友好的Web界面浏览文件和版本
- **可配置性**: 支持命令行参数和环境变量配置

## 快速开始

### 安装

```bash
git clone https://github.com/your-repo/CXCDN.git
cd CXCDN
go build -o cxcdn ./cmd/main.go
```

### 运行

```bash
# 默认配置 (监听 :8080, 启用持久缓存)
./cxcdn

# 自定义端口
./cxcdn -addr ":9000"

# 自定义缓存文件路径
./cxcdn -cache-file "./cache.data"

# 禁用持久缓存
./cxcdn -no-cache
```

### Docker运行

```bash
docker run -d -p 8080:8080 \
  -v $(pwd)/cache.data:/data/cache.data \
  cxcdn:latest -cache-file "/data/cache.data"
```

## 使用示例

### NPM包文件

访问npm包中的特定文件:

```
/npm/{package}@{version}/{file}
/npm/vue@3.3.4/dist/vue.global.js
/npm/lodash@4.17.21/lodash.min.js
```

### NPM包文件列表

浏览npm包中的所有文件:

```
/npm/{package}@{version}/
/npm/vue@3.3.4/
/npm/react@18.2.0/
```

### GitHub仓库文件

从GitHub仓库获取文件:

```
/gh/{user}/{repo}@{ref}/{path}
/gh/vuejs/core@v3.3.4/README.md
/gh/facebook/react@main/packages/react/index.js
/gh/twitter/bootstrap@v5.3.0/dist/css/bootstrap.min.css
```

### GitHub目录列表

浏览GitHub仓库的目录结构:

```
/gh/{user}/{repo}@{ref}/
/gh/vuejs/core@main/
/gh/facebook/react@18.2.0/
```

## API文档

### 端点列表

| 端点 | 方法 | 说明 |
|------|------|------|
| `/` | GET | 首页，显示使用示例 |
| `/npm/:package/*file` | GET | NPM包文件 |
| `/gh/:user/:repo/*file` | GET | GitHub仓库文件 |
| `/_stats` | GET | 连接池统计信息 |

### NPM URL格式

```
/npm/vue@3.3.4/dist/vue.global.js
     │    │      │
     │    │      └── 文件路径
     │    └── 版本号 (@可省略)
     └── 包名
```

### GitHub URL格式

```
/gh/vuejs/core@v3.3.4/packages/vue/dist/vue.global.js
   │    │    │      │
   │    │    │      └── 文件路径
   │    │    └── 版本 (分支/tag/commit)
   │    └── 仓库名
   └── 用户名
```

### 版本说明

- **NPM**: 版本号 (如 `3.3.4`) 或标签 (如 `latest`, `next`)
- **GitHub**: 分支名 (`main`)、标签名 (`v3.3.4`) 或完整commit SHA

### 响应头

所有文件请求都包含以下响应头:

```
Content-Type: {根据文件类型}
Cache-Control: public, max-age=3600 (GitHub) / 31536000, immutable (NPM)
Access-Control-Allow-Origin: *
```

## 配置选项

### 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-addr` | 监听地址 | `:8080` |
| `-mode` | 服务器模式 | `hertz` |
| `-cache-file` | 缓存持久化文件路径 | `./cxcdn.cache` |
| `-no-cache` | 禁用磁盘缓存持久化 | `false` |

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `CXCDN_ADDR` | 监听地址 | `:8080` |
| `CXCDN_MODE` | 服务器模式 | `hertz` |
| `CXCDN_CACHE_FILE` | 缓存持久化文件路径 | `./cxcdn.cache` |
| `CXCDN_NO_CACHE` | 禁用磁盘缓存 | `false` |

## 缓存策略

### 缓存时间

| 类型 | 缓存时间 | 说明 |
|------|----------|------|
| NPM文件 | 1年 | `immutable`，永不过期 |
| NPM目录列表 | 5分钟 | 可重新验证 |
| GitHub文件 | 1小时 | 可重新验证 |
| GitHub目录列表 | 5分钟 | 可重新验证 |
| GitHub元数据 | 30分钟 | ref、tree、tags等 |

### 缓存持久化

启用缓存持久化后:
- 缓存数据每5分钟自动保存到磁盘
- 服务重启时自动恢复缓存
- 可显著减少冷启动时的API请求

```bash
./cxcdn -cache-file "/path/to/cache.data"
```

## 目录结构

```
CXCDN
├── cmd/
│   └── main.go              # 程序入口
├── internal/
│   ├── server/              # HTTP服务器
│   ├── handler/             # 请求处理器
│   ├── gh/                  # GitHub API交互
│   ├── registry/            # NPM Registry交互
│   ├── storage/             # 文件存储
│   ├── cache/               # 缓存管理
│   └── pool/                # 连接池
├── AGENTS.md                # 开发者文档
├── README.md                # 项目文档
├── go.mod                   # Go模块定义
└── go.sum                   # 依赖校验
```

## 技术栈

- **Web框架**: [Cloudwego Hertz](https://www.cloudwego.org/) - 高性能Go HTTP框架
- **缓存**: [go-cache](https://github.com/patrickmn/go-cache) - 内存缓存
- **序列化**: [Protocol Buffers](https://protobuf.dev/) - 高效二进制格式

## 性能优化

1. **连接池**: 复用HTTP连接，减少握手开销
2. **epoll**: 使用netpoll实现异步I/O
3. **缓存**: 多级缓存策略，减少上游请求
4. **并发控制**: 包级别锁避免重复下载

## 开发

### 前置要求

- Go 1.21+
- protoc (如需修改protobuf定义)

### 编译

```bash
go build -o cxcdn ./cmd/main.go
```

### 测试

```bash
# 启动服务
./cxcdn

# 测试NPM
curl http://localhost:8080/npm/vue@3.3.4/

# 测试GitHub
curl http://localhost:8080/gh/vuejs/core@main/

# 查看统计
curl http://localhost:8080/_stats
```

## 许可证

MIT License

## 致谢

- 灵感来自 [jsDelivr](https://www.jsdelivr.com/)
- 使用 [Cloudwego Hertz](https://www.cloudwego.org/) 作为Web框架

## 贡献

欢迎提交Issue和Pull Request来帮助改进这个项目！