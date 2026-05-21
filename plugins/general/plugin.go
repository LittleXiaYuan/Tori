package general

import (
	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/pkg/skills"
)

// GeneralPlugin bundles general-purpose skills (search, code exec, file ops, doc gen).
type GeneralPlugin struct {
	hostReadPaths  []string
	hostWritePaths []string
	searchFn       SearchFunc
	wfStore        workflow.Store
	pythonBin      string // injected from PythonEnv; empty = auto-detect
}

func New(hostReadPaths []string) *GeneralPlugin {
	return &GeneralPlugin{hostReadPaths: hostReadPaths}
}

func (p *GeneralPlugin) SetHostWritePaths(paths []string) {
	p.hostWritePaths = paths
}

func (p *GeneralPlugin) Name() string { return "general" }
func (p *GeneralPlugin) Description() string {
	return "通用插件：搜索、代码执行、文件浏览"
}

func (p *GeneralPlugin) SetSearchFunc(fn SearchFunc) {
	p.searchFn = fn
}

func (p *GeneralPlugin) SetWorkflowStore(s workflow.Store) {
	p.wfStore = s
}

// SetPythonBin injects the resolved Python binary for Office skills.
func (p *GeneralPlugin) SetPythonBin(bin string) {
	p.pythonBin = bin
}

func (p *GeneralPlugin) Skills() []skills.Skill {
	ws := NewWebSearchSkill()
	if p.searchFn != nil {
		ws.SetExternalSearch(p.searchFn)
	}

	// Default write dirs: data/tasks for task artifacts, data/output for general output,
	// plus "." and "output" to be more forgiving for apps/desktop/CLI usage.
	writeDirs := p.hostWritePaths
	if len(writeDirs) == 0 {
		writeDirs = []string{"data/tasks", "data/output", "output", "."}
	}
	// Read dirs for zip / xlsx_split: host read paths + write dirs (can read own outputs)
	readDirs := append(append([]string{}, p.hostReadPaths...), writeDirs...)

	return []skills.Skill{
		ws,
		NewComputerUseSkill(),
		NewCodeGenSkill(),
		NewFileSearchSkill(p.hostReadPaths),
		NewDocParseSkill(p.hostReadPaths),
		NewTranslateSkill(),
		NewImageGenSkill(),
		NewBrowserSkill(),
		NewFileWriteSkill(writeDirs),
		NewFileOpenSkill(readDirs),
		NewZipPackSkill(readDirs, writeDirs),
		NewZipUnpackSkill(readDirs, writeDirs),
		p.docxSkill(writeDirs),
		NewDocxFillSkill(readDirs, writeDirs),
		NewDocxEditSkill(readDirs, writeDirs),
		NewXlsxCreateSkill(writeDirs),
		NewXlsxFillSkill(readDirs, writeDirs),
		NewXlsxEditSkill(readDirs, writeDirs),
		NewXlsxSplitSkill(readDirs, writeDirs),
		NewPdfCreateSkill(writeDirs),
		NewHtmlExportSkill(writeDirs),
		p.pptxSkill(writeDirs),
		NewPptxFillSkill(readDirs, writeDirs),
		NewPptxEditSkill(readDirs, writeDirs),
		NewPptxTemplateSearchSkill(writeDirs),
		NewSendEmailSkill(),
		newWorkflowGenWithStore(p.wfStore),
	}
}

func (p *GeneralPlugin) docxSkill(dirs []string) skills.Skill {
	s := NewDocxCreateSkill(dirs)
	if p.pythonBin != "" {
		s.SetPythonBin(p.pythonBin)
	}
	return s
}

func (p *GeneralPlugin) pptxSkill(dirs []string) skills.Skill {
	s := NewPptxCreateSkill(dirs)
	if p.pythonBin != "" {
		s.SetPythonBin(p.pythonBin)
	}
	return s
}

func newWorkflowGenWithStore(store workflow.Store) skills.Skill {
	sk := NewWorkflowGenSkill()
	if store != nil {
		sk.SetStore(store)
	}
	return sk
}

func (p *GeneralPlugin) SystemPrompt() string {
	return `你具备通用能力，以下是各能力对应的工具名称（必须通过工具调用使用，不要直接输出内容替代）：

**信息获取**
- 网络搜索 → web_search
- 浏览器自动化（获取网页内容、提取正文/链接/HTTP头）→ browser_*

**代码与文件**
- 云端桌面沙箱（E2B Desktop，优先使用）→ computer_use（create/exec/status/destroy）
- 本地安全沙盒执行代码（Python/JavaScript/Go）→ code_gen（E2B不可用时回退）
- 浏览和搜索主机文件 → file_search
- 解析文档（PDF/Word/Excel/CSV/TXT/Markdown）→ doc_parse
- 创建和写入文件 → file_write

**文档生成（用户说"生成文档/写报告/做方案"时必须使用以下工具）**
- 生成 Word(.docx) → docx_create（content 参数使用 Markdown 子集：# ## ### 标题、列表、表格、图片）
- 填充 Word 模板 → docx_fill（{{key}} 占位符替换）
- 编辑 Word → docx_edit（替换文字、插入/删除段落、添加表格）
- 生成 Excel(.xlsx) → xlsx_create（CSV 格式数据）
- 填充 Excel 模板 → xlsx_fill
- 编辑 Excel → xlsx_edit
- 拆分 Excel → xlsx_split
- 生成 PDF → pdf_create
- 导出 HTML 报告 → html_export
- 生成 PPT(.pptx) → pptx_create（--- 分隔幻灯片）
- 填充 PPT 模板 → pptx_fill
- 编辑 PPT → pptx_edit

**压缩与通信**
- 打包 zip → zip_pack / 解压 → zip_unpack
- 发送邮件(SMTP) → send_email
- 多语言翻译 → translate
- AI 图片生成 → image_gen
- 自动生成工作流(NL2Workflow) → workflow_gen`
}
