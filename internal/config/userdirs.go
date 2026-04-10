package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// UserDir represents a discovered user directory.
type UserDir struct {
	Label   string `json:"label"`
	LabelZh string `json:"label_zh"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Kind    string `json:"kind"` // desktop, documents, downloads, pictures, music, videos, home
}

// DetectUserDirs discovers common user directories across platforms.
func DetectUserDirs() []UserDir {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	candidates := []struct {
		label, labelZh, kind string
		relPaths             []string // try in order; first existing wins
	}{
		{"Home", "用户主目录", "home", []string{""}},
		{"Desktop", "桌面", "desktop", desktopPaths()},
		{"Documents", "文档", "documents", documentsPaths()},
		{"Downloads", "下载", "downloads", downloadsPaths()},
		{"Pictures", "图片", "pictures", picturesPaths()},
		{"Music", "音乐", "music", musicPaths()},
		{"Videos", "视频", "videos", videosPaths()},
	}

	var dirs []UserDir
	for _, c := range candidates {
		dir := UserDir{
			Label:   c.label,
			LabelZh: c.labelZh,
			Kind:    c.kind,
		}
		if c.kind == "home" {
			dir.Path = home
			dir.Exists = true
			dirs = append(dirs, dir)
			continue
		}
		for _, rel := range c.relPaths {
			p := filepath.Join(home, rel)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				dir.Path = p
				dir.Exists = true
				break
			}
		}
		if dir.Path == "" && len(c.relPaths) > 0 {
			dir.Path = filepath.Join(home, c.relPaths[0])
		}
		dirs = append(dirs, dir)
	}

	return dirs
}

// DefaultReadPaths returns a set of safe default directories suitable for
// HOST_READ_PATHS when the user hasn't configured any.
// Only returns paths that actually exist on disk.
func DefaultReadPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	tryPaths := []string{
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Downloads"),
	}
	if runtime.GOOS == "windows" {
		tryPaths = append(tryPaths,
			filepath.Join(home, "桌面"),
			filepath.Join(home, "文档"),
			filepath.Join(home, "下载"),
		)
	}

	var result []string
	seen := map[string]bool{}
	for _, p := range tryPaths {
		norm := strings.ToLower(filepath.Clean(p))
		if seen[norm] {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			result = append(result, p)
			seen[norm] = true
		}
	}
	return result
}

func desktopPaths() []string {
	if runtime.GOOS == "windows" {
		return []string{"Desktop", "桌面", "OneDrive\\Desktop", "OneDrive\\桌面"}
	}
	return []string{"Desktop", "桌面"}
}

func documentsPaths() []string {
	if runtime.GOOS == "windows" {
		return []string{"Documents", "文档", "OneDrive\\Documents", "OneDrive\\文档"}
	}
	return []string{"Documents", "文档"}
}

func downloadsPaths() []string {
	return []string{"Downloads", "下载"}
}

func picturesPaths() []string {
	if runtime.GOOS == "windows" {
		return []string{"Pictures", "图片", "OneDrive\\Pictures", "OneDrive\\图片"}
	}
	return []string{"Pictures", "图片"}
}

func musicPaths() []string {
	return []string{"Music", "音乐"}
}

func videosPaths() []string {
	return []string{"Videos", "视频"}
}
