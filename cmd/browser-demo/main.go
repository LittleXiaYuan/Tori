package main

import (
	"fmt"
	"os"
	"time"

	"yunque-agent/internal/execution/browser"
)

func main() {
	engine, err := browser.New(browser.Config{
		Headless: false,
		Timeout:  30 * time.Second,
		DataDir:  os.TempDir() + "/yunque-github-test",
	})
	if err != nil {
		fmt.Println("启动失败:", err)
		os.Exit(1)
	}
	defer engine.Close()

	fmt.Println("🌐 正在打开 GitHub 搜索 LittleXiaYuan ...")
	result, err := engine.Navigate("https://github.com/search?q=LittleXiaYuan&type=users")
	if err != nil {
		fmt.Println("导航失败:", err)
		os.Exit(1)
	}

	fmt.Printf("📄 标题: %s\n", result.Title)
	fmt.Printf("🔗 URL: %s\n", result.URL)

	// 等页面渲染
	time.Sleep(3 * time.Second)

	text, err := engine.ReadText("")
	if err != nil {
		fmt.Println("读取失败:", err)
	} else {
		preview := text
		if len(preview) > 1500 {
			preview = preview[:1500]
		}
		fmt.Printf("\n📝 页面内容:\n%s\n", preview)
	}

	ssPath := "data/browser/github_search.png"
	if err := engine.Screenshot(ssPath); err != nil {
		fmt.Println("截图失败:", err)
	} else {
		fmt.Printf("\n📸 截图已保存: %s\n", ssPath)
	}

	fmt.Println("\n✅ 完成！5秒后关闭...")
	time.Sleep(5 * time.Second)
}
