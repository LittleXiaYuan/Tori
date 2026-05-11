package general

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"yunque-agent/pkg/skills"
)

// DocxCreateSkill — generates .docx from structured content.
// DOCX is Open XML (ECMA-376): a zip of XML parts.
// We shell out to Python (python-docx) for the heavy lifting.

type DocxCreateSkill struct {
	allowedDirs []string
	verifier    *DocxVerifier
	pythonBin   string
}

func NewDocxCreateSkill(allowedDirs []string) *DocxCreateSkill {
	return &DocxCreateSkill{allowedDirs: allowedDirs}
}

// SetPythonBin injects the Python binary path from PythonEnv.
func (s *DocxCreateSkill) SetPythonBin(bin string) {
	s.pythonBin = bin
}

// SetVerifier attaches a DocxVerifier for post-generation preview rendering.
func (s *DocxCreateSkill) SetVerifier(v *DocxVerifier) {
	s.verifier = v
}

func (s *DocxCreateSkill) Name() string { return "docx_create" }
func (s *DocxCreateSkill) Description() string {
	return "生成 Word 文档(.docx)。支持 #/##/### 标题、列表、Markdown 表格、![](path) 图片(path 须在允许目录内)、页眉。用于报告与方案"
}

func (s *DocxCreateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "输出文件路径（如 data/output/report.docx）",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "文档标题",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Markdown：# ## ### 标题、-/* 列表、GFM 表格(| a | b |)、单独一行的 ![](路径) 图片",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (s *DocxCreateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if !isUnderAllowed(absPath, s.allowedDirs) {
		return "", fmt.Errorf("access denied: path %s is not under allowed directories", path)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}

	payload := parseDocContentToPayload(title, content, s.allowedDirs)

	complexity := estimateDocComplexity(payload)
	pyBin := s.pythonBin
	if pyBin == "" {
		pyBin = findPython()
	}
	hasPython := pyBin != ""

	var engine string
	switch {
	case complexity >= 2 && hasPython:
		if err := tryPythonDocx(ctx, pyBin, absPath, title, content, payload); err != nil {
			slog.Info("docx: python-docx failed on complex doc, falling back to Go", "err", err)
			if err2 := writeDocx(absPath, payload); err2 != nil {
				return "", fmt.Errorf("docx generation failed: %w", err2)
			}
			engine = "Go-OOXML(fallback)"
		} else {
			engine = "python-docx"
		}
	case complexity >= 2 && !hasPython:
		slog.Info("docx: complex document but no Python, using Go engine")
		if err := writeDocx(absPath, payload); err != nil {
			return "", fmt.Errorf("docx generation failed: %w", err)
		}
		engine = "Go-OOXML"
	default:
		if err := writeDocx(absPath, payload); err != nil {
			return "", fmt.Errorf("docx generation failed: %w", err)
		}
		engine = "Go-OOXML(fast)"
	}

	info, _ := os.Stat(absPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}

	var extras []string

	if warnings := StructuralCheck(payload); len(warnings) > 0 {
		slog.Warn("docx: structural issues", "count", len(warnings), "warnings", warnings)
		extras = append(extras, fmt.Sprintf("warnings=%d", len(warnings)))
	}

	if s.verifier != nil {
		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		pngPath, err := s.verifier.RenderPreview(ctx, payload, baseName)
		if err != nil {
			slog.Warn("docx: preview render failed", "err", err)
			extras = append(extras, "preview=failed")
		} else {
			extras = append(extras, fmt.Sprintf("preview=%s", pngPath))
		}
	}

	result := fmt.Sprintf("已生成 Word 文档: %s (%d bytes, %d 块, engine=%s)", path, size, len(payload.Blocks), engine)
	if len(extras) > 0 {
		result += " [" + strings.Join(extras, ", ") + "]"
	}
	return result, nil
}

// tryPythonDocx attempts to generate the docx via python-docx for higher quality output.
// Returns an error if python or python-docx is not available, allowing caller to fall back.
func tryPythonDocx(ctx context.Context, pythonBin, absPath, title, content string, payload docxPayload) error {
	if pythonBin == "" {
		return fmt.Errorf("python not found")
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	jsonFile, err := os.CreateTemp("", "docx_data_*.json")
	if err != nil {
		return err
	}
	jsonPath := jsonFile.Name()
	defer os.Remove(jsonPath)
	jsonFile.Write(jsonBytes)
	jsonFile.Close()

	pyFile, err := os.CreateTemp("", "docx_renderer_*.py")
	if err != nil {
		return err
	}
	pyPath := pyFile.Name()
	defer os.Remove(pyPath)
	pyFile.WriteString(docxPythonScript)
	pyFile.Close()

	templatePath, _ := filepath.Abs(filepath.Join("data", "templates", "report.docx"))
	cmd := exec.CommandContext(ctx, pythonBin, pyPath, jsonPath, absPath, templatePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

// estimateDocComplexity scores the payload's layout difficulty.
// 0-1 = simple (plain text/lists), 2+ = complex (tables, images, many headings).
func estimateDocComplexity(p docxPayload) int {
	score := 0
	headings, tables, images := 0, 0, 0
	for _, b := range p.Blocks {
		switch b.Type {
		case "table":
			tables++
		case "image":
			images++
		case "paragraph":
			if b.Style == "Heading1" || b.Style == "Heading2" || b.Style == "Heading3" {
				headings++
			}
		}
	}
	if tables > 0 {
		score += 2
	}
	if images > 0 {
		score += 2
	}
	if headings >= 4 {
		score++
	}
	if len(p.Blocks) > 30 {
		score++
	}
	return score
}

func findPython() string {
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

const docxPythonScript = `#!/usr/bin/env python3
import sys, json, os, re

def add_inline_runs(paragraph, text, extra_bold=False):
    """Parse **bold**, *italic*, ***bold-italic*** and add as runs."""
    pattern = re.compile(r'(\*\*\*(.+?)\*\*\*|\*\*(.+?)\*\*|\*(.+?)\*)')
    pos = 0
    for m in pattern.finditer(text):
        if m.start() > pos:
            run = paragraph.add_run(text[pos:m.start()])
            if extra_bold: run.bold = True
        if m.group(2):
            run = paragraph.add_run(m.group(2))
            run.bold = True; run.italic = True
        elif m.group(3):
            run = paragraph.add_run(m.group(3))
            run.bold = True
        elif m.group(4):
            run = paragraph.add_run(m.group(4))
            run.italic = True
        pos = m.end()
    if pos < len(text):
        run = paragraph.add_run(text[pos:])
        if extra_bold: run.bold = True
    if pos == 0 and not text:
        pass

def main():
    json_path, out_path = sys.argv[1], sys.argv[2]
    template_path = sys.argv[3] if len(sys.argv) > 3 else None
    try:
        from docx import Document
        from docx.shared import Inches, Pt, RGBColor, Cm
        from docx.enum.text import WD_ALIGN_PARAGRAPH
        from docx.enum.table import WD_TABLE_ALIGNMENT
        from docx.oxml.ns import qn, nsdecls
        from docx.oxml import parse_xml
    except ImportError:
        sys.exit("python-docx not installed")
    with open(json_path, 'r', encoding='utf-8') as f:
        payload = json.load(f)
    doc = Document(template_path) if template_path and os.path.exists(template_path) else Document()
    if not (template_path and os.path.exists(template_path)):
        style = doc.styles['Normal']; style.font.name = 'Calibri'; style.font.size = Pt(11)
    header_text = payload.get('header', '')
    if header_text:
        sec = doc.sections[0]; h = sec.header; h.is_linked_to_previous = False
        hp = h.paragraphs[0] if h.paragraphs else h.add_paragraph()
        hp.text = header_text; hp.alignment = WD_ALIGN_PARAGRAPH.RIGHT
        for run in hp.runs: run.font.size = Pt(9); run.font.color.rgb = RGBColor(0x99,0x99,0x99)
    for blk in payload.get('blocks', []):
        bt = blk.get('type','paragraph')
        if bt == 'paragraph':
            sn = blk.get('style','Normal'); txt = blk.get('text','')
            if sn == 'Title': doc.add_heading(txt, level=0)
            elif sn == 'Heading1': doc.add_heading(txt, level=1)
            elif sn == 'Heading2': doc.add_heading(txt, level=2)
            elif sn == 'Heading3': doc.add_heading(txt, level=3)
            elif sn == 'ListBullet':
                p = doc.add_paragraph(style='List Bullet')
                add_inline_runs(p, txt)
            elif sn == 'ListNumber':
                p = doc.add_paragraph(style='List Number')
                add_inline_runs(p, txt)
            elif sn == 'Quote':
                p = doc.add_paragraph()
                pf = p.paragraph_format
                pf.left_indent = Cm(1.5)
                pPr = p._element.get_or_add_pPr()
                pBdr = parse_xml(
                    '<w:pBdr %s>'
                    '<w:left w:val="single" w:sz="12" w:space="4" w:color="CCCCCC"/>'
                    '</w:pBdr>' % nsdecls('w'))
                pPr.append(pBdr)
                add_inline_runs(p, txt)
                for run in p.runs:
                    run.font.color.rgb = RGBColor(0x40, 0x40, 0x40)
                    run.italic = True
            else:
                p = doc.add_paragraph()
                add_inline_runs(p, txt)
        elif bt == 'table':
            rows = blk.get('rows',[])
            if not rows: continue
            nc = max(len(r) for r in rows)
            tbl = doc.add_table(rows=len(rows), cols=nc, style='Light Grid Accent 1')
            tbl.alignment = WD_TABLE_ALIGNMENT.CENTER
            for ri, row in enumerate(rows):
                for ci, ct in enumerate(row):
                    cell = tbl.cell(ri, ci)
                    cell.text = ''
                    p = cell.paragraphs[0]
                    add_inline_runs(p, ct, extra_bold=(ri == 0))
            doc.add_paragraph()
        elif bt == 'hr':
            p = doc.add_paragraph()
            pPr = p._element.get_or_add_pPr()
            pBdr = parse_xml(
                '<w:pBdr %s>'
                '<w:bottom w:val="single" w:sz="6" w:space="1" w:color="CCCCCC"/>'
                '</w:pBdr>' % nsdecls('w'))
            pPr.append(pBdr)
        elif bt == 'image':
            ip = blk.get('path','')
            if ip and os.path.isfile(ip):
                try: doc.add_picture(ip, width=Inches(5.5))
                except: doc.add_paragraph(f'[Image: {ip}]')
    doc.save(out_path)
if __name__ == '__main__': main()
`

// writeDocx generates a valid OOXML .docx file as a zip archive.
func writeDocx(path string, payload docxPayload) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	bodyXML := buildDocxBodyXML(payload)

	for _, entry := range []struct{ name, content string }{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRootRels},
		{"word/_rels/document.xml.rels", docxDocRels},
		{"word/styles.xml", docxStylesXML},
		{"word/document.xml", bodyXML},
	} {
		if err := writeZipFile(w, entry.name, entry.content); err != nil {
			return fmt.Errorf("write %s: %w", entry.name, err)
		}
	}
	return nil
}

func buildDocxBodyXML(payload docxPayload) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"
  xmlns:mo="http://schemas.microsoft.com/office/mac/office/2008/main"
  xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
  xmlns:mv="urn:schemas-microsoft-com:mac:vml"
  xmlns:o="urn:schemas-microsoft-com:office:office"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"
  xmlns:v="urn:schemas-microsoft-com:vml"
  xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
  xmlns:w10="urn:schemas-microsoft-com:office:word"
  xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml">
<w:body>
`)
	for _, blk := range payload.Blocks {
		switch blk.Type {
		case "paragraph":
			sb.WriteString(docxParagraph(blk.Style, blk.Text))
		case "table":
			sb.WriteString(docxTable(blk.Rows))
		case "hr":
			sb.WriteString(docxHorizontalRule())
		}
	}
	sb.WriteString(`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>
</w:body>
</w:document>`)
	return sb.String()
}

var reInlineBoldItalic = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)
var reInlineBold = regexp.MustCompile(`\*\*(.+?)\*\*`)
var reInlineItalic = regexp.MustCompile(`\*(.+?)\*`)
var reNumberedList = regexp.MustCompile(`^(\d{1,3})\.\s+(.*)$`)

type inlineRun struct {
	text   string
	bold   bool
	italic bool
}

// parseInlineMarkdown splits text into runs with bold/italic attributes.
func parseInlineMarkdown(text string) []inlineRun {
	type segment struct {
		start, end int
		text       string
		bold, ital bool
	}
	var segs []segment

	used := make([]bool, len(text))

	for _, m := range reInlineBoldItalic.FindAllStringSubmatchIndex(text, -1) {
		for i := m[0]; i < m[1]; i++ {
			used[i] = true
		}
		segs = append(segs, segment{m[0], m[1], text[m[2]:m[3]], true, true})
	}
	for _, m := range reInlineBold.FindAllStringSubmatchIndex(text, -1) {
		overlap := false
		for i := m[0]; i < m[1]; i++ {
			if used[i] {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}
		for i := m[0]; i < m[1]; i++ {
			used[i] = true
		}
		segs = append(segs, segment{m[0], m[1], text[m[2]:m[3]], true, false})
	}
	for _, m := range reInlineItalic.FindAllStringSubmatchIndex(text, -1) {
		overlap := false
		for i := m[0]; i < m[1]; i++ {
			if used[i] {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}
		for i := m[0]; i < m[1]; i++ {
			used[i] = true
		}
		segs = append(segs, segment{m[0], m[1], text[m[2]:m[3]], false, true})
	}

	if len(segs) == 0 {
		return []inlineRun{{text: text}}
	}

	// Sort segments by position
	for i := 0; i < len(segs)-1; i++ {
		for j := i + 1; j < len(segs); j++ {
			if segs[j].start < segs[i].start {
				segs[i], segs[j] = segs[j], segs[i]
			}
		}
	}

	var runs []inlineRun
	pos := 0
	for _, s := range segs {
		if s.start > pos {
			runs = append(runs, inlineRun{text: text[pos:s.start]})
		}
		runs = append(runs, inlineRun{text: s.text, bold: s.bold, italic: s.ital})
		pos = s.end
	}
	if pos < len(text) {
		runs = append(runs, inlineRun{text: text[pos:]})
	}
	return runs
}

func docxParagraph(style, text string) string {
	pStyle := ""
	switch style {
	case "Title":
		pStyle = "Title"
	case "Heading1":
		pStyle = "Heading1"
	case "Heading2":
		pStyle = "Heading2"
	case "Heading3":
		pStyle = "Heading3"
	case "ListBullet":
		pStyle = "ListBullet"
	case "ListNumber":
		pStyle = "ListNumber"
	case "Quote":
		pStyle = "Quote"
	default:
		pStyle = ""
	}
	var sb strings.Builder
	sb.WriteString("<w:p>")
	if pStyle != "" {
		sb.WriteString(fmt.Sprintf(`<w:pPr><w:pStyle w:val="%s"/></w:pPr>`, pStyle))
	}

	runs := parseInlineMarkdown(text)
	for _, r := range runs {
		sb.WriteString("<w:r>")
		if r.bold || r.italic {
			sb.WriteString("<w:rPr>")
			if r.bold {
				sb.WriteString("<w:b/>")
			}
			if r.italic {
				sb.WriteString("<w:i/>")
			}
			sb.WriteString("</w:rPr>")
		}
		sb.WriteString(fmt.Sprintf(`<w:t xml:space="preserve">%s</w:t>`, docXMLEscape(r.text)))
		sb.WriteString("</w:r>")
	}
	sb.WriteString("</w:p>\n")
	return sb.String()
}

func docxTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/></w:tblPr>`)
	for ri, row := range rows {
		sb.WriteString("<w:tr>")
		for _, cell := range row {
			sb.WriteString("<w:tc><w:p>")
			runs := parseInlineMarkdown(cell)
			for _, r := range runs {
				sb.WriteString("<w:r><w:rPr>")
				if ri == 0 || r.bold {
					sb.WriteString("<w:b/>")
				}
				if r.italic {
					sb.WriteString("<w:i/>")
				}
				sb.WriteString("</w:rPr>")
				sb.WriteString(fmt.Sprintf(`<w:t xml:space="preserve">%s</w:t>`, docXMLEscape(r.text)))
				sb.WriteString("</w:r>")
			}
			sb.WriteString("</w:p></w:tc>")
		}
		sb.WriteString("</w:tr>")
	}
	sb.WriteString("</w:tbl>\n<w:p/>\n")
	return sb.String()
}

func docxHorizontalRule() string {
	return `<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="CCCCCC"/></w:pBdr></w:pPr></w:p>` + "\n"
}

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

const docxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const docxDocRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const docxStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:jc w:val="center"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="44"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="32"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="200" w:after="80"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="28"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="160" w:after="60"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="24"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListBullet">
    <w:name w:val="List Bullet"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:ind w:left="720"/></w:pPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListNumber">
    <w:name w:val="List Number"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:ind w:left="720"/></w:pPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Quote">
    <w:name w:val="Quote"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:ind w:left="720"/><w:pBdr><w:left w:val="single" w:sz="12" w:space="4" w:color="CCCCCC"/></w:pBdr></w:pPr>
    <w:rPr><w:i/><w:color w:val="404040"/></w:rPr>
  </w:style>
  <w:style w:type="table" w:styleId="TableGrid">
    <w:name w:val="Table Grid"/><w:tblPr><w:tblBorders>
      <w:top w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:left w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:bottom w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:right w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:insideH w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:insideV w:val="single" w:sz="4" w:space="0" w:color="000000"/>
    </w:tblBorders></w:tblPr>
  </w:style>
</w:styles>`

// docxPayload is the JSON contract between Go and the Python renderer.
type docxPayload struct {
	Header string     `json:"header,omitempty"`
	Blocks []docBlock `json:"blocks"`
}

type docBlock struct {
	Type  string     `json:"type"` // paragraph | table | image
	Style string     `json:"style,omitempty"`
	Text  string     `json:"text,omitempty"`
	Rows  [][]string `json:"rows,omitempty"`
	Path  string     `json:"path,omitempty"`
}

func parseDocContentToPayload(title, content string, allowed []string) docxPayload {
	var p docxPayload
	if title != "" {
		p.Header = title
	}
	lines := strings.Split(content, "\n")
	i := 0
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		trim := strings.TrimSpace(line)
		if trim == "" {
			i++
			continue
		}
		if trim == "---" || trim == "***" || trim == "___" {
			p.Blocks = append(p.Blocks, docBlock{Type: "hr"})
			i++
			continue
		}
		if m := mdImageRe.FindStringSubmatch(trim); m != nil {
			ap := sanitizeImagePath(m[1], allowed)
			if ap != "" {
				p.Blocks = append(p.Blocks, docBlock{Type: "image", Path: ap})
			}
			i++
			continue
		}
		if isTableRow(trim) {
			start := i
			for i < len(lines) {
				t := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
				if t == "" {
					break
				}
				if isTableRow(t) || isSeparatorRow(t) {
					i++
					continue
				}
				break
			}
			var rows [][]string
			for _, ln := range lines[start:i] {
				t := strings.TrimSpace(strings.TrimRight(ln, "\r"))
				if isSeparatorRow(t) {
					continue
				}
				if r := splitPipeRow(t); len(r) >= 2 {
					rows = append(rows, r)
				}
			}
			if len(rows) > 0 {
				p.Blocks = append(p.Blocks, docBlock{Type: "table", Rows: rows})
			}
			continue
		}
		if strings.HasPrefix(trim, "### ") {
			p.Blocks = append(p.Blocks, docBlock{Type: "paragraph", Style: "Heading3", Text: strings.TrimPrefix(trim, "### ")})
		} else if strings.HasPrefix(trim, "## ") {
			p.Blocks = append(p.Blocks, docBlock{Type: "paragraph", Style: "Heading2", Text: strings.TrimPrefix(trim, "## ")})
		} else if strings.HasPrefix(trim, "# ") {
			p.Blocks = append(p.Blocks, docBlock{Type: "paragraph", Style: "Heading1", Text: strings.TrimPrefix(trim, "# ")})
		} else if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") {
			p.Blocks = append(p.Blocks, docBlock{Type: "paragraph", Style: "ListBullet", Text: strings.TrimSpace(trim[2:])})
		} else if reNumberedList.MatchString(trim) {
			m := reNumberedList.FindStringSubmatch(trim)
			p.Blocks = append(p.Blocks, docBlock{Type: "paragraph", Style: "ListNumber", Text: m[1] + ". " + strings.TrimSpace(m[2])})
		} else if strings.HasPrefix(trim, "> ") || trim == ">" {
			text := strings.TrimPrefix(trim, "> ")
			text = strings.TrimPrefix(text, ">")
			p.Blocks = append(p.Blocks, docBlock{Type: "paragraph", Style: "Quote", Text: text})
		} else {
			p.Blocks = append(p.Blocks, docBlock{Type: "paragraph", Style: "Normal", Text: line})
		}
		i++
	}
	if title != "" {
		p.Blocks = append([]docBlock{{Type: "paragraph", Style: "Title", Text: title}}, p.Blocks...)
	}
	return p
}

// parseDocContent is kept for backward-compatible tests and callers.
// It returns only paragraph blocks in original order.
func parseDocContent(title, content string) []docBlock {
	p := parseDocContentToPayload(title, content, nil)
	out := make([]docBlock, 0, len(p.Blocks))
	for _, b := range p.Blocks {
		if b.Type == "paragraph" {
			out = append(out, b)
		}
	}
	return out
}

func writeZipFile(w *zip.Writer, name, content string) error {
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(content))
	return err
}

func docXMLEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

