# 状态筛选和运行历史增强功能

## 概述

为 flo 添加了两个重要的新功能，提升用户体验和操作便利性：

1. 主界面支持按 `a` 键在全部流水线和 RUNNING+WAITING 流水线之间切换
2. 运行历史表格界面支持按 `r` 键运行流水线

## 新增功能

### 1. 状态筛选功能

**按键**: `a`

**功能描述**:
- 在主界面流水线列表中按 `a` 键可以在两种显示模式之间切换：
  - **全部流水线**: 显示所有流水线
  - **RUNNING+WAITING**: 仅显示状态为 RUNNING 或 WAITING 的流水线

**界面变化**:
- 标题栏会显示当前筛选状态：
  - `All Pipelines` - 显示所有流水线
  - `Pipelines (RUNNING+WAITING)` - 显示筛选后的流水线
  - `Pipelines in 'GroupName' (RUNNING+WAITING)` - 在组内显示筛选后的流水线

**使用场景**:
- 快速查看当前正在运行或等待运行的流水线
- 在大量流水线中快速定位活跃的流水线
- 监控当前系统的运行状态

### 2. 运行历史中运行流水线功能

**按键**: `r`

**功能描述**:
- 在运行历史表格界面按 `r` 键可以直接运行当前查看的流水线
- 功能与主界面的运行流水线功能完全一致
- 会弹出分支选择对话框，允许用户指定运行参数

**使用场景**:
- 查看历史运行记录时，发现需要重新运行流水线
- 基于历史运行的分支信息快速启动新的运行
- 减少界面切换，提高操作效率

## 技术实现

### API 层面修改

#### 新增 API 方法

```go
// ListPipelinesWithStatus lists pipelines with optional status filtering
func (c *Client) ListPipelinesWithStatus(organizationId string, statusList []string) ([]Pipeline, error)

// listPipelinesWithTokenAndStatus - 内部实现，支持状态筛选的 Token 认证方法
func (c *Client) listPipelinesWithTokenAndStatus(organizationId string, statusList []string) ([]Pipeline, error)
```

#### API 调用参数

根据阿里云官方 API 文档，状态筛选通过 `statusList` 查询参数实现：

```
GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines?statusList=RUNNING,WAITING
```

支持的状态值：
- `RUNNING` - 运行中
- `WAITING` - 等待中  
- `SUCCESS` - 成功
- `FAILED` - 失败
- `CANCELED` - 已取消

### UI 层面修改

#### 新增全局变量

```go
// Status filtering
showOnlyRunningWaiting bool // Toggle between all pipelines and RUNNING+WAITING only
```

#### 修改的函数

1. **`updatePipelineTable()`**:
   - 根据 `showOnlyRunningWaiting` 状态调用不同的 API
   - 更新标题显示当前筛选状态
   - 处理 API 调用错误

2. **流水线表格事件处理**:
   - 添加 `a` 键处理逻辑
   - 切换筛选状态并刷新表格

3. **运行历史表格事件处理**:
   - 添加 `r` 键处理逻辑
   - 查找当前流水线对象并调用运行对话框

#### 帮助文本更新

**主界面**:
```
Keys: j/k=move, Enter=run history, r=run, a=toggle filter, Ctrl+G=groups, /=search, q=back, Q=quit
```

**运行历史界面**:
```
Keys: j/k=move, Enter=view logs, r=run pipeline, [/]=prev/next page, 0=first page, q=back to pipelines, Q=quit
```

## 用户体验改进

### 状态筛选功能

1. **快速切换**: 一键切换显示模式，无需复杂的筛选操作
2. **视觉反馈**: 标题栏清晰显示当前筛选状态
3. **保持上下文**: 在组内查看时也支持状态筛选
4. **实时更新**: 切换时立即从服务器获取最新数据

### 运行历史增强

1. **操作便利**: 无需返回主界面即可运行流水线
2. **上下文保持**: 在查看历史的同时可以启动新运行
3. **一致体验**: 与主界面运行功能完全一致的用户体验
4. **智能查找**: 自动查找当前流水线对象，无需用户手动选择

## 实现细节

### 状态筛选逻辑

```go
if showOnlyRunningWaiting {
    // Fetch pipelines with status filter
    statusList := []string{"RUNNING", "WAITING"}
    filteredPipelines, err := apiClient.ListPipelinesWithStatus(orgId, statusList)
    // ... handle results
} else {
    // Use cached all pipelines
    tempFilteredByGroup = append(tempFilteredByGroup, allPipelines...)
}
```

### 运行历史中的流水线查找

```go
case 'r': // Run pipeline
    // Find the pipeline object for the current pipeline
    var selectedPipeline *api.Pipeline
    for _, p := range allPipelines {
        if p.PipelineID == currentPipelineIDForRun {
            selectedPipeline = &p
            break
        }
    }
    if selectedPipeline != nil {
        showRunPipelineDialog(selectedPipeline, app, apiClient, orgId)
    }
```

## 兼容性

- **向后兼容**: 完全兼容现有功能，不影响原有操作
- **API 兼容**: 基于官方 API 文档实现，确保稳定性
- **界面兼容**: 保持原有界面布局和导航逻辑
- **快捷键兼容**: 新增快捷键不与现有快捷键冲突

## 测试建议

### 状态筛选测试

1. **基本切换测试**:
   - 在主界面按 `a` 键
   - 验证标题变化和流水线列表更新
   - 确认只显示 RUNNING 和 WAITING 状态的流水线

2. **组内筛选测试**:
   - 进入流水线组
   - 按 `a` 键切换筛选
   - 验证组内筛选功能正常

3. **搜索结合测试**:
   - 启用状态筛选
   - 使用搜索功能
   - 确认搜索在筛选结果中正常工作

### 运行历史增强测试

1. **基本运行测试**:
   - 进入任意流水线的运行历史
   - 按 `r` 键
   - 验证弹出分支选择对话框

2. **流水线查找测试**:
   - 测试不同流水线的运行历史
   - 确认能正确找到对应的流水线对象
   - 验证运行参数正确传递

3. **界面导航测试**:
   - 从运行历史启动流水线后
   - 验证能正确跳转到日志界面
   - 确认返回导航正常

## 总结

这两个新功能显著提升了 flo 的用户体验：

- **状态筛选**: 提供了快速查看活跃流水线的能力，特别适合监控和运维场景
- **运行历史增强**: 减少了界面切换，提高了操作效率，特别适合基于历史记录的重复运行场景

所有功能都经过精心设计，确保与现有功能完美集成，不影响用户的现有工作流程。 