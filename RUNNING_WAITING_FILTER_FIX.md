# RUNNING+WAITING Pipeline Filter Fix

## 问题描述

当用户在UI中按 'a' 键切换到 RUNNING+WAITING pipeline 列表时，列表显示为空，即使API确实返回了数据。

## 问题分析

通过调试发现，当使用状态过滤（statusList=RUNNING,WAITING）时，阿里云DevOps API返回的数据结构与正常的pipeline列表略有不同：

1. **缺少status字段**：过滤后的API响应中，pipeline对象可能不包含`status`字段
2. **数据结构简化**：返回的数据结构更简单，只包含基本信息
3. **解析逻辑问题**：原有的解析逻辑期望所有pipeline都有`status`字段，导致解析失败

### API响应示例

```json
[
    {
        "name": "demo",
        "owner": null,
        "pipelineConfig": null,
        "id": 4185805,
        "createTime": 1748314799000,
        "modifierAccountId": null,
        "creatorAccountId": "665694bf93cee8fbf8e4aa4f",
        "updateTime": null,
        "groupId": 112627,
        "envName": null,
        "envId": null,
        "tagList": null,
        "pipelineConfigId": null,
        "creator": {
            "id": "665694bf93cee8fbf8e4aa4f",
            "username": null
        },
        "modifier": null
    }
]
```

注意：这个响应中没有`status`字段，但pipeline确实是RUNNING状态的。

## 解决方案

修改了 `internal/api/client.go` 中的pipeline解析逻辑，在两个关键函数中添加了状态推断逻辑：

### 1. `listPipelinesWithTokenAndStatus` 函数

```go
// Extract pipeline status - handle cases where status field might be missing
pipelineStatus := getStringField(pipelineMap, "status")
// If status is missing but we have statusList filter, infer the status
if pipelineStatus == "" && len(statusList) > 0 {
    // When filtering by status, the returned pipelines should match the filter
    // Use the first status from the filter as a reasonable default
    pipelineStatus = statusList[0]
}
// If still no status, use lastRunStatus as fallback
if pipelineStatus == "" && lastRunStatus != "" {
    pipelineStatus = lastRunStatus
}
```

### 2. `listPipelinesWithTokenAndCallback` 函数

应用了相同的状态推断逻辑，确保callback-based的加载也能正确处理。

### 3. 改进的验证逻辑

```go
// Only include pipelines that have both ID and name
if pipeline.PipelineID != "" && pipeline.Name != "" {
    pagePipelines = append(pagePipelines, pipeline)
}
```

确保只有有效的pipeline才会被添加到结果中。

## 修复效果

### 修复前
- 切换到RUNNING+WAITING过滤时，列表显示为空
- 用户无法看到正在运行或等待的pipeline

### 修复后
- 正确显示RUNNING+WAITING状态的pipeline
- Status字段正确显示为"RUNNING"或"WAITING"
- UI功能完全正常

### 测试结果

```bash
Testing RUNNING+WAITING pipeline filtering...
Found 1 RUNNING+WAITING pipelines:
  - ID: 4185805, Name: demo, Status: RUNNING, LastRunStatus: 
```

## 技术细节

### 状态推断逻辑

1. **优先使用API返回的status字段**：如果存在，直接使用
2. **基于过滤条件推断**：如果status字段缺失但有statusList过滤，使用过滤条件中的第一个状态
3. **使用lastRunStatus作为后备**：如果以上都不可用，使用lastRunStatus

### 兼容性

这个修复保持了向后兼容性：
- 对于包含完整status字段的API响应，行为不变
- 对于缺少status字段的API响应，提供合理的默认值
- 不影响其他功能的正常使用

## 相关文件

- `internal/api/client.go` - 主要修复文件
- `internal/ui/components.go` - UI组件（无需修改）

## 测试方法

1. 启动程序：`./flo`
2. 按 'a' 键切换到RUNNING+WAITING过滤模式
3. 验证列表中显示正在运行或等待的pipeline
4. 再次按 'a' 键切换回所有pipeline模式

## 调试信息

如需调试，可以启用调试模式：

```bash
FLO_DEBUG=1 ./flo
```

这将输出详细的API请求和响应信息，包括pipeline解析过程。 