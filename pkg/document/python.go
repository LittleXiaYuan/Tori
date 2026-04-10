package document

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"yunque-agent/pkg/skills"
)

const (
	pythonExecTimeout = 120 * time.Second
	maxOutputBytes    = 256 * 1024 // 256 KB
)

// PythonInterpreterSkill emulates ChatGPT's Code Interpreter by executing
// LLM-generated Python scripts in a subprocess with timeout and output limits.
type PythonInterpreterSkill struct{}

func (s *PythonInterpreterSkill) Name() string {
	return "python_interpreter"
}

func (s *PythonInterpreterSkill) Description() string {
	return "代码解释器沙盒：允许你编写并执行 Python 脚本，用于数据分析、可视化、数学计算或特殊文档处理。自动写入临时文件执行，限时2分钟，输出上限256KB。参数: code(Python脚本代码), timeout_seconds(可选,执行超时秒数,默认120)。"
}

func (s *PythonInterpreterSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "完整的 Python 脚本代码",
			},
			"timeout_seconds": map[string]any{
				"type":        "number",
				"description": "执行超时秒数，默认120，最大300",
			},
		},
		"required": []string{"code"},
	}
}

func (s *PythonInterpreterSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	code, _ := args["code"].(string)
	if code == "" {
		return "", fmt.Errorf("code 参数是必需的")
	}

	timeout := pythonExecTimeout
	if ts, ok := args["timeout_seconds"].(float64); ok && ts > 0 {
		if ts > 300 {
			ts = 300
		}
		timeout = time.Duration(ts) * time.Second
	}

	tmpFile, err := os.CreateTemp("", "yunque-sandbox-*.py")
	if err != nil {
		return "", fmt.Errorf("无法创建临时文件: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("写入代码失败: %w", err)
	}
	tmpFile.Close()

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "python", filepath.ToSlash(tmpPath))

	var outBuf, errBuf limitedBuffer
	outBuf.max = maxOutputBytes
	errBuf.max = maxOutputBytes
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()

	outStr := outBuf.String()
	errStr := errBuf.String()

	if outBuf.truncated {
		outStr += "\n...(输出已截断，超过256KB限制)"
	}
	if errBuf.truncated {
		errStr += "\n...(错误输出已截断)"
	}

	if execCtx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("【执行超时】\n脚本执行超过 %v 被强制终止。\n已捕获的输出:\n%s\n错误:\n%s",
			timeout, outStr, errStr), nil
	}

	if err != nil {
		return fmt.Sprintf("【执行失败】\n退出信号: %v\n标准错误:\n%s\n标准输出:\n%s", err, errStr, outStr), nil
	}

	result := outStr
	if errStr != "" {
		result += "\n\n【警告/Stderr】\n" + errStr
	}

	return fmt.Sprintf("【执行成功】\n%s", result), nil
}

// limitedBuffer is a bytes.Buffer that stops accepting writes after max bytes.
type limitedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	if lb.truncated {
		return len(p), nil // discard silently
	}
	remaining := lb.max - lb.buf.Len()
	if remaining <= 0 {
		lb.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		lb.buf.Write(p[:remaining])
		lb.truncated = true
		return len(p), nil
	}
	return lb.buf.Write(p)
}

func (lb *limitedBuffer) String() string {
	return lb.buf.String()
}
