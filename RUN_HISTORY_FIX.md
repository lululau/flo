# 运行历史功能修复说明

## 问题描述

之前运行历史表格显示错误：
```
Error fetching runs: all pipeline endpoints failed, Last error: API request failed with status 404: "errorCode": "NotFound", "errorMessage": "Not Found"
```

## 修复内容

### 1. API端点修复
- **修复前**: 使用了错误的API端点，导致404错误
- **修复后**: 使用正确的阿里云DevOps API端点
  ```
  GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/runs
  ```

### 2. 字段映射更新
根据官方API文档更新了字段映射：
- `pipelineRunId` → `RunID` (运行实例ID)
- `endTime` → `FinishTime` (结束时间，注意不是finishTime)
- `triggerMode` 整数映射为字符串：
  - 1 = "MANUAL" (手动触发)
  - 2 = "SCHEDULE" (定时触发)
  - 3 = "PUSH" (代码提交触发)
  - 5 = "PIPELINE" (流水线触发)
  - 6 = "WEBHOOK" (Webhook触发)

### 3. 分页功能增强
- 支持使用 `[` 和 `]` 键进行翻页
- 按 `0` 键回到第一页
- 每页显示10条记录
- 标题栏显示分页信息：`(Page 2/5) [/] to navigate, 0 to go to first page`

## 使用方法

### 1. 配置文件
确保 `~/.flo/config.yml` 配置正确：
```yaml
auth_method: "personal_access_token"
personal_access_token: "your_token_here"
endpoint: "openapi-rdc.aliyuncs.com"
organization_id: "your_org_id_here"
debug: false
```

### 2. 运行应用
```bash
./flo
```

### 3. 查看运行历史
1. 在流水线列表中选择一个流水线
2. 按 `Enter` 键进入运行历史
3. 使用以下快捷键：
   - `j/k` - 上下移动
   - `[/]` - 上一页/下一页
   - `0` - 回到第一页
   - `Enter` - 查看日志
   - `Esc` - 返回流水线列表

## 调试模式

如果遇到问题，可以开启调试模式：
```bash
export FLOWT_DEBUG=1
./flo
```

调试信息会保存到 `logs/api_debug.log` 文件中。

## 测试API连接

可以使用测试脚本验证API连接：
```bash
./test_api
```

这会测试：
- 流水线列表获取
- 运行历史获取
- 显示前5条运行记录

## 技术细节

### API文档参考
- [ListPipelineRuns API文档](https://help.aliyun.com/zh/yunxiao/developer-reference/listpipelineruns)
- 请求方法: GET
- 认证方式: x-yunxiao-token 头部
- 分页参数: page, perPage (最大30)
- 响应格式: 直接数组

### 响应头分页信息
- `x-page`: 当前页
- `x-total-pages`: 总页数
- `x-per-page`: 每页数据条数
- `x-total`: 总数据量

## 已知限制

1. 每页最多显示30条记录（API限制）
2. 日志查看功能需要JobID，目前尚未完全实现
3. 某些流水线可能没有运行历史记录

## 后续改进计划

1. 实现完整的日志查看功能
2. 添加运行状态过滤
3. 支持按时间范围筛选
4. 添加运行记录搜索功能 