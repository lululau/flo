# 编辑器和分页器功能

## 概述

flo 现在支持在日志显示界面通过按键 `e` 和 `v` 使用配置的编辑器和分页器打开日志文件。这个功能参考了 tali 项目的实现，提供了更好的日志查看和编辑体验。

## 新增功能

### 按键绑定

在日志显示界面中：
- **`e` 键**: 使用配置的编辑器打开当前日志内容
- **`v` 键**: 使用配置的分页器查看当前日志内容
- **`q` 键**: 返回到上一个界面（保持原有功能）

### 配置优先级

#### 编辑器选择优先级
1. 配置文件中的 `editor` 字段
2. `VISUAL` 环境变量
3. `EDITOR` 环境变量  
4. 默认使用 `vim`

#### 分页器选择优先级
1. 配置文件中的 `pager` 字段
2. `PAGER` 环境变量
3. 默认使用 `less`

## 配置示例

### 在配置文件中设置

在 `~/.flo/config.yml` 中添加 `editor` 和 `pager` 字段：

```yaml
# 企业 ID（组织 ID）- 必填
organization_id: "your_organization_id"

# 个人访问令牌 - 推荐使用
personal_access_token: "your_personal_access_token"

# 编辑器配置
editor: "code --wait"

# 分页器配置
pager: "less -R"
```

### 通过环境变量设置

```bash
# 设置编辑器
export VISUAL="code"
export EDITOR="vim"

# 设置分页器
export PAGER="less -R"
```

## 支持的编辑器示例

- `vim` - 默认编辑器
- `nvim` - Neovim
- `code` - Visual Studio Code
- `code --wait` - VS Code 等待模式
- `nano` - Nano编辑器
- `emacs` - Emacs编辑器
- `subl` - Sublime Text

## 支持的分页器示例

- `less` - 默认分页器
- `less -R` - 支持颜色的less
- `more` - 简单分页器
- `cat` - 直接输出（不分页）

## 使用方法

1. 在流水线列表中选择一个流水线，按 `Enter` 进入运行历史
2. 在运行历史中选择一个运行记录，按 `Enter` 查看日志
3. 在日志显示界面中：
   - 按 `e` 键使用编辑器打开日志进行编辑
   - 按 `v` 键使用分页器查看日志
   - 按 `q` 键返回到运行历史

## 技术实现

### 临时文件处理
- 日志内容会被写入临时文件 `flo_logs_<timestamp>.txt`
- 临时文件位于系统临时目录中
- 编辑器或分页器关闭后，临时文件会被自动删除

### 应用挂起和恢复
- 使用 `app.Suspend()` 方法挂起 tview 应用
- 释放终端控制权给编辑器或分页器
- 编辑器或分页器退出后自动恢复 tview 应用
- 使用 `reset` 命令重置终端状态以避免显示问题

### 命令解析
- 支持带参数的命令（如 `"less -R"` 或 `"code --wait"`）
- 使用 `strings.Fields()` 解析命令和参数
- 临时文件路径作为最后一个参数传递给命令

## 注意事项

- 确保配置的编辑器和分页器程序已安装在系统中
- 编辑器和分页器命令支持参数，如 `"less -R"` 或 `"code --wait"`
- 编辑器会在临时文件中打开日志内容，编辑完成后临时文件会被自动删除
- 分页器用于只读查看，不会修改原始日志数据
- 如果没有配置编辑器或分页器，会显示相应的错误信息
- 从编辑器或分页器退出后，应用会自动重置终端状态并恢复界面显示

## 错误处理

- 如果编辑器或分页器命令不存在，会显示错误模态框
- 如果临时文件创建失败，会显示相应错误信息
- 错误信息通过模态框显示，不会中断应用运行

## 与 tali 项目的差异

虽然参考了 tali 项目的实现，但 flo 的实现有以下特点：
- 专门针对流水线日志内容进行优化
- 临时文件使用 `flo_logs_` 前缀
- 集成到现有的日志显示界面中
- 保持了 flo 原有的键盘快捷键风格

## 示例配置文件

```yaml
# Aliyun DevOps Pipelines TUI 配置文件
organization_id: "your_organization_id"
personal_access_token: "your_personal_access_token"
endpoint: "openapi-rdc.aliyuncs.com"

# 编辑器和分页器配置
editor: "code --wait"
pager: "less -R"
```

这样配置后，在日志界面按 `e` 会用 VS Code 打开日志，按 `v` 会用支持颜色的 less 查看日志。 