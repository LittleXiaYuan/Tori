package general

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

// ZipPackSkill creates zip archives from files or directories.
type ZipPackSkill struct {
	allowedReadDirs  []string
	allowedWriteDirs []string
}

func NewZipPackSkill(readDirs, writeDirs []string) *ZipPackSkill {
	return &ZipPackSkill{allowedReadDirs: readDirs, allowedWriteDirs: writeDirs}
}

func (s *ZipPackSkill) Name() string { return "zip_pack" }
func (s *ZipPackSkill) Description() string {
	return "将文件或目录打包成 zip 压缩包。支持多个源路径，可指定输出路径"
}

func (s *ZipPackSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sources": map[string]any{
				"type":        "string",
				"description": "要打包的文件或目录路径，多个用逗号分隔",
			},
			"output": map[string]any{
				"type":        "string",
				"description": "输出的 zip 文件路径",
			},
		},
		"required": []string{"sources", "output"},
	}
}

func (s *ZipPackSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	sourcesStr, _ := args["sources"].(string)
	output, _ := args["output"].(string)

	if sourcesStr == "" {
		return "", fmt.Errorf("sources is required")
	}
	if output == "" {
		return "", fmt.Errorf("output is required")
	}

	// Validate output path
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return "", fmt.Errorf("invalid output path: %w", err)
	}
	if !isUnderDirs(absOutput, s.allowedWriteDirs) {
		return "", fmt.Errorf("access denied: cannot write to %s", absOutput)
	}

	// Parse sources
	sources := strings.Split(sourcesStr, ",")
	for i := range sources {
		sources[i] = strings.TrimSpace(sources[i])
	}

	// Validate all source paths
	for _, src := range sources {
		absSrc, err := filepath.Abs(src)
		if err != nil {
			return "", fmt.Errorf("invalid source path %s: %w", src, err)
		}
		if !isUnderDirs(absSrc, s.allowedReadDirs) {
			return "", fmt.Errorf("access denied: cannot read %s", absSrc)
		}
	}

	// Create output directory
	if err := os.MkdirAll(filepath.Dir(absOutput), 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// Create zip file
	zf, err := os.Create(absOutput)
	if err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	defer zw.Close()

	fileCount := 0
	for _, src := range sources {
		absSrc, _ := filepath.Abs(src)
		info, err := os.Stat(absSrc)
		if err != nil {
			return "", fmt.Errorf("stat %s: %w", src, err)
		}

		if info.IsDir() {
			base := filepath.Base(absSrc)
			err = filepath.Walk(absSrc, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if fi.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(absSrc, path)
				return addFileToZip(zw, path, filepath.Join(base, rel))
			})
			if err != nil {
				return "", fmt.Errorf("walk %s: %w", src, err)
			}
		} else {
			if err := addFileToZip(zw, absSrc, filepath.Base(absSrc)); err != nil {
				return "", fmt.Errorf("add %s: %w", src, err)
			}
		}
		fileCount++
	}

	fi, _ := os.Stat(absOutput)
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}

	return fmt.Sprintf("已创建 %s（%d 个源，%d 字节）", absOutput, fileCount, size), nil
}

func addFileToZip(zw *zip.Writer, fsPath, zipPath string) error {
	// Normalize to forward slash in zip entry
	zipPath = filepath.ToSlash(zipPath)

	f, err := os.Open(fsPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// ──────────────────────────────────────────────

// ZipUnpackSkill extracts zip archives.
type ZipUnpackSkill struct {
	allowedReadDirs  []string
	allowedWriteDirs []string
}

func NewZipUnpackSkill(readDirs, writeDirs []string) *ZipUnpackSkill {
	return &ZipUnpackSkill{allowedReadDirs: readDirs, allowedWriteDirs: writeDirs}
}

func (s *ZipUnpackSkill) Name() string { return "zip_unpack" }
func (s *ZipUnpackSkill) Description() string {
	return "解压 zip 文件到指定目录，返回解压文件列表"
}

func (s *ZipUnpackSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"zip_path": map[string]any{
				"type":        "string",
				"description": "要解压的 zip 文件路径",
			},
			"output_dir": map[string]any{
				"type":        "string",
				"description": "解压目标目录",
			},
		},
		"required": []string{"zip_path", "output_dir"},
	}
}

func (s *ZipUnpackSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	zipPath, _ := args["zip_path"].(string)
	outputDir, _ := args["output_dir"].(string)

	if zipPath == "" {
		return "", fmt.Errorf("zip_path is required")
	}
	if outputDir == "" {
		return "", fmt.Errorf("output_dir is required")
	}

	absZip, err := filepath.Abs(zipPath)
	if err != nil {
		return "", fmt.Errorf("invalid zip path: %w", err)
	}
	if !isUnderDirs(absZip, s.allowedReadDirs) {
		return "", fmt.Errorf("access denied: cannot read %s", absZip)
	}

	absOut, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("invalid output dir: %w", err)
	}
	if !isUnderDirs(absOut, s.allowedWriteDirs) {
		return "", fmt.Errorf("access denied: cannot write to %s", absOut)
	}

	r, err := zip.OpenReader(absZip)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var extracted []string
	for _, f := range r.File {
		target := filepath.Join(absOut, filepath.FromSlash(f.Name))

		// Security: prevent zip slip
		if !strings.HasPrefix(filepath.Clean(target), absOut+string(filepath.Separator)) && filepath.Clean(target) != absOut {
			return "", fmt.Errorf("zip slip detected: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", fmt.Errorf("mkdir: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open entry %s: %w", f.Name, err)
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("create %s: %w", target, err)
		}

		// Limit extraction size to 100MB per file
		_, err = io.Copy(out, io.LimitReader(rc, 100*1024*1024))
		out.Close()
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("extract %s: %w", f.Name, err)
		}

		extracted = append(extracted, f.Name)
	}

	return fmt.Sprintf("已解压 %d 个文件到 %s\n%s", len(extracted), absOut, strings.Join(extracted, "\n")), nil
}

// isUnderDirs checks if absPath is under any of the allowed directories.
func isUnderDirs(absPath string, dirs []string) bool {
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
			return true
		}
	}
	return false
}
