# 运行历史分页功能使用指南

## 功能概述

运行历史表格现在支持分页显示，每页显示10条记录，并提供便捷的键盘导航功能。

## 分页快捷键

在运行历史页面中，您可以使用以下快捷键进行分页导航：

- **`[`** - 上一页
- **`]`** - 下一页  
- **`0`** - 回到第一页
- **`j/k`** - 在当前页面内上下移动
- **`Enter`** - 查看选中运行的日志
- **`Esc`** - 返回流水线列表

## 页面信息显示

运行历史表格的标题栏会显示当前分页信息：
```
Run History - 流水线名称 (Page 2/5) [/] to navigate, 0 to go to first page
```

- `Page 2/5` 表示当前在第2页，总共5页
- `[/] to navigate` 提示使用方括号键进行导航
- `0 to go to first page` 提示按0键回到首页

## 使用流程

1. 在流水线列表中，按 **`Enter`** 进入某个流水线的运行历史
2. 运行历史会自动从第1页开始显示
3. 使用 **`[`** 和 **`]`** 键在不同页面间导航
4. 使用 **`j/k`** 键在当前页面内选择不同的运行记录
5. 按 **`Enter`** 查看选中运行的详细日志
6. 按 **`0`** 可以快速回到第一页

## 技术实现

- 每页显示10条运行记录
- 运行编号按时间倒序排列（最新的运行显示为#1）
- 分页计算会自动处理总页数和当前页面的有效性
- 切换页面时会自动选中第一条记录

## API支持

分页功能基于以下API实现：
- 使用 `ListPipelineRuns` API 获取所有运行记录
- 在客户端进行分页处理，确保响应速度
- 支持阿里云DevOps官方API的分页参数

## 故障排除

如果运行历史显示为空：
1. 检查流水线是否有执行记录
2. 确认API认证配置正确
3. 开启调试模式查看详细错误信息：
   ```bash
   export FLOWT_DEBUG=1
   ./flo
   ```

## 最新修复

### v1.1.0 - 修复运行历史API调用
- **问题**: 运行历史表格显示"Error fetching runs: all pipeline endpoints failed, Last error: API request failed with status 404"
- **原因**: 使用了错误的API端点
- **修复**: 更新为正确的阿里云DevOps API端点
  - 正确端点: `GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/runs`
  - 参数: `page`, `perPage` (最大30)
  - 响应: 直接返回数组格式
- **字段映射更新**:
  - `pipelineRunId` → RunID
  - `endTime` → FinishTime (不是finishTime)
  - `triggerMode` 整数映射为字符串 (1=MANUAL, 2=SCHEDULE, 3=PUSH, 5=PIPELINE, 6=WEBHOOK)
- **分页支持**: 使用响应头 `x-page`, `x-total-pages`, `x-per-page` 进行分页控制

## 未来改进

计划中的功能增强：
- 支持服务端分页以提高大数据量的性能
- 添加运行状态过滤功能
- 支持按时间范围筛选运行记录
- 添加运行记录搜索功能 