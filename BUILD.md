# 构建说明

本文档说明如何构建 flo 的多平台二进制文件。

## 支持的平台

- **macOS Intel (x64)**: `darwin/amd64`
- **macOS Apple Silicon (ARM64)**: `darwin/arm64`
- **Linux x64**: `linux/amd64`
- **Linux ARM64**: `linux/arm64`
- **Windows x64**: `windows/amd64`

## 方法一：GitHub Actions（推荐）

### 手动触发构建

我们提供了两个 GitHub Actions workflow：

#### 方式一：Build Multi-Platform Binaries (推荐)
1. 访问 GitHub 仓库的 Actions 页面
2. 选择 "Build Multi-Platform Binaries" workflow
3. 点击 "Run workflow"
4. 输入版本号（如 `v1.0.0`）
5. 点击 "Run workflow" 开始构建

#### 方式二：Build Multi-Platform Binaries (Simple) (备用)
如果主 workflow 遇到问题，可以使用简化版本：
1. 访问 GitHub 仓库的 Actions 页面
2. 选择 "Build Multi-Platform Binaries (Simple)" workflow
3. 点击 "Run workflow"
4. 输入版本号（如 `v1.0.0`）
5. 点击 "Run workflow" 开始构建

### 构建产物

- **Artifacts**: 构建完成后，可以在 Actions 页面下载各平台的二进制文件
- **Release**: 如果输入了版本号，会自动创建 GitHub Release 并上传所有二进制文件
- **Checksums**: 自动生成 SHA256 校验和文件

## 方法二：本地构建

### 前置要求

- Go 1.22 或更高版本
- Git
- Unix 系统（macOS/Linux）或 WSL（Windows）

### 使用构建脚本

```bash
# 构建开发版本
./scripts/build-all.sh

# 构建指定版本
./scripts/build-all.sh v1.0.0
```

### 手动构建单个平台

```bash
# macOS Intel
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o flo-macos-intel ./cmd/aliyun-pipelines-tui

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o flo-macos-arm64 ./cmd/aliyun-pipelines-tui

# Linux x64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o flo-linux-x64 ./cmd/aliyun-pipelines-tui

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o flo-linux-arm64 ./cmd/aliyun-pipelines-tui

# Windows x64
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o flo-windows-x64.exe ./cmd/aliyun-pipelines-tui
```

## 构建选项说明

### 编译标志

- `-ldflags="-s -w"`: 去除调试信息和符号表，减小二进制文件大小
- `-ldflags="-s -w -X main.version=v1.0.0"`: 同时设置版本信息
- `CGO_ENABLED=0`: 禁用 CGO，确保静态链接

### 环境变量

- `GOOS`: 目标操作系统
- `GOARCH`: 目标架构
- `CGO_ENABLED`: 是否启用 CGO（设为 0 表示禁用）

## 构建产物

### 文件命名规范

```
flo-{version}-{platform}-{arch}[.exe]
```

示例：
- `flo-v1.0.0-macos-intel-x64`
- `flo-v1.0.0-macos-aarch64`
- `flo-v1.0.0-linux-x64`
- `flo-v1.0.0-linux-arm64`
- `flo-v1.0.0-windows-x64.exe`

### 校验和文件

构建完成后会生成 `checksums.txt` 文件，包含所有二进制文件的 SHA256 校验和：

```bash
# 验证文件完整性
sha256sum -c checksums.txt
```

## 发布流程

### 自动发布（GitHub Actions）

1. 使用 GitHub Actions 构建时输入版本号
2. 构建完成后自动创建 GitHub Release
3. 上传所有平台的二进制文件和校验和文件
4. 生成详细的 Release Notes

### 手动发布

1. 使用本地构建脚本构建所有平台
2. 手动创建 GitHub Release
3. 上传 `dist/` 目录中的所有文件

## 故障排除

### 常见问题

**构建失败 - 依赖下载问题**
```bash
# 清理模块缓存
go clean -modcache
go mod download
```

**构建失败 - 交叉编译问题**
```bash
# 确保 Go 版本支持目标平台
go version
go env GOOS GOARCH
```

**权限问题**
```bash
# 给构建脚本添加执行权限
chmod +x scripts/build-all.sh
```

### 调试构建

```bash
# 启用详细输出
go build -v -ldflags="-s -w" -o flo ./cmd/aliyun-pipelines-tui

# 查看构建信息
go version -m flo
```

## 性能优化

### 减小二进制大小

```bash
# 使用 UPX 压缩（可选）
upx --best flo

# 去除更多调试信息
go build -ldflags="-s -w -buildid=" -trimpath -o flo ./cmd/aliyun-pipelines-tui
```

### 构建缓存

GitHub Actions 已配置 Go 模块缓存，本地构建可以利用：

```bash
# 预热缓存
go mod download
go build -i ./cmd/aliyun-pipelines-tui
``` 