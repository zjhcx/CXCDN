# CXCDN 开发指南

本文档为CXCDN项目的开发者和维护者提供详细的技术指南。

## 项目概述

CXCDN是一个类jsDelivr的CDN服务，使用Go语言开发，支持从npm和GitHub获取文件并进行缓存分发。

### 核心技术栈

- **Web框架**: Cloudwego Hertz (高性能Go HTTP框架)
- **缓存**: github.com/patrickmn/go-cache (内存缓存) + Protocol Buffers (持久化)
- **HTTP客户端**: Hertz内置连接池
- **语言**: Go 1.21+

## 项目架构

```
CXCDN
├── cmd/
│   └── main.go              # 程序入口，命令行参数解析
├── internal/
│   ├── server/
│   │   └── server.go       # Hertz服务器初始化与路由配置
│   ├── handler/
│   │   ├── gh.go           # GitHub文件处理 (GhFile, GhList)
│   │   ├── npm.go          # NPM包处理 (NpmFile, NpmList)
│   │   └── render.go       # HTML模板渲染 (文件列表、首页)
│   ├── gh/
│   │   ├── gh.go           # GitHub API交互 (获取文件、目录、tags、branches)
│   │   └── persist.go       # GitHub数据结构与缓存序列化
│   ├── registry/
│   │   ├── registry.go     # NPM Registry交互 (解析包、版本解析)
│   │   └── persist.go       # NPM包数据序列化
│   ├── storage/
│   │   └── npm.go          # NPM tarball下载与解压
│   ├── cache/
│   │   ├── cache.go        # 内存缓存管理与磁盘持久化
│   │   ├── cache.proto     # Protobuf缓存格式定义
│   │   └── cache.pb.go     # 生成的Protobuf代码
│   └── pool/
│       └── pool.go         # HTTP连接池管理
├── go.mod                  # Go模块定义
└── go.sum                  # 依赖校验
```

## 模块详解

### 1. 服务器模块 (server)

**文件**: [server.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/server/server.go)

负责初始化Hertz服务器，配置中间件和路由。

```go
// 核心配置
server.WithHostPorts(addr)           // 监听地址
server.WithTransport(netpoll.NewTransporter)  // 高性能传输
server.WithReadBufferSize(16*1024)    // 16KB读取缓冲
server.WithIdleTimeout(60*time.Second) // 60秒空闲超时
```

**路由配置**:
- `GET /` - 首页
- `GET /npm/:package/*file` - NPM文件路由
- `GET /gh/:user/:repo/*file` - GitHub文件路由
- `GET /_stats` - 连接池统计

### 2. 处理器模块 (handler)

#### 2.1 GitHub处理器 ([gh.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/handler/gh.go))

处理GitHub相关请求，包括文件获取和目录列表。

**URL格式**: `/gh/{user}/{repo}@{version}/{path}`

**核心函数**:
- `GhFile()` - 获取单个文件内容
- `GhList()` - 列出仓库根目录文件
- `GhListSubPath()` - 列出子目录文件

**响应头**:
```
Cache-Control: public, max-age=3600
Access-Control-Allow-Origin: *
```

#### 2.2 NPM处理器 ([npm.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/handler/npm.go))

处理NPM包文件请求。

**URL格式**: `/npm/{package}@{version}/{path}`

**核心函数**:
- `NpmFile()` - 获取npm包中的文件
- `NpmList()` - 列出包内文件

**响应头**:
```
Cache-Control: public, max-age=31536000, immutable
Access-Control-Allow-Origin: *
```

#### 2.3 渲染模块 ([render.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/handler/render.go))

负责HTML页面渲染，包括文件列表和首页。

**数据结构**:
```go
type FileEntry struct {
    Name    string  // 显示名称
    IsDir   bool    // 是否目录
    Path    string  // 文件路径
    Size    string  // 文件大小
    ModTime string  // 修改时间
}

type VersionOption struct {
    Version string  // 版本号
    URL     string  // 版本URL
    Current bool    // 是否当前版本
}
```

**支持的文件类型MIME**:
- `.js`, `.mjs` → `application/javascript`
- `.css` → `text/css`
- `.html` → `text/html`
- `.json` → `application/json`
- 图片类型: `.png`, `.jpg`, `.gif`, `.svg`, `.webp`
- 字体类型: `.woff`, `.woff2`, `.ttf`, `.eot`
- 其他: `.map`, `.ts`, `.wasm`, `.xml`, `.txt`, `.md`

### 3. GitHub模块 (gh)

**文件**: [gh.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/gh/gh.go)

与GitHub API交互，获取仓库文件和元数据。

**API端点**:
- `https://api.github.com/repos/{owner}/{repo}/commits/{ref}` - 解析ref
- `https://api.github.com/repos/{owner}/{repo}/git/trees/{sha}?recursive=1` - 获取目录树
- `https://api.github.com/repos/{owner}/{repo}/tags` - 获取标签列表
- `https://api.github.com/repos/{owner}/{repo}/branches` - 获取分支列表
- `https://raw.githubusercontent.com/{owner}/{repo}/{ref}/{path}` - 获取文件内容

**数据结构**:
```go
type GitHubTree struct {
    Sha       string           // 提交SHA
    Tree      []GitHubTreeItem // 目录项列表
    Truncated bool             // 是否被截断
}

type GitHubTreeItem struct {
    Path string // 文件路径
    Mode string // 文件模式
    Type string // "blob"或"tree"
    Sha  string // 文件SHA
    Size int64  // 文件大小
}
```

### 4. Registry模块 (registry)

**文件**: [registry.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/registry/registry.go)

与NPM Registry交互，解析包信息和版本。

**Registry URL**: `https://registry.npmjs.org/{package}`

**数据结构**:
```go
type NpmPackage struct {
    Name     string                // 包名
    Versions map[string]NpmVersion // 版本映射
    DistTags map[string]string     // 标签映射 (latest, next等)
}

type NpmVersion struct {
    Name    string // 完整名称
    Version string // 版本号
    Dist    struct {
        Tarball string // tarball URL
    }
}
```

### 5. 存储模块 (storage)

**文件**: [npm.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/storage/npm.go)

负责NPM tarball的下载、缓存和解压。

**本地缓存目录**: `.cache/npm/{package}@{version}/`

**处理流程**:
1. 检查本地是否已解压
2. 使用Hertz客户端下载tarball
3. 创建临时文件保存
4. 解压到缓存目录 (`package/` 前缀会被去除)
5. 返回文件内容或文件列表

**并发控制**:
- 使用包级别的互斥锁映射 (`pkgLocks`)
- 避免同一包同时解压

### 6. 缓存模块 (cache)

**文件**: [cache.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/cache/cache.go)

内存缓存管理，支持可选的磁盘持久化。

**配置**:
- 内存缓存过期: 30分钟
- 清理间隔: 10分钟
- 磁盘保存周期: 5分钟

**持久化格式**: Protocol Buffers ([cache.proto](file:///Users/zhangjiahui/Downloads/CXCDN/internal/cache/cache.proto))

**支持的缓存值类型**:
- `string` / `bool` / `[]byte`
- `[]string` (StringList)
- `[2]string` (FileContent: 内容+类型)
- `*GitHubTree` (GitHub目录树)
- `*NpmPackage` (NPM包信息)

**缓存键命名规范**:
- `gh:ref:{owner}/{repo}@{ref}` - GitHub ref解析
- `gh:tree:{owner}/{repo}:{sha}` - GitHub目录树
- `gh:file:{owner}/{repo}@{ref}:{path}` - GitHub文件
- `gh:tags:{owner}/{repo}` - GitHub标签列表
- `gh:branches:{owner}/{repo}` - GitHub分支列表
- `gh:page:{user}/{repo}` - GitHub页面缓存
- `npm:pkg:{name}` - NPM包信息
- `npm:file:{name}@{version}:{path}` - NPM文件
- `npm:files:{name}@{version}` - NPM文件列表
- `npm:page:{package}` - NPM页面缓存

### 7. 连接池模块 (pool)

**文件**: [pool.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/pool/pool.go)

管理HTTP客户端连接池，提高性能。

**配置**:
```go
maxConnsPerHost = 256           // 每主机最大连接数
maxIdleConnDuration = 90s      // 最大空闲连接时间
dialTimeout = 10s              // 拨号超时
connDuration = 30s             // 连接持续时间
readTimeout = 30s              // 读取超时
tarballConnDuration = 120s     // tarball下载连接超时
```

## 开发指南

### 开发流程

1. **环境准备**
   - 安装Go 1.21+
   - 安装protoc (如需修改protobuf定义)

2. **代码克隆与构建**
   ```bash
   git clone https://github.com/your-repo/CXCDN.git
   cd CXCDN
   go build -o cxcdn ./cmd/main.go
   ```

3. **本地测试**
   - 启动服务: `./cxcdn`
   - 访问 `http://localhost:8080` 查看首页
   - 测试NPM和GitHub文件访问

4. **代码规范**
   - 遵循Go标准代码风格
   - 使用 `go fmt` 格式化代码
   - 编写清晰的注释

### 添加新的处理器

1. 在 `internal/handler/` 创建新文件
2. 实现处理函数，签名: `func(ctx context.Context, c *app.RequestContext)`
3. 在 [server.go](file:///Users/zhangjiahui/Downloads/CXCDN/internal/server/server.go) 注册路由

### 添加新的缓存类型

1. 在 [cache.proto](file:///Users/zhangjiahui/Downloads/CXCDN/internal/cache/cache.proto) 添加message定义
2. 运行 `protoc --go_out=. internal/cache/cache.proto` 重新生成代码
3. 在对应模块的 `init()` 中注册marshaler/unmarshaler

### 添加新的数据源

1. 创建新模块目录 (如 `internal/{source}/`)
2. 实现API交互逻辑
3. 注册缓存序列化器
4. 在handler中调用

## 编译和运行

### 编译

```bash
go build -o cxcdn ./cmd/main.go
```

### 运行

```bash
# 默认配置 (启用持久缓存)
./cxcdn

# 自定义端口
./cxcdn -addr ":9000"

# 自定义缓存文件路径
./cxcdn -cache-file "./cache.data"

# 禁用持久缓存
./cxcdn -no-cache
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `CXCDN_ADDR` | 监听地址 | `:8080` |
| `CXCDN_MODE` | 服务器模式 | `hertz` |
| `CXCDN_CACHE_FILE` | 缓存持久化文件路径 | `./cxcdn.cache` |
| `CXCDN_NO_CACHE` | 禁用磁盘缓存 | `false` |

## 测试方法

```bash
# NPM包文件
curl http://localhost:8080/npm/vue@3.3.4/dist/vue.global.js

# NPM包文件列表
curl http://localhost:8080/npm/vue@3.3.4/

# GitHub文件
curl http://localhost:8080/gh/vuejs/core@v3.3.4/README.md

# GitHub目录列表
curl http://localhost:8080/gh/vuejs/core@main/

# 统计信息
curl http://localhost:8080/_stats
```

## Protobuf编译

如需修改缓存格式:

1. 安装protoc: `brew install protobuf`
2. 修改 [cache.proto](file:///Users/zhangjiahui/Downloads/CXCDN/internal/cache/cache.proto)
3. 重新生成: `protoc --go_out=. internal/cache/cache.proto`

## 贡献指南

### 提交代码

1. **Fork仓库**
2. **创建分支**
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **提交更改**
   ```bash
   git commit -m "Add your feature description"
   ```
4. **推送到远程**
   ```bash
   git push origin feature/your-feature-name
   ```
5. **创建Pull Request**

### 代码审查

- 确保代码符合项目规范
- 确保所有测试通过
- 提供清晰的提交信息
- 说明代码变更的原因和影响

## 性能优化建议

1. **缓存策略优化**
   - 合理设置缓存时间
   - 避免缓存过大的文件
   - 定期清理过期缓存

2. **并发控制**
   - 使用适当的锁机制
   - 避免阻塞操作
   - 合理使用goroutine

3. **网络优化**
   - 复用HTTP连接
   - 适当设置超时时间
   - 压缩传输数据

## 常见问题与解决方案

### 问题: GitHub API速率限制
**解决方案**:
- 实现指数退避重试机制
- 增加缓存时间
- 考虑使用GitHub API令牌

### 问题: NPM tarball下载失败
**解决方案**:
- 增加下载超时时间
- 实现重试机制
- 检查网络连接

### 问题: 内存使用过高
**解决方案**:
- 调整缓存大小
- 定期清理缓存
- 监控内存使用情况

## 部署建议

1. **生产环境配置**
   - 使用固定的缓存文件路径
   - 配置适当的端口和地址
   - 启用持久缓存

2. **监控与日志**
   - 监控服务健康状态
   - 记录关键操作日志
   - 监控缓存使用情况

3. **负载均衡**
   - 考虑使用多个实例
   - 配置负载均衡器
   - 共享缓存数据

## 注意事项

1. **并发安全**: 使用包级互斥锁管理共享资源
2. **缓存策略**: GitHub文件1小时缓存，NPM文件永久缓存(immutable)
3. **错误处理**: 所有API调用都检查状态码和错误
4. **资源清理**: 服务器关闭时保存缓存到磁盘
5. **CORS**: 所有响应都包含 `Access-Control-Allow-Origin: *`
6. **安全性**: 避免缓存敏感信息，定期清理缓存
