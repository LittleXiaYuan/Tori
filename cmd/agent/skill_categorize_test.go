package main

import "testing"

func TestCategorizeSkillName(t *testing.T) {
	cases := map[string]string{
		// file
		"docx_create":    "file",
		"xlsx_split":     "file",
		"pptx_fill":      "file",
		"pdf_create":     "file",
		"file_open":      "file",
		"file_generate":  "file",
		"zip_pack":       "file",
		"deck_create":    "file",
		"html_export":    "file",
		"document_parse": "file",
		// image
		"image_generate": "image",
		// research (web_* fetch/search, deep research, translate)
		"deep_research": "research",
		"web_search":    "research",
		"web_fetch":     "research",
		"translate":     "research",
		// workflow
		"run_workflow":     "workflow",
		"save_as_workflow": "workflow",
		"orchestrate_task": "workflow",
		// uncategorized (general tools must stay always-available)
		"code_execute": "",
		"computer_use": "",
		"send_email":   "",
		// browser/connector handled separately → not claimed here
		"browser_click":          "",
		"connector_github_issue": "",
	}
	for name, want := range cases {
		if got := categorizeSkillName(name); got != want {
			t.Errorf("categorizeSkillName(%q) = %q, want %q", name, got, want)
		}
	}
}
