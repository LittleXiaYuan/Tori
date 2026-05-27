# Git Hooks / Git 钩子

This directory contains optional Git hooks to help maintain commit message quality.

本目录包含可选的 Git 钩子，用于帮助维护提交消息质量。

## Available Hooks / 可用钩子

### `commit-msg`

Validates commit messages against Conventional Commits format and provides helpful warnings.

验证提交消息是否符合 Conventional Commits 格式，并提供有用的警告。

**Features / 功能**:
- ✓ Validates commit message format
- ✓ Checks for common mistakes (capitalization, trailing period, length)
- ✓ Suggests potential type misclassifications
- ✓ Allows merge and revert commits
- ✓ Non-blocking warnings for style issues

## Installation / 安装

### Option 1: Enable for this repository (Recommended) / 选项 1：为此仓库启用（推荐）

```bash
git config core.hooksPath .githooks
```

This tells Git to use hooks from `.githooks/` directory instead of `.git/hooks/`.

这告诉 Git 使用 `.githooks/` 目录中的钩子，而不是 `.git/hooks/`。

### Option 2: Copy to .git/hooks / 选项 2：复制到 .git/hooks

```bash
# On Unix/Linux/macOS
cp .githooks/commit-msg .git/hooks/commit-msg
chmod +x .git/hooks/commit-msg

# On Windows (Git Bash)
cp .githooks/commit-msg .git/hooks/commit-msg
```

### Option 3: Symlink (Unix/Linux/macOS only) / 选项 3：符号链接（仅 Unix/Linux/macOS）

```bash
ln -s ../../.githooks/commit-msg .git/hooks/commit-msg
```

## Disabling Hooks / 禁用钩子

### Temporarily skip for one commit / 临时跳过一次提交

```bash
git commit --no-verify -m "your message"
```

### Disable permanently / 永久禁用

```bash
# If using Option 1
git config --unset core.hooksPath

# If using Option 2 or 3
rm .git/hooks/commit-msg
```

## Example Output / 示例输出

### Valid commit / 有效提交

```
$ git commit -m "feat(cogni): add reflective learning loop"
✓ Commit message format is valid
```

### Invalid format / 无效格式

```
$ git commit -m "added new feature"
✗ Invalid commit message format

Expected format:
  <type>(<scope>): <subject>

Valid types:
  feat     - New feature
  fix      - Bug fix
  refactor - Code refactoring
  ...

Examples:
  feat(cogni): add reflective learning loop
  fix(planner): prevent nil pointer in context assembly
```

### Warning for potential misclassification / 潜在误分类警告

```
$ git commit -m "feat(planner): extract runtime boundary"
⚠ Warning: This might be a 'refactor' instead of 'feat'
Subject: extract runtime boundary
Reason: Suggests code restructuring without new functionality

✓ Commit message format is valid
```

## Customization / 自定义

You can modify the hooks in `.githooks/` to fit your team's needs. The hooks are written in Bash and are easy to customize.

您可以修改 `.githooks/` 中的钩子以适应您团队的需求。这些钩子使用 Bash 编写，易于自定义。

## Troubleshooting / 故障排除

### Hook not running / 钩子未运行

1. Check if hooks are executable:
   ```bash
   ls -la .githooks/commit-msg
   ```
   Should show `-rwxr-xr-x` (executable)

2. Verify Git configuration:
   ```bash
   git config core.hooksPath
   ```
   Should output `.githooks`

3. Test the hook manually:
   ```bash
   echo "test: invalid message" > /tmp/test-msg
   .githooks/commit-msg /tmp/test-msg
   ```

### Hook fails on Windows / 钩子在 Windows 上失败

Make sure you're using Git Bash or WSL. The hooks are Bash scripts and won't work in PowerShell or CMD.

确保您使用的是 Git Bash 或 WSL。这些钩子是 Bash 脚本，在 PowerShell 或 CMD 中无法工作。

## See Also / 另请参阅

- [docs/COMMIT-CONVENTIONS.md](../docs/COMMIT-CONVENTIONS.md) - Full commit conventions guide
- [.gitmessage](../.gitmessage) - Commit message template
- [scripts/check-commit-type.mjs](../scripts/check-commit-type.mjs) - Commit type distribution analyzer
