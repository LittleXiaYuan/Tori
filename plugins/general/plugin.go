package general

import "yunque-agent/pkg/skills"

// GeneralPlugin bundles all general-purpose skills.
type GeneralPlugin struct {
	hostReadPaths  []string
	hostWritePaths []string
	searchFn       SearchFunc
}

func New(hostReadPaths []string) *GeneralPlugin {
	return &GeneralPlugin{hostReadPaths: hostReadPaths}
}

// SetHostWritePaths sets directories where file_write and zip skills can write.
func (p *GeneralPlugin) SetHostWritePaths(paths []string) {
	p.hostWritePaths = paths
}

func (p *GeneralPlugin) Name() string { return "general" }
func (p *GeneralPlugin) Description() string {
	return "通用插件：搜索、代码执行、文件浏览"
}

// SetSearchFunc injects an external search function into the web search skill.
func (p *GeneralPlugin) SetSearchFunc(fn SearchFunc) {
	p.searchFn = fn
}

func (p *GeneralPlugin) Skills() []skills.Skill {
	ws := NewWebSearchSkill()
	if p.searchFn != nil {
		ws.SetExternalSearch(p.searchFn)
	}

	// Default write dirs: data/tasks for task artifacts, data/output for general output
	writeDirs := p.hostWritePaths
	if len(writeDirs) == 0 {
		writeDirs = []string{"data/tasks", "data/output"}
	}
	// Read dirs for zip: host read paths + write dirs (can zip own outputs)
	readDirs := append(append([]string{}, p.hostReadPaths...), writeDirs...)

	return []skills.Skill{
		ws,
		NewCodeGenSkill(),
		NewFileSearchSkill(p.hostReadPaths),
		NewDocParseSkill(p.hostReadPaths),
		NewTranslateSkill(),
		NewImageGenSkill(),
		NewBrowserSkill(),
		NewFileWriteSkill(writeDirs),
		NewZipPackSkill(readDirs, writeDirs),
		NewZipUnpackSkill(readDirs, writeDirs),
		NewDocxCreateSkill(writeDirs),
		NewXlsxCreateSkill(writeDirs),
		NewHtmlExportSkill(writeDirs),
		NewPptxCreateSkill(writeDirs),
	}
}

func (p *GeneralPlugin) SystemPrompt() string {
	return `你具备通用能力：
- 网络搜索获取最新信息
- 在安全沙盒中执行代码（Python/JavaScript/Go）
- 浏览和搜索主机文件系统（只读）
- 创建和写入文件（报告、代码、配置等任务产出物）
- 打包文件成 zip 压缩包 / 解压 zip 文件
- 解析文档文件（PDF/Word/Excel/CSV/TXT/Markdown），提取文本内容
- 生成 Word 文档(.docx)：支持标题、段落、列表，使用 Markdown 子集语法描述内容
- 生成 Excel 表格(.xlsx)：支持 CSV 格式数据输入，自动表头加粗
- 导出 HTML 网页报告：将 Markdown 内容渲染为美观的独立 HTML 页面
- 生成 PowerPoint 演示文稿(.pptx)：用 --- 分隔幻灯片，每张第一行为标题
- 多语言翻译（支持中英日韩法德西俄等20+语言，可指定翻译风格）
- AI图片生成（根据文字描述生成图片，支持多种尺寸和风格）
- 浏览器自动化（获取网页内容、提取正文、提取链接、查看HTTP头）`
}
