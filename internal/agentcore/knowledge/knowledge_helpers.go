package knowledge

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"
)

// splitText splits text into chunks of approximately maxChars.
func splitText(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}

	var chunks []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	var current strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if current.Len()+len(line)+1 > maxChars && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// extractReadableText finds printable ASCII runs in binary data.
func extractReadableText(data []byte) string {
	var sb strings.Builder
	var run strings.Builder
	for _, b := range data {
		if b >= 32 && b < 127 || b == '\n' || b == '\r' || b == '\t' {
			run.WriteByte(b)
		} else {
			if run.Len() > 20 {
				sb.WriteString(run.String())
				sb.WriteByte('\n')
			}
			run.Reset()
		}
	}
	if run.Len() > 20 {
		sb.WriteString(run.String())
	}
	return sb.String()
}

func shouldSkipRepoDir(name string) bool {
	switch name {
	case ".git", ".svn", ".hg", "node_modules", "vendor", "dist", "build", ".next", "coverage", "tmp", "Temp":
		return true
	default:
		return false
	}
}

func shouldSkipRepoFile(path, name string) bool {
	if strings.HasPrefix(name, ".") && name != ".env.example" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".rs", ".md", ".json", ".yaml", ".yml", ".sql", ".sh", ".txt":
		return false
	default:
		return true
	}
}

func detectRepoLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".js":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".sql":
		return "sql"
	case ".sh":
		return "shell"
	default:
		return "text"
	}
}

func splitRepoContent(relPath, language, content string, maxChars int) []string {
	header := fmt.Sprintf("FILE: %s\nLANG: %s\n\n", relPath, language)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	chunkBudget := maxChars - len(header)
	if chunkBudget < 100 {
		chunkBudget = 100
	}
	if language == "markdown" || language == "text" || language == "json" || language == "yaml" {
		parts := splitText(trimmed, chunkBudget)
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			out = append(out, header+part)
		}
		return out
	}

	lines := strings.Split(trimmed, "\n")
	var chunks []string
	var current strings.Builder
	current.WriteString(header)
	for _, line := range lines {
		if current.Len()+len(line)+1 > maxChars && current.Len() > len(header) {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteString(header)
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > len(header) {
		chunks = append(chunks, current.String())
	}
	return chunks
}
