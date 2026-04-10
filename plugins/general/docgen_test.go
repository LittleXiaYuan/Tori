package general

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocxCreate(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		if _, err2 := exec.LookPath("python"); err2 != nil {
			t.Skip("skipping: python not found in PATH")
		}
	}
	dir := t.TempDir()
	skill := NewDocxCreateSkill([]string{dir})

	result, err := skill.Execute(context.Background(), map[string]any{
		"path":  filepath.Join(dir, "test.docx"),
		"title": "测试报告",
		"content": `# 第一章 概述
这是一段普通文本。

## 1.1 背景
项目背景介绍。

- 要点一
- 要点二
- 要点三

## 1.2 目标
实现文档生成功能。`,
	}, nil)
	if err != nil {
		t.Fatalf("docx_create failed: %v", err)
	}
	if !strings.Contains(result, ".docx") {
		t.Errorf("result should mention .docx: %s", result)
	}

	// Verify it's a valid zip
	r, err := zip.OpenReader(filepath.Join(dir, "test.docx"))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	defer r.Close()

	// Check required files exist
	required := map[string]bool{
		"[Content_Types].xml":          false,
		"_rels/.rels":                  false,
		"word/document.xml":            false,
		"word/styles.xml":              false,
		"word/_rels/document.xml.rels": false,
	}
	for _, f := range r.File {
		if _, ok := required[f.Name]; ok {
			required[f.Name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Errorf("missing required file in docx: %s", name)
		}
	}

	// Verify document.xml contains content
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			content := string(data)
			if !strings.Contains(content, "测试报告") {
				t.Error("document.xml missing title")
			}
			if !strings.Contains(content, "概述") {
				t.Error("document.xml missing heading")
			}
			if !strings.Contains(content, "要点一") {
				t.Error("document.xml missing list item")
			}
		}
	}
}

func TestDocxAccessDenied(t *testing.T) {
	skill := NewDocxCreateSkill([]string{t.TempDir()})
	_, err := skill.Execute(context.Background(), map[string]any{
		"path":    "/etc/evil.docx",
		"content": "test",
	}, nil)
	if err == nil {
		t.Fatal("expected access denied error")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestXlsxCreate(t *testing.T) {
	dir := t.TempDir()
	skill := NewXlsxCreateSkill([]string{dir})

	result, err := skill.Execute(context.Background(), map[string]any{
		"path": filepath.Join(dir, "report.xlsx"),
		"data": `名称,数量,单价,总计
苹果,10,5.5,55
香蕉,20,3.0,60
橙子,15,4.0,60`,
		"sheet_name": "销售数据",
	}, nil)
	if err != nil {
		t.Fatalf("xlsx_create failed: %v", err)
	}
	if !strings.Contains(result, ".xlsx") {
		t.Errorf("result should mention .xlsx: %s", result)
	}
	if !strings.Contains(result, "4行") {
		t.Errorf("result should mention 4 rows: %s", result)
	}

	// Verify it's a valid zip with xlsx structure
	r, err := zip.OpenReader(filepath.Join(dir, "report.xlsx"))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	defer r.Close()

	required := map[string]bool{
		"[Content_Types].xml":      false,
		"xl/workbook.xml":          false,
		"xl/worksheets/sheet1.xml": false,
		"xl/sharedStrings.xml":     false,
		"xl/styles.xml":            false,
	}
	for _, f := range r.File {
		if _, ok := required[f.Name]; ok {
			required[f.Name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Errorf("missing required file in xlsx: %s", name)
		}
	}

	// Verify sheet contains data
	for _, f := range r.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			content := string(data)
			if !strings.Contains(content, "<row") {
				t.Error("sheet1.xml missing rows")
			}
		}
	}

	// Verify shared strings
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			content := string(data)
			if !strings.Contains(content, "苹果") {
				t.Error("sharedStrings.xml missing data")
			}
		}
	}

	// Verify workbook has sheet name
	for _, f := range r.File {
		if f.Name == "xl/workbook.xml" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			content := string(data)
			if !strings.Contains(content, "销售数据") {
				t.Error("workbook.xml missing sheet name")
			}
		}
	}
}

func TestXlsxAccessDenied(t *testing.T) {
	skill := NewXlsxCreateSkill([]string{t.TempDir()})
	_, err := skill.Execute(context.Background(), map[string]any{
		"path": "/etc/evil.xlsx",
		"data": "a,b\n1,2",
	}, nil)
	if err == nil {
		t.Fatal("expected access denied error")
	}
}

func TestXlsxDefaultSheetName(t *testing.T) {
	dir := t.TempDir()
	skill := NewXlsxCreateSkill([]string{dir})

	_, err := skill.Execute(context.Background(), map[string]any{
		"path": filepath.Join(dir, "default.xlsx"),
		"data": "col1,col2\nval1,val2",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify default sheet name
	r, _ := zip.OpenReader(filepath.Join(dir, "default.xlsx"))
	defer r.Close()
	for _, f := range r.File {
		if f.Name == "xl/workbook.xml" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			if !strings.Contains(string(data), "Sheet1") {
				t.Error("default sheet name should be Sheet1")
			}
		}
	}
}

func TestColRef(t *testing.T) {
	tests := []struct {
		col  int
		want string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{701, "ZZ"},
	}
	for _, tt := range tests {
		got := colRef(tt.col)
		if got != tt.want {
			t.Errorf("colRef(%d) = %s, want %s", tt.col, got, tt.want)
		}
	}
}

func TestParseDocContent(t *testing.T) {
	paras := parseDocContent("Title", "# H1\n## H2\n- Item\nNormal text")
	if len(paras) != 5 {
		t.Fatalf("expected 5 paragraphs, got %d", len(paras))
	}
	if paras[0].Style != "Title" {
		t.Errorf("first paragraph should be Title, got %s", paras[0].Style)
	}
	if paras[1].Style != "Heading1" {
		t.Errorf("second paragraph should be Heading1, got %s", paras[1].Style)
	}
	if paras[2].Style != "Heading2" {
		t.Errorf("third paragraph should be Heading2, got %s", paras[2].Style)
	}
	if paras[3].Style != "ListBullet" {
		t.Errorf("fourth paragraph should be ListBullet, got %s", paras[3].Style)
	}
	if paras[4].Style != "Normal" {
		t.Errorf("fifth paragraph should be Normal, got %s", paras[4].Style)
	}
}

func init() {
	_ = os.MkdirAll("data", 0755)
}
