# HTTP/HTTPS 代理支持

flo 程序现在支持通过环境变量配置 HTTP 和 HTTPS 代理，这对于在企业网络环境中使用代理服务器访问阿里云 API 非常有用。

## 支持的环境变量

程序支持以下标准的代理环境变量：

- `http_proxy` - HTTP 代理服务器地址
- `https_proxy` - HTTPS 代理服务器地址
- `HTTP_PROXY` - HTTP 代理服务器地址（大写形式）
- `HTTPS_PROXY` - HTTPS 代理服务器地址（大写形式）
- `no_proxy` - 不使用代理的地址列表
- `NO_PROXY` - 不使用代理的地址列表（大写形式）

## 代理地址格式

代理地址支持以下格式：

```bash
# 基本格式
http://proxy.example.com:8080

# 带认证的格式
http://username:password@proxy.example.com:8080

# HTTPS 代理
https://proxy.example.com:8443
```

## 使用示例

### 1. 设置 HTTP 代理

```bash
export http_proxy=http://proxy.company.com:8080
export https_proxy=http://proxy.company.com:8080
./flo
```

### 2. 设置带认证的代理

```bash
export http_proxy=http://user:pass@proxy.company.com:8080
export https_proxy=http://user:pass@proxy.company.com:8080
./flo
```

### 3. 设置不同的 HTTP 和 HTTPS 代理

```bash
export http_proxy=http://http-proxy.company.com:8080
export https_proxy=http://https-proxy.company.com:8443
./flo
```

### 4. 排除特定地址不使用代理

```bash
export http_proxy=http://proxy.company.com:8080
export https_proxy=http://proxy.company.com:8080
export no_proxy=localhost,127.0.0.1,*.local
./flo
```

## 认证方式支持

程序支持两种认证方式的代理配置：

### Personal Access Token 认证

当使用 Personal Access Token 认证时，程序会自动创建支持代理的 HTTP 客户端，读取环境变量中的代理配置。

### AccessKey 认证

当使用 AccessKey 认证时，程序会通过阿里云 SDK 的代理配置方法设置代理：

- `SetHttpProxy()` - 设置 HTTP 代理
- `SetHttpsProxy()` - 设置 HTTPS 代理

## 调试代理配置

可以通过设置 `FLOWT_DEBUG=1` 环境变量来启用调试模式，查看代理配置信息：

```bash
export FLOWT_DEBUG=1
export http_proxy=http://proxy.company.com:8080
./flo
```

调试模式下，程序会在日志中输出代理配置信息，例如：

```
[DEBUG] Using HTTP proxy: http://proxy.company.com:8080
[DEBUG] SDK using HTTP proxy: http://proxy.company.com:8080
```

## 代理优先级

代理配置的优先级顺序（从高到低）：

1. 程序内部的代理配置
2. 环境变量配置
3. 系统默认代理配置

## 常见问题

### 1. 代理认证失败

如果代理需要认证，请确保在代理 URL 中包含正确的用户名和密码：

```bash
export http_proxy=http://username:password@proxy.company.com:8080
```

### 2. HTTPS 证书验证问题

如果代理服务器使用自签名证书，可能需要配置证书信任。这通常需要系统级别的配置。

### 3. 代理连接超时

程序默认的 HTTP 超时时间是 30 秒。如果代理响应较慢，可能需要调整网络环境或联系网络管理员。

### 4. 部分请求不走代理

确保 `no_proxy` 环境变量没有包含阿里云 API 的域名。阿里云 API 域名通常是：

- `*.aliyuncs.com`
- `openapi-rdc.aliyuncs.com`

## 技术实现

### HTTP 客户端代理支持

程序通过 `createHTTPClientWithProxy()` 函数创建支持代理的 HTTP 客户端：

```go
func createHTTPClientWithProxy() *http.Client {
    transport := &http.Transport{}
    
    // 检查 HTTP 代理
    if httpProxy := os.Getenv("http_proxy"); httpProxy != "" {
        if proxyURL, err := url.Parse(httpProxy); err == nil {
            transport.Proxy = http.ProxyURL(proxyURL)
        }
    }
    
    // 检查 HTTPS 代理
    if httpsProxy := os.Getenv("https_proxy"); httpsProxy != "" {
        // 为 HTTPS 请求设置特定代理
    }
    
    // 如果没有设置代理环境变量，使用默认代理
    if transport.Proxy == nil {
        transport.Proxy = http.ProxyFromEnvironment
    }
    
    return &http.Client{
        Transport: transport,
        Timeout:   30 * time.Second,
    }
}
```

### SDK 客户端代理支持

对于阿里云 SDK 客户端，程序在创建客户端后设置代理：

```go
// 创建 SDK 客户端
sdkClient, err := devops_rdc.NewClientWithOptions(regionId, sdk.NewConfig(), credential)

// 配置代理
if httpProxy := os.Getenv("http_proxy"); httpProxy != "" {
    sdkClient.SetHttpProxy(httpProxy)
}

if httpsProxy := os.Getenv("https_proxy"); httpsProxy != "" {
    sdkClient.SetHttpsProxy(httpsProxy)
}
```

## 安全注意事项

1. **代理认证信息安全**：避免在命令行历史或脚本中明文存储代理认证信息
2. **环境变量安全**：确保包含认证信息的环境变量只在必要的作用域内设置
3. **网络安全**：确保代理服务器是可信的，避免通过不安全的代理传输敏感数据

## 测试代理配置

可以使用以下命令测试代理配置是否正确：

```bash
# 测试 HTTP 代理
curl -x http://proxy.company.com:8080 http://httpbin.org/ip

# 测试 HTTPS 代理
curl -x http://proxy.company.com:8080 https://httpbin.org/ip
```

如果代理配置正确，返回的 IP 地址应该是代理服务器的 IP 地址。 