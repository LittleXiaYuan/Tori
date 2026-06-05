package general

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

// DeckCreateSkill generates an original, professionally-designed slide deck.
//
// Two-phase, multi-style design:
//  1. The LLM produces a compact JSON spec (per-slide content, icons, charts,
//     and a named visual style) — small and fast, so it never times out.
//  2. Go renders that spec into one of several proven, print-correct HTML design
//     systems (inline-SVG icons + SVG charts + decorative meshes, no images
//     needed) and rasterizes it to PDF via a headless Chromium browser.
//
// Result: Gamma/Kimi-grade visuals with the LLM in control of content and
// palette, a render that always works headless, and zero user dependencies
// (no Docker, no Python, no external services — just a browser already present).
type DeckCreateSkill struct {
	allowedReadDirs  []string
	allowedWriteDirs []string
}

func NewDeckCreateSkill(readDirs, writeDirs []string) *DeckCreateSkill {
	return &DeckCreateSkill{allowedReadDirs: readDirs, allowedWriteDirs: writeDirs}
}

func (s *DeckCreateSkill) Name() string { return "deck_create" }

func (s *DeckCreateSkill) Description() string {
	return `生成"原创设计"的高质量演示文稿(PDF)。模型规划每页内容/图标/图表与配色,由内置品牌级设计系统
渲染为 16:9 PDF(无头浏览器),含 SVG 图标/环形条形图/装饰,4 种风格(aurora科技/clean商务/sunset暖潮/mono杂志),
观感对标 Gamma/Kimi,用户零依赖(无需 Docker/Python)。用户要"做一份好看的 PPT/汇报/路演"时优先用它。`
}

func (s *DeckCreateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":     map[string]any{"type": "string", "description": "输出 PDF 路径(如 data/output/deck.pdf)"},
			"brief":    map[string]any{"type": "string", "description": "演示主题与核心要点/素材(越具体越好;可含已检索到的事实)。"},
			"title":    map[string]any{"type": "string", "description": "封面主标题(可选)"},
			"slides":   map[string]any{"type": "integer", "description": "目标页数(可选,默认约 10)"},
			"style":    map[string]any{"type": "string", "description": "风格(可选): aurora(科技深色) | clean(商务简约) | sunset(暖色潮流) | mono(杂志简约)。留空则模型按主题自选。"},
			"language": map[string]any{"type": "string", "description": "语言(可选,默认 中文)"},
			"images":   map[string]any{"type": "string", "description": "可选:配图。可填【一个目录】(自动收集其中图片,常配合 zip_unpack 解压目录)或【逗号分隔的图片路径】。支持 jpg/png/webp/gif。模型会按序号把图片放进封面背景/图文分栏/全幅/画廊版式。"},
		},
		"required": []string{"path", "brief"},
	}
}

func (s *DeckCreateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	pathStr, _ := args["path"].(string)
	brief, _ := args["brief"].(string)
	title, _ := args["title"].(string)
	style, _ := args["style"].(string)
	language, _ := args["language"].(string)
	imagesArg, _ := args["images"].(string)
	nSlides := asInt(args["slides"])

	if pathStr == "" || brief == "" {
		return "", fmt.Errorf("path and brief are required")
	}
	if env == nil || env.LLMCall == nil {
		return "", fmt.Errorf("deck_create requires an LLM (env.LLMCall not available)")
	}
	if language == "" {
		language = "中文"
	}
	if nSlides <= 0 {
		nSlides = 10
	}
	if !strings.EqualFold(filepath.Ext(pathStr), ".pdf") {
		pathStr = strings.TrimSuffix(pathStr, filepath.Ext(pathStr)) + ".pdf"
	}
	absPath, err := filepath.Abs(pathStr)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if !isUnderAllowed(absPath, s.allowedWriteDirs) {
		return "", fmt.Errorf("access denied: path %s is not under allowed directories", pathStr)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}

	imgs := s.collectImages(imagesArg)

	raw, err := env.LLMCall(ctx, deckSpecSystemPrompt(nSlides, language, style, imageManifest(imgs)), deckSpecUserPrompt(title, brief))
	if err != nil {
		return "", fmt.Errorf("deck planning (LLM) failed: %w", err)
	}
	spec, err := parseDeckSpec(raw)
	if err != nil {
		return "", fmt.Errorf("deck spec parse failed: %w", err)
	}
	if style != "" {
		spec.Style = style
	}
	if len(spec.Slides) == 0 {
		return "", fmt.Errorf("LLM returned no slides")
	}

	deckHTML := renderDeckHTML(spec, imgs)

	stage, err := os.MkdirTemp("", "yqdeck")
	if err != nil {
		return "", fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(stage)
	htmlPath := filepath.Join(stage, "deck.html")
	pdfTmp := filepath.Join(stage, "deck.pdf")
	if err := os.WriteFile(htmlPath, []byte(deckHTML), 0644); err != nil {
		return "", fmt.Errorf("write html: %w", err)
	}

	browser, err := findChromium()
	if err != nil {
		return "", err
	}
	rctx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		rctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(rctx, browser,
		"--headless=new", "--disable-gpu", "--no-pdf-header-footer",
		"--user-data-dir="+filepath.Join(stage, "cdp"),
		"--print-to-pdf="+pdfTmp,
		fileURL(htmlPath),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("render failed (%s): %v: %s", filepath.Base(browser), err, lastLine(string(out)))
	}
	info, err := os.Stat(pdfTmp)
	if err != nil || info.Size() < 2048 {
		return "", fmt.Errorf("render produced empty PDF (check headless browser)")
	}
	if err := copyFile(pdfTmp, absPath); err != nil {
		return "", fmt.Errorf("save pdf: %w", err)
	}
	size := info.Size()
	if fi, e := os.Stat(absPath); e == nil {
		size = fi.Size()
	}
	return fmt.Sprintf("已生成原创设计演示文稿(PDF): %s (%d bytes, %d 张, 风格=%s, 渲染器=%s)",
		pathStr, size, len(spec.Slides), spec.Style, filepath.Base(browser)), nil
}

// ---- spec ----

type deckSpec struct {
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle"`
	Style    string      `json:"style"`
	Footer   string      `json:"footer"`
	Slides   []deckSlide `json:"slides"`
}

type deckSlide struct {
	Type     string     `json:"type"`
	Kicker   string     `json:"kicker"`
	Title    string     `json:"title"`
	Subtitle string     `json:"subtitle"`
	Num      string     `json:"num"`
	Desc     string     `json:"desc"`
	Note     string     `json:"note"`
	Icon     string     `json:"icon"`
	Quote    string     `json:"quote"`
	By       string     `json:"by"`
	Badges   []string   `json:"badges"`
	Bullets  []string   `json:"bullets"`
	Cards    []deckCard `json:"cards"`
	Stats    []deckStat `json:"stats"`
	Steps    []deckStep `json:"steps"`
	Bars     []deckBar  `json:"bars"`
	Left     *deckCol   `json:"left"`
	Right    *deckCol   `json:"right"`
	Image    int        `json:"image"`     // 1-based index into the provided images; 0 = none
	Images   []int      `json:"images"`    // for gallery: list of 1-based indices
	ImgRight bool       `json:"imgRight"`  // imagesplit: place image on the right (default left)
	Caption  string     `json:"caption"`   // image caption (imagefull/imagesplit)
}

type deckCard struct {
	Icon  string `json:"icon"`
	Title string `json:"title"`
	Text  string `json:"text"`
	Tag   string `json:"tag"`
}
type deckStat struct {
	N   string `json:"n"`
	L   string `json:"l"`
	Pct int    `json:"pct"` // 0-100; >0 renders a ring chart
}
type deckStep struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}
type deckBar struct {
	Label string `json:"label"`
	Value int    `json:"value"` // 0-100
}
type deckCol struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

func parseDeckSpec(raw string) (*deckSpec, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}
	if i := strings.LastIndex(raw, "}"); i >= 0 {
		raw = raw[:i+1]
	}
	var spec deckSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return nil, err
	}
	if spec.Footer == "" {
		spec.Footer = "云雀 Yunque"
	}
	return &spec, nil
}

// ---- images ----

type deckImage struct {
	Name    string
	DataURI string
}

var imgExts = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".webp": "image/webp", ".gif": "image/gif",
}

const (
	maxDeckImages  = 12
	maxImageBytes  = 8 << 20  // 8MB per image
	maxImagesTotal = 28 << 20 // 28MB total embedded
)

// collectImages accepts a single directory (auto-scanned, e.g. a zip_unpack
// output dir) or a comma-separated list of image paths, validates them against
// the allowed read dirs, and base64-embeds each (so the headless render never
// hits file-path/permission issues).
func (s *DeckCreateSkill) collectImages(arg string) []deckImage {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil
	}
	var paths []string
	if abs, err := filepath.Abs(arg); err == nil {
		if fi, e := os.Stat(abs); e == nil && fi.IsDir() {
			entries, _ := os.ReadDir(abs)
			for _, en := range entries {
				if en.IsDir() {
					continue
				}
				if _, ok := imgExts[strings.ToLower(filepath.Ext(en.Name()))]; ok {
					paths = append(paths, filepath.Join(abs, en.Name()))
				}
			}
			sort.Strings(paths)
		}
	}
	if len(paths) == 0 {
		for _, p := range strings.Split(arg, ",") {
			if p = strings.TrimSpace(p); p != "" {
				paths = append(paths, p)
			}
		}
	}

	var out []deckImage
	total := 0
	for _, p := range paths {
		if len(out) >= maxDeckImages {
			break
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		mime, ok := imgExts[strings.ToLower(filepath.Ext(abs))]
		if !ok {
			continue
		}
		if !isUnderAllowed(abs, s.allowedReadDirs) {
			continue
		}
		fi, err := os.Stat(abs)
		if err != nil || fi.IsDir() || fi.Size() > maxImageBytes {
			continue
		}
		if total+int(fi.Size()) > maxImagesTotal {
			break
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		total += len(data)
		out = append(out, deckImage{
			Name:    filepath.Base(abs),
			DataURI: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
		})
	}
	return out
}

func imageManifest(imgs []deckImage) string {
	if len(imgs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, im := range imgs {
		if i > 0 {
			b.WriteString("、")
		}
		fmt.Fprintf(&b, "%d=%s", i+1, im.Name)
	}
	return b.String()
}

// imgURI returns the data URI for a 1-based index, or "" if out of range.
func imgURI(imgs []deckImage, idx int) string {
	if idx >= 1 && idx <= len(imgs) {
		return imgs[idx-1].DataURI
	}
	return ""
}

// ---- styles ----

// each style is a set of CSS custom properties appended after the base CSS.
var deckStyleVars = map[string]string{
	"aurora": `--bg:#0b1020;--ink:#f4f6fc;--muted:#9aa6c0;--soft:rgba(255,255,255,.05);--bd:rgba(255,255,255,.11);
--a1:#6d5efc;--a2:#22d3ee;--a3:#a855f7;--cover:#0b1020;--rad:18px`,
	"clean": `--bg:#ffffff;--ink:#0f172a;--muted:#5b6678;--soft:#f6f8fc;--bd:#e6e9f1;
--a1:#2563eb;--a2:#0ea5e9;--a3:#7c3aed;--cover:#0b1230;--rad:16px`,
	"sunset": `--bg:#fff8f3;--ink:#2a1a12;--muted:#8a6f60;--soft:#ffffff;--bd:#f1e3d8;
--a1:#f97316;--a2:#ef4444;--a3:#f59e0b;--cover:#2a160e;--rad:22px`,
	"mono": `--bg:#fafafa;--ink:#0a0a0a;--muted:#6b6b6b;--soft:#ffffff;--bd:#e6e6e6;
--a1:#111111;--a2:#3f3f46;--a3:#e11d48;--cover:#0a0a0a;--rad:10px`,
}

func styleVars(name string) (string, string) {
	n := strings.ToLower(strings.TrimSpace(name))
	if v, ok := deckStyleVars[n]; ok {
		return n, v
	}
	return "aurora", deckStyleVars["aurora"]
}

// ---- render ----

func renderDeckHTML(spec *deckSpec, imgs []deckImage) string {
	styleName, vars := styleVars(spec.Style)
	spec.Style = styleName
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang=\"zh-CN\"><head><meta charset=\"UTF-8\"><title>")
	b.WriteString(html.EscapeString(spec.Title))
	b.WriteString("</title><style>")
	b.WriteString(deckBaseCSS)
	b.WriteString(":root{")
	b.WriteString(vars)
	b.WriteString("}</style></head><body class=\"st-")
	b.WriteString(styleName)
	b.WriteString("\">")
	for _, sl := range spec.Slides {
		b.WriteString(renderSlide(spec, sl, imgs))
	}
	b.WriteString("</body></html>")
	return b.String()
}

func renderSlide(spec *deckSpec, sl deckSlide, imgs []deckImage) string {
	foot := fmt.Sprintf(`<div class="pagefoot"><span>%s</span><span class="brand"><span class="dot"></span>%s</span></div>`,
		esc(spec.Title), esc(spec.Footer))
	head := func() string {
		k := orDefault(sl.Kicker, spec.Title)
		return fmt.Sprintf(`<div class="head"><span class="kicker">%s</span><h2>%s</h2>%s</div>`,
			esc(k), esc(sl.Title), optP(sl.Subtitle, "subt"))
	}
	deco := `<svg class="deco" viewBox="0 0 1280 720" preserveAspectRatio="none"><defs><radialGradient id="g" cx="85%" cy="12%" r="55%"><stop offset="0" stop-color="var(--a1)" stop-opacity=".18"/><stop offset="1" stop-color="var(--a1)" stop-opacity="0"/></radialGradient><radialGradient id="g2" cx="8%" cy="92%" r="45%"><stop offset="0" stop-color="var(--a2)" stop-opacity=".14"/><stop offset="1" stop-color="var(--a2)" stop-opacity="0"/></radialGradient></defs><rect width="1280" height="720" fill="url(#g)"/><rect width="1280" height="720" fill="url(#g2)"/></svg>`

	switch strings.ToLower(sl.Type) {
	case "cover":
		badges := ""
		for _, bd := range sl.Badges {
			badges += `<span class="pill">` + esc(bd) + `</span>`
		}
		cls, bg := "slide cover", ""
		if uri := imgURI(imgs, sl.Image); uri != "" {
			cls = "slide cover hasbg"
			bg = `<img class="coverbg" src="` + uri + `"/><div class="cshade"></div>`
		}
		return fmt.Sprintf(`<section class="%s">%s<svg class="cdeco" viewBox="0 0 1280 720" preserveAspectRatio="none"><defs><radialGradient id="cg" cx="80%%" cy="6%%" r="60%%"><stop offset="0" stop-color="var(--a1)" stop-opacity=".55"/><stop offset="1" stop-color="var(--a1)" stop-opacity="0"/></radialGradient><radialGradient id="cg2" cx="6%%" cy="110%%" r="55%%"><stop offset="0" stop-color="var(--a2)" stop-opacity=".40"/><stop offset="1" stop-color="var(--a2)" stop-opacity="0"/></radialGradient></defs><rect width="1280" height="720" fill="url(#cg)"/><rect width="1280" height="720" fill="url(#cg2)"/></svg><div class="cwrap"><span class="kicker">%s</span><h1>%s</h1><div class="rule"></div><p class="sub">%s</p><div class="meta">%s</div></div></section>`,
			cls, bg, esc(orDefault(sl.Kicker, "YUNQUE INSIGHT")), esc(orDefault(sl.Title, spec.Title)), esc(orDefault(sl.Subtitle, spec.Subtitle)), badges)
	case "section":
		return fmt.Sprintf(`<section class="slide divider">%s<div class="dwrap"><span class="kicker">SECTION %s</span><div class="num">%s</div><h2>%s</h2><p>%s</p></div></section>`,
			deco, esc(sl.Num), esc(orDefault(sl.Num, "·")), esc(sl.Title), esc(sl.Desc))
	case "cards":
		gcls := "g3"
		if n := len(sl.Cards); n == 2 || n == 4 {
			gcls = "g2"
		}
		cards := ""
		for _, c := range sl.Cards {
			tag := ""
			if c.Tag != "" {
				tag = `<span class="tag">` + esc(c.Tag) + `</span>`
			}
			cards += fmt.Sprintf(`<div class="card"><div class="ic">%s</div><h3>%s</h3><p>%s</p>%s</div>`,
				iconSVG(c.Icon), esc(c.Title), esc(c.Text), tag)
		}
		return fmt.Sprintf(`<section class="slide">%s%s<div class="body"><div class="grid %s">%s</div></div>%s</section>`, deco, head(), gcls, cards, foot)
	case "cols":
		col := func(c *deckCol, cls string) string {
			if c == nil {
				return ""
			}
			items := ""
			for _, it := range c.Items {
				items += "<li>" + esc(it) + "</li>"
			}
			return fmt.Sprintf(`<div class="col %s"><h3>%s</h3><ul>%s</ul></div>`, cls, esc(c.Title), items)
		}
		return fmt.Sprintf(`<section class="slide">%s%s<div class="body"><div class="cols">%s%s</div></div>%s</section>`, deco, head(), col(sl.Left, "a"), col(sl.Right, "b"), foot)
	case "stats":
		st := ""
		for _, s := range sl.Stats {
			if s.Pct > 0 {
				st += fmt.Sprintf(`<div class="stat ring">%s<div class="l">%s</div></div>`, ringSVG(s.Pct, s.N), esc(s.L))
			} else {
				st += fmt.Sprintf(`<div class="stat"><div class="n">%s</div><div class="l">%s</div></div>`, esc(s.N), esc(s.L))
			}
		}
		note := optP(sl.Note, "note")
		return fmt.Sprintf(`<section class="slide">%s%s<div class="body"><div class="stats">%s</div>%s</div>%s</section>`, deco, head(), st, note, foot)
	case "bars":
		return fmt.Sprintf(`<section class="slide">%s%s<div class="body"><div class="barwrap">%s</div>%s</div>%s</section>`, deco, head(), barsSVG(sl.Bars), optP(sl.Note, "note"), foot)
	case "steps":
		st := ""
		for i, s := range sl.Steps {
			st += fmt.Sprintf(`<div class="step"><div class="si">%d</div><div><h3>%s</h3><p>%s</p></div></div>`, i+1, esc(s.Title), esc(s.Text))
		}
		return fmt.Sprintf(`<section class="slide">%s%s<div class="body"><div class="steps">%s</div></div>%s</section>`, deco, head(), st, foot)
	case "hero":
		return fmt.Sprintf(`<section class="slide hero">%s<div class="hwrap"><span class="kicker">%s</span><h1 class="big">%s</h1><p class="lead">%s</p></div></section>`,
			deco, esc(sl.Kicker), esc(sl.Title), esc(sl.Subtitle))
	case "quote":
		return fmt.Sprintf(`<section class="slide quote">%s<div class="qwrap"><div class="qmark">"</div><blockquote>%s</blockquote><div class="by">%s</div></div></section>`,
			deco, esc(orDefault(sl.Quote, sl.Title)), esc(sl.By))
	case "imagesplit", "split":
		lis := ""
		for _, it := range sl.Bullets {
			lis += "<li>" + esc(it) + "</li>"
		}
		inner := ""
		if lis != "" {
			inner = `<ul class="blist sm">` + lis + `</ul>`
		} else if sl.Desc != "" {
			inner = `<p class="lead">` + esc(sl.Desc) + `</p>`
		}
		imgblk := `<div class="splitimg ph"><span>` + esc(orDefault(sl.Caption, "图片")) + `</span></div>`
		if uri := imgURI(imgs, sl.Image); uri != "" {
			imgblk = `<div class="splitimg"><img src="` + uri + `"/></div>`
		}
		text := `<div class="sptext">` + head() + inner + `</div>`
		left, right := imgblk, text
		if sl.ImgRight {
			left, right = text, imgblk
		}
		return fmt.Sprintf(`<section class="slide split">%s<div class="splitwrap">%s%s</div>%s</section>`, deco, left, right, foot)
	case "imagefull", "fullimage":
		uri := imgURI(imgs, sl.Image)
		if uri == "" {
			return fmt.Sprintf(`<section class="slide">%s%s<div class="body"><p class="lead">%s</p></div>%s</section>`, deco, head(), esc(orDefault(sl.Desc, sl.Caption)), foot)
		}
		capbar := ""
		if strings.TrimSpace(sl.Title) != "" || strings.TrimSpace(sl.Caption) != "" {
			capbar = `<div class="capbar"><h2>` + esc(sl.Title) + `</h2>` + optP(sl.Caption, "cap") + `</div>`
		}
		return fmt.Sprintf(`<section class="slide imagefull"><img class="ffimg" src="%s"/><div class="ffshade"></div>%s</section>`, uri, capbar)
	case "gallery":
		idxs := sl.Images
		if len(idxs) == 0 && sl.Image > 0 {
			idxs = []int{sl.Image}
		}
		cells := ""
		for _, ix := range idxs {
			if uri := imgURI(imgs, ix); uri != "" {
				cells += `<div class="gcell"><img src="` + uri + `"/></div>`
			}
		}
		gcls := "g3"
		if n := len(idxs); n == 2 || n == 4 {
			gcls = "g2"
		}
		return fmt.Sprintf(`<section class="slide">%s%s<div class="body"><div class="gallery %s">%s</div></div>%s</section>`, deco, head(), gcls, cells, foot)
	case "closing":
		return fmt.Sprintf(`<section class="slide cover closing"><svg class="cdeco" viewBox="0 0 1280 720" preserveAspectRatio="none"><defs><radialGradient id="xg" cx="50%%" cy="115%%" r="60%%"><stop offset="0" stop-color="var(--a1)" stop-opacity=".5"/><stop offset="1" stop-color="var(--a1)" stop-opacity="0"/></radialGradient></defs><rect width="1280" height="720" fill="url(#xg)"/></svg><div class="cwrap" style="text-align:center;align-items:center"><span class="kicker">%s</span><h1>%s</h1><p class="sub">%s</p></div></section>`,
			esc(orDefault(sl.Kicker, "谢谢观看")), esc(orDefault(sl.Title, "谢谢")), esc(sl.Subtitle))
	default: // bullets
		lis := ""
		for _, it := range sl.Bullets {
			lis += "<li>" + esc(it) + "</li>"
		}
		body := ""
		if lis != "" {
			body = `<ul class="blist">` + lis + `</ul>`
		} else if sl.Desc != "" {
			body = `<p class="lead">` + esc(sl.Desc) + `</p>`
		}
		return fmt.Sprintf(`<section class="slide">%s%s<div class="body">%s</div>%s</section>`, deco, head(), body, foot)
	}
}

func optP(s, cls string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return `<p class="` + cls + `">` + esc(s) + `</p>`
}

func esc(s string) string { return html.EscapeString(s) }
func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

// ---- SVG charts ----

// ringSVG draws a donut showing pct (0-100) with center label.
func ringSVG(pct int, center string) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	const r = 52.0
	c := 2 * 3.14159265 * r
	dash := c * float64(pct) / 100.0
	if center == "" {
		center = fmt.Sprintf("%d%%", pct)
	}
	return fmt.Sprintf(`<svg class="ring-svg" viewBox="0 0 140 140"><circle cx="70" cy="70" r="52" fill="none" stroke="var(--bd)" stroke-width="14"/><circle cx="70" cy="70" r="52" fill="none" stroke="url(#rg)" stroke-width="14" stroke-linecap="round" stroke-dasharray="%.1f %.1f" transform="rotate(-90 70 70)"/><defs><linearGradient id="rg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="var(--a1)"/><stop offset="1" stop-color="var(--a2)"/></linearGradient></defs><text x="70" y="78" text-anchor="middle" class="ring-t">%s</text></svg>`,
		dash, c-dash, esc(center))
}

// barsSVG draws a horizontal bar chart (values 0-100).
func barsSVG(bars []deckBar) string {
	if len(bars) == 0 {
		return ""
	}
	var b strings.Builder
	for _, bar := range bars {
		v := bar.Value
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		b.WriteString(fmt.Sprintf(`<div class="bar"><div class="bl">%s</div><div class="bt"><div class="bf" style="width:%d%%"></div></div><div class="bv">%d</div></div>`,
			esc(bar.Label), v, bar.Value))
	}
	return b.String()
}

// ---- inline SVG icons (stroke=currentColor) ----

var deckIcons = map[string]string{
	"brain":    `<path d="M9 3a3 3 0 0 0-3 3 3 3 0 0 0-1 5 3 3 0 0 0 2 5 3 3 0 0 0 5 1V4a3 3 0 0 0-3-1Z"/><path d="M15 3a3 3 0 0 1 3 3 3 3 0 0 1 1 5 3 3 0 0 1-2 5 3 3 0 0 1-5 1"/>`,
	"memory":   `<rect x="4" y="4" width="16" height="16" rx="3"/><path d="M9 9h6M9 13h6M9 17h3"/>`,
	"doc":      `<path d="M7 3h7l5 5v13H7z"/><path d="M14 3v5h5M10 13h6M10 17h6"/>`,
	"slides":   `<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M9 21h6M12 17v4"/>`,
	"table":    `<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18M3 14h18M9 4v16"/>`,
	"shield":   `<path d="M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z"/><path d="M9 12l2 2 4-4"/>`,
	"lock":     `<rect x="5" y="11" width="14" height="9" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>`,
	"route":    `<circle cx="6" cy="6" r="2"/><circle cx="18" cy="18" r="2"/><path d="M8 6h7a3 3 0 0 1 0 6H9a3 3 0 0 0 0 6h7"/>`,
	"chart":    `<path d="M4 20V4M4 20h16"/><rect x="7" y="12" width="3" height="6"/><rect x="12" y="8" width="3" height="10"/><rect x="17" y="5" width="3" height="13"/>`,
	"rocket":   `<path d="M5 15c-1 2-1 4-1 4s2 0 4-1m9-13c-3 0-7 2-10 7l3 3c5-3 7-7 7-10z"/><circle cx="14.5" cy="9.5" r="1.5"/>`,
	"bulb":     `<path d="M9 18h6M10 21h4"/><path d="M12 3a6 6 0 0 0-4 10c1 1 1 2 1 3h6c0-1 0-2 1-3a6 6 0 0 0-4-10Z"/>`,
	"gear":     `<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2"/>`,
	"globe":    `<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18"/>`,
	"users":    `<circle cx="9" cy="8" r="3"/><path d="M3 20c0-3 3-5 6-5s6 2 6 5"/><path d="M16 6a3 3 0 0 1 0 6M17 20c0-2-1-3-2-4"/>`,
	"star":     `<path d="M12 3l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9 6.8 19l1-5.8-4.3-4.1 5.9-.9z"/>`,
	"check":    `<circle cx="12" cy="12" r="9"/><path d="M8 12l3 3 5-6"/>`,
	"search":   `<circle cx="11" cy="11" r="7"/><path d="M21 21l-4-4"/>`,
	"layers":   `<path d="M12 3l9 5-9 5-9-5z"/><path d="M3 13l9 5 9-5M3 17l9 5 9-5"/>`,
	"sparkles": `<path d="M12 4l1.5 4L18 9.5 13.5 11 12 15l-1.5-4L6 9.5 10.5 8z"/><path d="M18 15l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8z"/>`,
	"clock":    `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
}

func iconSVG(name string) string {
	inner, ok := deckIcons[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		inner = deckIcons["sparkles"]
	}
	return `<svg viewBox="0 0 24 24" fill="none" stroke="url(#ig)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><defs><linearGradient id="ig" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="var(--a1)"/><stop offset="1" stop-color="var(--a2)"/></linearGradient></defs>` + inner + `</svg>`
}

func deckSpecSystemPrompt(slides int, language, style, imgManifest string) string {
	styleLine := "style 从这些里选一个最契合主题的:aurora(科技深色)、clean(商务简约)、sunset(暖色潮流)、mono(杂志简约)。"
	if strings.TrimSpace(style) != "" {
		styleLine = "style 固定为:" + style + "。"
	}
	icons := "brain,memory,doc,slides,table,shield,lock,route,chart,rocket,bulb,gear,globe,users,star,check,search,layers,sparkles,clock"
	imgSection := ""
	if strings.TrimSpace(imgManifest) != "" {
		imgSection = fmt.Sprintf(`

【可用配图】(用 image=序号 或 images=[序号...] 引用):%s
请把这些图片自然融入,优先用这些图片版式(用真实图片,别凭空捏造序号):
- {"type":"cover",...,"image":1}  // 封面用整图做背景(自动加深色蒙版,文字清晰)
- {"type":"imagesplit","title":"页标题","bullets":["要点","要点","要点"],"image":2,"imgRight":true,"caption":"图注"}  // 图文分栏,imgRight=true 时图在右
- {"type":"imagefull","image":3,"title":"叠加标题","caption":"图注"}  // 整幅大图+底部标题条
- {"type":"gallery","title":"页标题","images":[2,3,4]}  // 图片画廊(2-6 张)
没有合适图片的页就别硬塞图片,正常用文字/图表版式。`, imgManifest)
	}
	return fmt.Sprintf(`你是顶尖演示文稿策划。为给定主题规划约 %d 页、%s 的幻灯片,"只输出 JSON"(不要解释、不要代码围栏)。

%s

JSON: {"title","subtitle","style","footer":"云雀 Yunque","slides":[...]}

slide 类型(混用以制造节奏;每页信息要"填满但不挤",避免大片留白):
- {"type":"cover","kicker":"小标签","title":"主标题","subtitle":"一句话","badges":["要点","要点","要点"]}
- {"type":"section","num":"01","title":"章节名","desc":"一句话引言"}
- {"type":"hero","kicker":"小标签","title":"一句有冲击力的核心主张","subtitle":"一句支撑"}
- {"type":"cards","title":"页标题","subtitle":"可选副标题","cards":[{"icon":"图标名","title":"卡片标题","text":"2-3 句充实说明(别只写一句,避免卡片空旷)","tag":"短标签"}]}  // 卡片 3 或 6 个
- {"type":"cols","title":"页标题","left":{"title":"左栏标题","items":["要点","要点","要点","要点"]},"right":{"title":"右栏标题","items":["要点","要点","要点","要点"]}}  // 每栏 4-5 条
- {"type":"stats","title":"页标题","stats":[{"n":"$1T","l":"说明"},{"n":"40%%","l":"说明","pct":40}],"note":"可选脚注"}  // 给 pct(0-100)的会画成环形图,适合"百分比/完成度"
- {"type":"bars","title":"页标题","bars":[{"label":"维度","value":85},{"label":"维度","value":60}],"note":"可选"}  // 横向条形图,value 0-100
- {"type":"steps","title":"页标题","steps":[{"title":"步骤名","text":"1-2 句说明"}]}  // 3-4 步
- {"type":"quote","quote":"一句金句/用户证言","by":"出处"}
- {"type":"bullets","title":"页标题","bullets":["要点","要点","要点","要点"]}
- {"type":"closing","kicker":"谢谢观看","title":"收尾金句","subtitle":"一句话"}%s

icon 名只能从这里选:%s

要求:首页 cover、末页 closing;中间穿插 section 分隔、hero、cards、stats/bars(尽量用图表)、cols、quote;内容充实(卡片每张 2-3 句、列表每栏 4-5 条),让每页视觉饱满。只返回 JSON。`,
		slides, language, styleLine, imgSection, icons)
}

func deckSpecUserPrompt(title, brief string) string {
	var b strings.Builder
	if strings.TrimSpace(title) != "" {
		b.WriteString("封面主标题:" + title + "\n\n")
	}
	b.WriteString("主题与要点素材:\n\n")
	b.WriteString(brief)
	return b.String()
}

// ---- browser / io helpers ----

func findChromium() (string, error) {
	for _, env := range []string{"YUNQUE_CHROME", "CHROME_PATH", "EDGE_PATH"} {
		if p := os.Getenv(env); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	var cands []string
	switch runtime.GOOS {
	case "windows":
		pf := os.Getenv("ProgramFiles")
		pfx := os.Getenv("ProgramFiles(x86)")
		cands = []string{
			filepath.Join(pf, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(pfx, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(pfx, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(pf, `Microsoft\Edge\Application\msedge.exe`),
		}
	case "darwin":
		cands = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default:
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"} {
			if p, err := exec.LookPath(name); err == nil {
				return p, nil
			}
		}
	}
	for _, c := range cands {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}
	return "", fmt.Errorf("no Chrome/Edge/Chromium found for headless rendering; set YUNQUE_CHROME to the browser path")
}

func fileURL(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	abs = filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	return "file://" + abs
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "\n")
	return strings.TrimSpace(parts[len(parts)-1])
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		var x int
		fmt.Sscanf(n, "%d", &x)
		return x
	}
	return 0
}

// deckBaseCSS — structure + components; theme via :root vars (set per style).
// NOTE: kept free of fmt verbs (it contains many literal '%').
const deckBaseCSS = `@page{size:1280px 720px;margin:0}
*{margin:0;padding:0;box-sizing:border-box}
html,body{font-family:"Segoe UI","Microsoft YaHei","PingFang SC",system-ui,sans-serif;-webkit-font-smoothing:antialiased}
.slide{position:relative;width:1280px;height:720px;overflow:hidden;background:var(--bg);color:var(--ink);display:flex;flex-direction:column;padding:50px 76px 104px;page-break-after:always}
.slide:last-child{page-break-after:auto}
.deco,.cdeco{position:absolute;inset:0;width:100%;height:100%;z-index:0;pointer-events:none}
.slide>*:not(.deco):not(.cdeco){position:relative;z-index:1}
.kicker{display:inline-flex;align-items:center;gap:9px;font-size:14px;font-weight:700;letter-spacing:.16em;color:var(--a1);text-transform:uppercase}
.kicker::before{content:"";width:26px;height:3px;border-radius:3px;background:linear-gradient(90deg,var(--a1),var(--a2))}
.head{display:flex;flex-direction:column;gap:6px;margin-bottom:18px}
.head h2{font-size:38px;font-weight:800;letter-spacing:-.5px;line-height:1.1}
.head .subt{font-size:17px;color:var(--muted);max-width:900px}
.body{flex:1;display:flex;flex-direction:column;justify-content:center;min-height:0}
.lead{font-size:19px;color:var(--muted);line-height:1.6;max-width:940px}
.pagefoot{position:absolute;left:76px;right:76px;bottom:26px;display:flex;justify-content:space-between;align-items:center;font-size:12.5px;color:var(--muted);opacity:.8;border-top:1px solid var(--bd);padding-top:13px}
.pagefoot .brand{display:flex;align-items:center;gap:8px;font-weight:600}
.dot{width:9px;height:9px;border-radius:50%;background:linear-gradient(135deg,var(--a1),var(--a2))}
/* cover */
.cover{justify-content:center;background:var(--cover);color:#fff}
.cwrap{display:flex;flex-direction:column;align-items:flex-start;max-width:920px}
.cover .kicker{color:#fff;opacity:.85}
.cover h1{font-size:62px;font-weight:850;letter-spacing:-1.2px;line-height:1.08;margin:20px 0 16px}
.cover .sub{font-size:21px;color:rgba(255,255,255,.78);line-height:1.5;max-width:780px}
.cover .rule{width:90px;height:5px;border-radius:5px;background:linear-gradient(90deg,var(--a1),var(--a2),var(--a3))}
.cover .meta{margin-top:40px;display:flex;gap:13px;flex-wrap:wrap}
.pill{padding:9px 17px;border:1px solid rgba(255,255,255,.24);border-radius:999px;font-size:14px;color:rgba(255,255,255,.92);background:rgba(255,255,255,.06)}
.closing h1{font-size:64px}
/* section divider */
.divider{background:var(--cover);color:#fff;justify-content:center}
.dwrap{display:flex;flex-direction:column}
.divider .num{font-size:150px;font-weight:850;line-height:.9;color:var(--a1)}
.divider h2{font-size:48px;font-weight:800;margin-top:4px}
.divider p{color:rgba(255,255,255,.7);font-size:19px;margin-top:14px;max-width:720px;line-height:1.5}
/* cards */
.grid{display:grid;gap:20px;width:100%}
.g3{grid-template-columns:repeat(3,1fr)}
.g2{grid-template-columns:repeat(2,1fr)}
.card{background:var(--soft);border:1px solid var(--bd);border-radius:var(--rad);padding:20px 22px;position:relative;overflow:hidden;display:flex;flex-direction:column}
.card::before{content:"";position:absolute;top:0;left:0;right:0;height:4px;background:linear-gradient(90deg,var(--a1),var(--a2))}
.card .ic{width:48px;height:48px;border-radius:13px;display:flex;align-items:center;justify-content:center;background:linear-gradient(135deg,color-mix(in srgb,var(--a1) 16%,transparent),color-mix(in srgb,var(--a2) 16%,transparent));margin-bottom:13px}
.card .ic svg{width:26px;height:26px}
.card h3{font-size:20px;font-weight:750;margin-bottom:7px}
.card p{font-size:14.5px;color:var(--muted);line-height:1.5;flex:1}
.card .tag{align-self:flex-start;margin-top:12px;font-size:12.5px;font-weight:650;color:var(--a1);background:color-mix(in srgb,var(--a1) 12%,transparent);padding:4px 11px;border-radius:999px}
/* stats + ring */
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:24px;align-items:start}
.stat .n{font-size:54px;font-weight:850;letter-spacing:-1px;line-height:1;color:var(--a1)}
.stat .l{font-size:15px;color:var(--muted);margin-top:10px;line-height:1.45}
.stat.ring{display:flex;flex-direction:column;align-items:center;text-align:center}
.ring-svg{width:140px;height:140px}
.ring-t{font-size:30px;font-weight:800;fill:var(--ink)}
.stat.ring .l{margin-top:6px}
/* bars */
.barwrap{display:flex;flex-direction:column;gap:22px;width:100%}
.bar{display:grid;grid-template-columns:200px 1fr 56px;align-items:center;gap:18px}
.bar .bl{font-size:17px;font-weight:600}
.bar .bt{height:16px;border-radius:999px;background:var(--bd);overflow:hidden}
.bar .bf{height:100%;border-radius:999px;background:linear-gradient(90deg,var(--a1),var(--a2))}
.bar .bv{font-size:20px;font-weight:800;color:var(--a1);text-align:right}
/* cols */
.cols{display:grid;grid-template-columns:1fr 1fr;gap:24px;width:100%}
.col{border-radius:var(--rad);padding:28px;border:1px solid var(--bd);background:var(--soft)}
.col h3{font-size:22px;font-weight:800;margin-bottom:18px;display:flex;align-items:center;gap:10px}
.col h3::before{content:"";width:8px;height:24px;border-radius:4px;background:linear-gradient(180deg,var(--a1),var(--a2))}
.col ul{list-style:none;display:flex;flex-direction:column;gap:14px}
.col li{font-size:16px;color:var(--ink);opacity:.92;line-height:1.5;padding-left:24px;position:relative}
.col li::before{content:"";position:absolute;left:0;top:8px;width:9px;height:9px;border-radius:50%;background:linear-gradient(135deg,var(--a1),var(--a2))}
/* steps */
.steps{display:flex;flex-direction:column;gap:16px;width:100%}
.step{display:flex;gap:20px;align-items:flex-start;background:var(--soft);border:1px solid var(--bd);border-radius:var(--rad);padding:22px 26px}
.step .si{flex:none;width:44px;height:44px;border-radius:13px;color:#fff;font-weight:800;font-size:20px;display:flex;align-items:center;justify-content:center;background:linear-gradient(135deg,var(--a1),var(--a2))}
.step h3{font-size:19px;font-weight:750}
.step p{font-size:15px;color:var(--muted);margin-top:4px;line-height:1.55}
/* bullets */
.blist{list-style:none;display:flex;flex-direction:column;gap:18px;width:100%}
.blist li{font-size:23px;color:var(--ink);line-height:1.45;padding-left:34px;position:relative;font-weight:550}
.blist li::before{content:"";position:absolute;left:0;top:12px;width:13px;height:13px;border-radius:50%;background:linear-gradient(135deg,var(--a1),var(--a2))}
/* hero */
.hero{justify-content:center}
.hwrap{max-width:1000px}
.hero .big{font-size:60px;font-weight:850;letter-spacing:-1px;line-height:1.12;margin:18px 0 18px;color:var(--ink)}
/* quote */
.quote{justify-content:center}
.qwrap{max-width:980px}
.qmark{font-size:120px;font-weight:850;line-height:.6;color:var(--a1);opacity:.35}
.quote blockquote{font-size:40px;font-weight:750;line-height:1.3;letter-spacing:-.5px;margin-top:6px}
.quote .by{margin-top:24px;font-size:18px;color:var(--muted);font-weight:600}
.note{font-size:14px;color:var(--muted);margin-top:18px;opacity:.85}
/* images: cover bg */
.cover.hasbg{background:#0a0c14}
.coverbg{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;z-index:0}
.cshade{position:absolute;inset:0;z-index:0;background:linear-gradient(90deg,rgba(8,10,18,.92) 0%,rgba(8,10,18,.72) 45%,rgba(8,10,18,.34) 100%)}
/* imagesplit */
.split{padding:0}
.splitwrap{flex:1;display:grid;grid-template-columns:1fr 1fr;width:100%}
.sptext{padding:64px 64px;display:flex;flex-direction:column;justify-content:center}
.sptext .head{margin-bottom:20px}
.splitimg{position:relative;overflow:hidden}
.splitimg img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}
.splitimg.ph{display:flex;align-items:center;justify-content:center;background:linear-gradient(135deg,color-mix(in srgb,var(--a1) 22%,var(--bg)),color-mix(in srgb,var(--a2) 18%,var(--bg)));color:var(--muted);font-size:15px;letter-spacing:.1em}
.blist.sm li{font-size:18px;font-weight:500;gap:0;margin:0;padding-left:30px}
.blist.sm li::before{top:9px;width:11px;height:11px}
.split .pagefoot{left:64px}
/* imagefull */
.imagefull{padding:0;justify-content:flex-end}
.ffimg{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;z-index:0}
.ffshade{position:absolute;inset:0;z-index:0;background:linear-gradient(180deg,rgba(8,10,18,.10) 40%,rgba(8,10,18,.86) 100%)}
.capbar{position:relative;z-index:1;padding:54px 76px;color:#fff}
.capbar h2{font-size:40px;font-weight:800;letter-spacing:-.5px}
.capbar .cap{font-size:18px;color:rgba(255,255,255,.82);margin-top:8px;max-width:900px}
/* gallery */
.gallery{display:grid;gap:18px;width:100%}
.gcell{position:relative;height:340px;border-radius:var(--rad);overflow:hidden;border:1px solid var(--bd)}
.gallery.g2 .gcell{height:380px}
.gcell img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}`
