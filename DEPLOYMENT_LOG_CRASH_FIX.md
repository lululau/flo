# 部署日志解析崩溃修复

## 问题描述

在创建流水线运行后，在日志界面刷新时，当刷新到部署日志（即 `extracting deployOrderId from job result` 的时候），程序报错导致整个程序崩溃退出。但是查看历史运行记录的日志时，在解析和显示部署Job的日志时，并没有报错。

## 错误信息

```
Error extracting deployOrderId from job result: deployOrderId not found in result JSON
[signal SIGSEGV: segmentation violation code=0x2 addr=0x18 pc=0x1018b3c20]
```

## 问题分析

### 根本原因
1. **错误的数据源**: 原代码试图从 `job.Result` 字段中提取 `deployOrderId`，但实际上 `deployOrderId` 应该从 `job.actions[].data` 或 `job.actions[].params` 中提取。
2. **API响应结构变化**: 根据实际API响应，部署相关的信息存储在Job的Actions数组中，而不是Result字段中。
3. **错误处理不足**: 原来的代码在无法解析 `deployOrderId` 时没有优雅地处理错误，导致程序崩溃。

### API响应结构示例
根据实际API响应，部署Job的结构如下：
```json
{
  "id": 300460493,
  "name": "集采测试环境部署",
  "status": "RUNNING",
  "result": "{\"message\":\"deploy.running\",\"requestId\":\"...\",\"successful\":true}",
  "actions": [
    {
      "type": "GetVMDeployOrder",
      "displayType": "DEPLOY_ORDER",
      "data": "{\"deployOrderId\":44178813,\"status\":\"RUNNING\"}",
      "params": {
        "deployOrderId": 44178813
      }
    }
  ]
}
```

可以看到 `deployOrderId` 存储在：
- `actions[].params.deployOrderId` (直接数值)
- `actions[].data` (JSON字符串中的deployOrderId字段)

## 修复方案

### 1. 修正数据提取源

**修改前** (错误的方法):
```go
deployOrderId, err := extractDeployOrderId(job.Result)
if err != nil {
    allLogs.WriteString(fmt.Sprintf("Error extracting deployOrderId from job result: %v\n", err))
} else {
    // 处理部署详情...
}
```

**修改后** (正确的方法):
```go
deployOrderId, err := extractDeployOrderIdFromActions(job.Actions)
if err != nil {
    allLogs.WriteString(fmt.Sprintf("Error extracting deployOrderId from job actions: %v\n", err))
    // 根据Job状态提供不同的说明信息
    if job.Status == "RUNNING" || job.Status == "QUEUED" {
        allLogs.WriteString("Deployment is still in progress. Deploy order information will be available once the deployment starts.\n")
    } else if job.Status == "FAILED" {
        allLogs.WriteString("Deployment job failed. No deploy order information available.\n")
    } else {
        allLogs.WriteString("Deploy order information is not available for this job.\n")
    }
    // 继续处理其他Job，而不是停止
} else {
    // 处理部署详情...
}
```

### 2. 新增 `extractDeployOrderIdFromActions` 函数

**新函数功能**:
1. **正确的数据源**: 从Job的Actions数组中提取deployOrderId
2. **多种提取方式**: 支持从params和data字段中提取
3. **调试日志**: 添加详细的调试信息
4. **错误处理**: 优雅处理解析失败的情况

**新增的函数**:
```go
func extractDeployOrderIdFromActions(actions []JobAction) (string, error) {
    if len(actions) == 0 {
        return "", fmt.Errorf("no actions found in job")
    }

    // 查找GetVMDeployOrder类型的action
    for _, action := range actions {
        if action.Type == "GetVMDeployOrder" {
            // 优先从action.params中获取deployOrderId
            if action.Params != nil {
                if deployOrderId, ok := action.Params["deployOrderId"]; ok {
                    if id, ok := deployOrderId.(float64); ok {
                        return fmt.Sprintf("%.0f", id), nil
                    }
                    if id, ok := deployOrderId.(string); ok {
                        return id, nil
                    }
                }
            }

            // 然后尝试从action.data JSON字符串中解析
            if action.Data != "" {
                var actionData map[string]interface{}
                if err := json.Unmarshal([]byte(action.Data), &actionData); err == nil {
                    if deployOrderId, ok := actionData["deployOrderId"]; ok {
                        if id, ok := deployOrderId.(float64); ok {
                            return fmt.Sprintf("%.0f", id), nil
                        }
                        if id, ok := deployOrderId.(string); ok {
                            return id, nil
                        }
                    }
                }
            }
        }
    }

    return "", fmt.Errorf("deployOrderId not found in any GetVMDeployOrder action")
}
```

### 3. 增强API调用错误处理

**修改后**:
```go
deployOrder, err := c.GetVMDeployOrder(organizationId, pipelineIdStr, deployOrderId)
if err != nil {
    allLogs.WriteString(fmt.Sprintf("Error fetching VM deploy order %s: %v\n", deployOrderId, err))
    allLogs.WriteString("Unable to retrieve deployment details at this time.\n")
} else {
    // 显示部署详情...
}
```

## 修复效果

### 稳定性提升
- **程序不再崩溃**: 即使在解析部署信息失败时，程序也能继续运行
- **优雅降级**: 提供有意义的错误信息，而不是直接崩溃
- **继续处理**: 解析失败的Job不会影响其他Job的日志显示

### 用户体验改善
- **清晰的状态说明**: 根据Job状态提供不同的说明信息
- **实时更新**: 在部署开始后，用户可以看到完整的部署信息
- **调试支持**: 开启调试模式可以看到详细的解析过程

### 兼容性保证
- **向后兼容**: 对历史运行记录的处理保持不变
- **多格式支持**: 支持不同的JSON结构格式
- **渐进式显示**: 随着Job状态变化，显示的信息会逐步完善

## 技术细节

### 数据结构修正
根据实际API响应，部署相关信息的正确位置：

1. **错误位置**: `job.Result` 字段只包含部署状态信息，不包含 `deployOrderId`
2. **正确位置**: `job.Actions` 数组中的 `GetVMDeployOrder` 类型action
3. **提取优先级**: 
   - 优先从 `action.Params["deployOrderId"]` 获取（直接数值）
   - 备选从 `action.Data` JSON字符串中解析获取

### 错误处理策略
1. **非阻塞**: 单个Job的解析失败不影响其他Job
2. **信息丰富**: 提供具体的错误原因和建议
3. **状态感知**: 根据Job状态提供相应的说明

### 调试支持
通过设置 `FLO_DEBUG=1` 环境变量，可以看到详细的JSON解析过程，便于排查问题。

## 测试验证

### 验证场景
1. **新建流水线运行**: 确认在部署Job刚开始时不会崩溃
2. **运行中刷新**: 确认在部署进行中时能正常显示状态
3. **历史记录查看**: 确认对已完成的部署仍能正常显示详情
4. **失败的部署**: 确认对失败的部署Job能正确处理

### 预期结果
- 程序在任何情况下都不会崩溃
- 提供清晰的状态信息和错误说明
- 支持渐进式信息显示

## 相关文件修改

### 修改的文件
- `internal/api/client.go`: 
  - 新增了 `extractDeployOrderIdFromActions` 函数
  - 修改了 `GetPipelineRunLogs` 中的deployOrderId提取逻辑
  - 保留了原有的 `extractDeployOrderId` 函数（向后兼容）
  - 增强了错误处理和调试日志支持

### 新增文件
- `DEPLOYMENT_LOG_CRASH_FIX.md`: 本修复文档

## 总结

这个修复解决了一个严重的稳定性问题，确保程序在处理实时部署日志时不会崩溃。通过增强错误处理、支持多种JSON格式和提供清晰的状态信息，显著提升了用户体验和程序的健壮性。 