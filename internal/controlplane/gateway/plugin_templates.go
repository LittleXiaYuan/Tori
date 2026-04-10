package gateway

// plugin_templates.go — Generates starter handler files for new plugins.
// Extracted from handlers_admin.go to keep handler files focused on HTTP logic.

import (
	"fmt"
	"strings"
)

// sanitizePluginName makes a name safe for use as directory name.
func sanitizePluginName(name string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if safe == "" {
		safe = "plugin"
	}
	return safe
}

// pluginBoilerplate generates a starter handler file for the given language and template.
func pluginBoilerplate(lang, pluginName, template string) (filename, code string) {
	// Template-specific Python boilerplate
	if lang == "python" && template != "" && template != "custom" {
		return "handler.py", pythonTemplateCode(pluginName, template)
	}

	switch lang {
	case "python":
		return "handler.py", fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s

Arguments are passed via:
  - stdin (JSON)
  - env var PLUGIN_ARGS (JSON)
  - env var PLUGIN_SKILL (skill name)

Print your result to stdout.
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    user_input = args.get("input", "")

    # --- Your logic here ---
    result = f"Processed: {user_input}"

    print(result)

if __name__ == "__main__":
    main()
`, pluginName)

	case "node":
		if template == "node_tool" {
			return "handler.js", fmt.Sprintf(`#!/usr/bin/env node
/**
 * Plugin: %s
 *
 * Node.js tool plugin using npm ecosystem.
 * Install dependencies: npm init -y && npm install axios cheerio
 */
const axios = require('axios');

let data = '';
process.stdin.on('data', chunk => { data += chunk; });
process.stdin.on('end', async () => {
  try {
    const args = JSON.parse(data);
    const url = args.url || args.input || '';

    // Example: HTTP GET request
    const response = await axios.get(url, { timeout: 10000 });
    const result = {
      status: response.status,
      content_length: response.data.length,
      preview: typeof response.data === 'string'
        ? response.data.slice(0, 500)
        : JSON.stringify(response.data).slice(0, 500),
    };

    console.log(JSON.stringify(result));
  } catch (err) {
    console.error(JSON.stringify({ error: err.message }));
    process.exit(1);
  }
});
`, pluginName)
		}
		return "handler.js", fmt.Sprintf(`#!/usr/bin/env node
/**
 * Plugin: %s
 *
 * Arguments arrive via stdin (JSON) and env PLUGIN_ARGS.
 * Print your result to stdout.
 */
let data = '';
process.stdin.on('data', chunk => { data += chunk; });
process.stdin.on('end', () => {
  const args = JSON.parse(data);
  const input = args.input || '';

  // --- Your logic here ---
  const result = 'Processed: ' + input;

  console.log(result);
});
`, pluginName)

	case "shell":
		return "handler.sh", fmt.Sprintf(`#!/bin/sh
# Plugin: %s
# Arguments come via stdin (JSON) and env $PLUGIN_ARGS

read INPUT

# --- Your logic here ---
echo "Processed: $INPUT"
`, pluginName)

	default:
		return "handler.py", "# Unsupported language, using Python template\nimport sys\nprint(sys.stdin.read())\n"
	}
}

// pythonTemplateCode generates template-specific Python boilerplate.
func pythonTemplateCode(pluginName, template string) string {
	switch template {
	case "word_doc":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — Word Document Processing

Dependencies: pip install python-docx
"""
import json
import sys
from pathlib import Path

def main():
    args = json.loads(sys.stdin.read())
    action = args.get("action", "create")  # create, read, modify
    filepath = args.get("filepath", "output.docx")
    content = args.get("content", "")

    try:
        from docx import Document
    except ImportError:
        print(json.dumps({"error": "python-docx not installed. Run: pip install python-docx"}))
        return

    if action == "create":
        doc = Document()
        doc.add_heading(args.get("title", "Document"), level=1)
        for paragraph in content.split("\n"):
            if paragraph.strip():
                doc.add_paragraph(paragraph.strip())
        doc.save(filepath)
        print(json.dumps({"status": "created", "filepath": filepath}))

    elif action == "read":
        doc = Document(filepath)
        text = "\n".join([p.text for p in doc.paragraphs])
        print(json.dumps({"text": text, "paragraphs": len(doc.paragraphs)}))

    elif action == "modify":
        doc = Document(filepath)
        doc.add_paragraph(content)
        doc.save(filepath)
        print(json.dumps({"status": "modified", "filepath": filepath}))

    else:
        print(json.dumps({"error": f"Unknown action: {action}"}))

if __name__ == "__main__":
    main()
`, pluginName)

	case "excel":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — Excel Spreadsheet Processing

Dependencies: pip install openpyxl
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    action = args.get("action", "create")  # create, read, modify
    filepath = args.get("filepath", "output.xlsx")

    try:
        from openpyxl import Workbook, load_workbook
    except ImportError:
        print(json.dumps({"error": "openpyxl not installed. Run: pip install openpyxl"}))
        return

    if action == "create":
        wb = Workbook()
        ws = wb.active
        ws.title = args.get("sheet_name", "Sheet1")
        headers = args.get("headers", [])
        rows = args.get("rows", [])
        if headers:
            ws.append(headers)
        for row in rows:
            ws.append(row)
        wb.save(filepath)
        print(json.dumps({"status": "created", "filepath": filepath, "rows": len(rows)}))

    elif action == "read":
        wb = load_workbook(filepath)
        ws = wb.active
        data = []
        for row in ws.iter_rows(values_only=True):
            data.append(list(row))
        print(json.dumps({"sheet": ws.title, "rows": len(data), "data": data[:50]}))

    elif action == "modify":
        wb = load_workbook(filepath)
        ws = wb.active
        row_data = args.get("row", [])
        ws.append(row_data)
        wb.save(filepath)
        print(json.dumps({"status": "modified", "total_rows": ws.max_row}))

    else:
        print(json.dumps({"error": f"Unknown action: {action}"}))

if __name__ == "__main__":
    main()
`, pluginName)

	case "api_call":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — REST API Caller

Dependencies: pip install requests
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    url = args.get("url", "")
    method = args.get("method", "GET").upper()
    headers = args.get("headers", {})
    body = args.get("body")

    if not url:
        print(json.dumps({"error": "url is required"}))
        return

    try:
        import requests
    except ImportError:
        print(json.dumps({"error": "requests not installed. Run: pip install requests"}))
        return

    try:
        resp = requests.request(
            method, url,
            headers=headers,
            json=body if body else None,
            timeout=30,
        )
        result = {
            "status_code": resp.status_code,
            "headers": dict(resp.headers),
            "body": resp.text[:2000],
        }
        try:
            result["json"] = resp.json()
        except ValueError:
            pass
        print(json.dumps(result))
    except requests.RequestException as e:
        print(json.dumps({"error": str(e)}))

if __name__ == "__main__":
    main()
`, pluginName)

	case "data_analysis":
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s — Data Analysis

Dependencies: pip install pandas
"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    action = args.get("action", "describe")  # describe, filter, aggregate
    filepath = args.get("filepath", "")
    data_inline = args.get("data")

    try:
        import pandas as pd
    except ImportError:
        print(json.dumps({"error": "pandas not installed. Run: pip install pandas"}))
        return

    # Load data
    if filepath:
        if filepath.endswith(".csv"):
            df = pd.read_csv(filepath)
        elif filepath.endswith(".xlsx"):
            df = pd.read_excel(filepath)
        elif filepath.endswith(".json"):
            df = pd.read_json(filepath)
        else:
            print(json.dumps({"error": f"Unsupported file format: {filepath}"}))
            return
    elif data_inline:
        df = pd.DataFrame(data_inline)
    else:
        print(json.dumps({"error": "filepath or data is required"}))
        return

    if action == "describe":
        desc = df.describe(include="all").to_dict()
        result = {
            "shape": list(df.shape),
            "columns": list(df.columns),
            "dtypes": {k: str(v) for k, v in df.dtypes.items()},
            "describe": desc,
            "head": df.head(5).to_dict(orient="records"),
        }
    elif action == "filter":
        column = args.get("column", "")
        value = args.get("value")
        op = args.get("op", "eq")
        if op == "eq":
            filtered = df[df[column] == value]
        elif op == "gt":
            filtered = df[df[column] > value]
        elif op == "lt":
            filtered = df[df[column] < value]
        elif op == "contains":
            filtered = df[df[column].astype(str).str.contains(str(value), na=False)]
        else:
            filtered = df
        result = {"rows": len(filtered), "data": filtered.head(50).to_dict(orient="records")}
    elif action == "aggregate":
        group_by = args.get("group_by", "")
        agg_col = args.get("agg_column", "")
        agg_func = args.get("agg_func", "sum")
        grouped = df.groupby(group_by)[agg_col].agg(agg_func).reset_index()
        result = {"data": grouped.to_dict(orient="records")}
    else:
        result = {"error": f"Unknown action: {action}"}

    print(json.dumps(result, default=str))

if __name__ == "__main__":
    main()
`, pluginName)

	default:
		return fmt.Sprintf(`#!/usr/bin/env python3
"""Plugin: %s"""
import json
import sys

def main():
    args = json.loads(sys.stdin.read())
    user_input = args.get("input", "")
    result = f"Processed: {user_input}"
    print(result)

if __name__ == "__main__":
    main()
`, pluginName)
	}
}
