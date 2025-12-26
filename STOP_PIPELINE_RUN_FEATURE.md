# 终止流水线运行功能

## 功能概述

为 flo 添加了终止流水线运行的功能，用户可以在运行历史表格或日志界面中使用大写 `X` 键来终止正在运行的流水线。

## 功能特性

### 1. 运行历史表格中的终止功能

**按键**: `X` (大写)

**功能描述**:
- 在运行历史表格中选择任意运行记录，按 `X` 键可以终止该运行
- 支持终止任何状态的运行（包括已完成的运行，但 API 可能会返回错误）
- 显示确认对话框，防止误操作
- 终止成功后自动刷新运行历史表格

**使用流程**:
1. 在流水线列表中按 `Enter` 进入运行历史
2. 使用 `j/k` 键选择要终止的运行
3. 按 `X` 键，系统显示确认对话框
4. 选择 "Yes" 确认终止，或 "No" 取消操作
5. 终止成功后显示成功消息并刷新表格

### 2. 日志界面中的终止功能

**按键**: `X` (大写)

**功能描述**:
- 在日志界面中按 `X` 键可以终止当前正在查看的运行
- 只对特定状态的运行有效：`RUNNING`、`INIT`、`WAITING`、`QUEUED`
- 对于已完成的运行，会显示提示信息说明无法终止
- 终止成功后自动停止日志自动刷新

**状态检查**:
- **可终止状态**: RUNNING, INIT, WAITING, QUEUED
- **不可终止状态**: SUCCESS, FAILED, CANCELED 等已完成状态
- 对于不可终止的状态，显示友好的提示信息

**使用流程**:
1. 在日志界面查看正在运行的流水线
2. 按 `X` 键，系统检查当前运行状态
3. 如果状态允许终止，显示确认对话框
4. 如果状态不允许终止，显示说明信息
5. 确认终止后，停止自动刷新并更新状态为 "STOPPING"

## API 实现

### 新增 API 方法

#### `StopPipelineRun(organizationId, pipelineId, runId string) error`
- **功能**: 终止指定的流水线运行
- **认证**: 支持个人访问令牌认证（推荐）
- **API 端点**: `PUT https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/runs/{pipelineRunId}`
- **返回值**: 布尔值表示是否成功

#### `stopPipelineRunWithToken(organizationId, pipelineId, runId string) error`
- **功能**: 使用个人访问令牌终止流水线运行
- **请求头**: `x-yunxiao-token: {personal_access_token}`
- **响应处理**: 支持布尔值和字符串 "true"/"false" 两种响应格式

### API 文档参考

基于阿里云官方 API 文档实现：
- **文档链接**: https://help.aliyun.com/zh/yunxiao/developer-reference/updatepipelinerun
- **请求方法**: PUT
- **认证方式**: 个人访问令牌（x-yunxiao-token 头）
- **响应格式**: 布尔值表示操作是否成功

## 用户界面更新

### 帮助文本更新

#### 运行历史页面
**更新前**:
```
Keys: j/k=move, Enter=view logs, r=run pipeline, [/]=prev/next page, 0=first page, q=back to pipelines, Q=quit
```

**更新后**:
```
Keys: j/k=move, Enter=view logs, r=run pipeline, X=stop run, [/]=prev/next page, 0=first page, q=back to pipelines, Q=quit
```

#### 日志界面状态栏
**更新前**:
```
Press 'r' to refresh, 'q' to return, 'e' to edit, 'v' to view in pager
```

**更新后**:
```
Press 'r' to refresh, 'X' to stop run, 'q' to return, 'e' to edit, 'v' to view in pager
```

### 确认对话框

#### 运行历史中的确认对话框
```
标题: Confirm Stop
内容: Are you sure you want to stop pipeline run #[RunID]?
      Status: [CurrentStatus]
按钮: [Yes] [No]
```

#### 日志界面中的确认对话框
```
标题: Confirm Stop
内容: Are you sure you want to stop the current pipeline run?
      Run ID: [RunID]
      Status: [CurrentStatus]
按钮: [Yes] [No]
```

#### 状态不允许终止时的提示
```
标题: Cannot Stop
内容: Pipeline run cannot be stopped.
      Current status: [CurrentStatus]
      
      Only runs with status RUNNING, INIT, WAITING, or QUEUED can be stopped.
按钮: [OK]
```

## 错误处理

### API 错误处理
- **网络错误**: 显示网络连接失败的错误信息
- **认证错误**: 显示认证失败的错误信息
- **权限错误**: 显示权限不足的错误信息
- **API 返回 false**: 显示 "API returned false" 的错误信息

### 用户体验优化
- **异步操作**: 所有 API 调用都在后台执行，不阻塞 UI
- **即时反馈**: 操作成功或失败都有明确的提示信息
- **状态更新**: 终止成功后立即更新相关状态和显示

## 技术实现细节

### 代码结构

#### API 客户端 (`internal/api/client.go`)
```go
// 主要接口
func (c *Client) StopPipelineRun(organizationId, pipelineId, runId string) error

// 令牌认证实现
func (c *Client) stopPipelineRunWithToken(organizationId, pipelineId, runId string) error
```

#### UI 组件 (`internal/ui/components.go`)
```go
// 运行历史表格事件处理
case 'X': // 在 runHistoryTable.SetInputCapture 中

// 日志界面事件处理  
case 'X': // 在 logViewTextView.SetInputCapture 中
```

### 状态管理
- **currentRunStatus**: 跟踪当前运行状态
- **currentRunID**: 跟踪当前运行 ID
- **currentPipelineIDForRun**: 跟踪当前流水线 ID

### 并发安全
- 使用 `app.QueueUpdateDraw()` 确保 UI 更新的线程安全
- API 调用在独立的 goroutine 中执行
- 适当的错误处理和资源清理

## 使用示例

### 场景 1: 终止运行历史中的流水线
1. 启动 flo 应用
2. 选择一个流水线，按 `Enter` 进入运行历史
3. 选择一个正在运行的记录，按 `X`
4. 在确认对话框中选择 "Yes"
5. 查看成功消息，运行历史表格自动刷新

### 场景 2: 在日志界面终止当前运行
1. 创建一个新的流水线运行
2. 在日志界面观察运行进度
3. 按 `X` 键终止运行
4. 确认终止操作
5. 观察自动刷新停止，状态更新为 "STOPPING"

### 场景 3: 尝试终止已完成的运行
1. 在日志界面查看已完成的运行
2. 按 `X` 键
3. 系统显示 "Cannot Stop" 提示信息
4. 了解只有特定状态的运行可以被终止

## 兼容性和限制

### 兼容性
- **认证方式**: 目前只支持个人访问令牌认证
- **API 版本**: 基于阿里云云效最新 API 实现
- **向后兼容**: 不影响现有功能，完全向后兼容

### 限制
- **AccessKey 认证**: 暂未实现 AccessKey 认证方式的终止功能
- **批量操作**: 目前只支持单个运行的终止，不支持批量终止
- **状态限制**: 只能终止特定状态的运行

### 未来改进
- 添加 AccessKey 认证方式的支持
- 支持批量终止多个运行
- 添加终止原因的输入功能
- 支持更多的运行状态检查

## 测试建议

### 功能测试
1. **正常终止**: 测试终止正在运行的流水线
2. **状态检查**: 测试不同状态下的终止行为
3. **错误处理**: 测试网络错误、权限错误等场景
4. **UI 响应**: 测试确认对话框和提示信息的显示

### 集成测试
1. **运行历史集成**: 测试从运行历史终止的完整流程
2. **日志界面集成**: 测试从日志界面终止的完整流程
3. **状态同步**: 测试终止后状态更新的正确性
4. **自动刷新**: 测试终止后自动刷新的停止

### 边界测试
1. **无效 ID**: 测试使用无效的运行 ID
2. **权限不足**: 测试权限不足的场景
3. **网络异常**: 测试网络连接异常的处理
4. **并发操作**: 测试同时进行多个终止操作

## 总结

终止流水线运行功能为 flo 提供了重要的运行控制能力，让用户能够：

- **及时止损**: 快速终止有问题的流水线运行
- **资源管理**: 释放不必要占用的计算资源
- **操作便利**: 在查看运行历史或日志时直接进行终止操作
- **安全可靠**: 通过确认对话框防止误操作

该功能的实现遵循了 flo 的设计原则：
- **用户友好**: 清晰的提示信息和确认机制
- **功能完整**: 支持多种使用场景和状态检查
- **技术可靠**: 适当的错误处理和并发安全
- **向后兼容**: 不影响现有功能的正常使用

## 问题修复记录

### 焦点恢复问题修复 (2024-12-19)

**问题描述**: 
在按 'X' 键出现确认弹出框之后，不管是按 "Yes" 还是 "No"，待弹出框关闭之后，按任何键界面都没有响应。

**问题原因**: 
`HideModal` 函数中的焦点恢复逻辑不完整，只考虑了流水线表格和组表格的情况，没有处理运行历史表格和日志视图的焦点恢复。

**修复方案**: 
更新 `HideModal` 函数，添加基于当前活动视图的智能焦点恢复：

```go
func HideModal() {
    // ... 移除模态框 ...
    
    // 根据当前活动视图恢复焦点
    if isLogViewActive && logViewTextView != nil {
        // 如果日志视图活跃，恢复焦点到日志视图
        appGlobal.SetFocus(logViewTextView)
    } else if isRunHistoryActive && runHistoryTable != nil {
        // 如果运行历史活跃，恢复焦点到运行历史表格
        appGlobal.SetFocus(runHistoryTable)
    } else if pipelineTableGlobal != nil && (currentViewMode == "all_pipelines" || currentViewMode == "pipelines_in_group") {
        // 默认恢复到流水线表格
        appGlobal.SetFocus(pipelineTableGlobal)
    } else if currentViewMode == "group_list" && groupTableGlobal != nil {
        // 默认恢复到组表格
        appGlobal.SetFocus(groupTableGlobal)
    }
}
```

**修复效果**: 
- 在运行历史页面按 'X' 键后，确认弹出框关闭时焦点正确恢复到运行历史表格
- 在日志页面按 'X' 键后，确认弹出框关闭时焦点正确恢复到日志视图
- 保持其他页面的焦点恢复功能不变
- 用户可以正常继续操作界面，不会出现无响应的情况 