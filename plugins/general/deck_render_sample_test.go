package general

import (
	"os"
	"path/filepath"
	"testing"
)

// deckTestDir is the scratch output directory for the (guarded) local deck
// render tests. Override with YQ_DECK_OUT; defaults to a temp dir so the tests
// carry no machine-specific paths.
func deckTestDir() string {
	if d := os.Getenv("YQ_DECK_OUT"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "yqdeck")
}

// TestRenderDeckSample writes one HTML per style covering every slide type, for
// local visual iteration. Skipped unless YQ_DECK_SAMPLE=1 (so it never runs in CI).
//   $env:YQ_DECK_SAMPLE="1"; go test ./plugins/general/ -run TestRenderDeckSample
func TestRenderDeckSample(t *testing.T) {
	if os.Getenv("YQ_DECK_SAMPLE") == "" {
		t.Skip("set YQ_DECK_SAMPLE=1 to emit sample decks")
	}
	spec := &deckSpec{
		Title:    "云雀 Yunque · AI 陪伴助手",
		Subtitle: "不只是 AI,是懂你的伙伴 —— 记得你、会规划、会做事。",
		Footer:   "云雀 Yunque",
		Slides: []deckSlide{
			{Type: "cover", Kicker: "YUNQUE INSIGHT · 产品简报", Title: "云雀 Yunque",
				Subtitle: "一个真正记得你的 AI 陪伴体:语义记忆 + 认知内核 + 原生创作。",
				Badges:   []string{"长期记忆", "认知引擎", "隐私安全", "2026"}},
			{Type: "section", Num: "01", Title: "核心能力", Desc: "五大特性,重新定义 AI 陪伴。"},
			{Type: "cards", Title: "记得你 · 会思考 · 会做事", Subtitle: "从工具升级为懂你的执行体",
				Cards: []deckCard{
					{Icon: "memory", Title: "长期记忆", Text: "跨会话记住你的偏好、背景与习惯;换个说法也能精准想起,不必重复交代。", Tag: "语义召回"},
					{Icon: "brain", Title: "认知内核 Cogni", Text: "会规划、会反思、会主动学习;按场景切换性情与能力,像一个真正成长的伙伴。", Tag: "Cogni"},
					{Icon: "shield", Title: "隐私安全", Text: "多租户严格隔离,你的记忆只属于你;本地优先,数据可控可追溯。", Tag: "多租户"},
				}},
			{Type: "stats", Title: "可度量的体验", Stats: []deckStat{
				{N: "20/20", L: "换说法召回命中(真机评测)"},
				{N: "记得你", L: "语义记忆已真机验证", Pct: 100},
				{N: "↓71%", L: "上下文 token 治理", Pct: 71},
				{N: "0", L: "用户额外依赖(无需 Docker)"},
			}},
			{Type: "bars", Title: "与传统助手的差距", Bars: []deckBar{
				{Label: "换说法召回", Value: 95}, {Label: "原生创作质量", Value: 88},
				{Label: "隐私隔离", Value: 92}, {Label: "用户安装成本(越低越好)", Value: 12},
			}, Note: "示意值,用于趋势对比。"},
			{Type: "cards", Title: "六大核心能力", Subtitle: "来自云雀的全面智能,不止于聊天",
				Cards: []deckCard{
					{Icon: "memory", Title: "长期语义记忆", Text: "跨会话记住偏好、背景与关键信息;即便换个问法也能精准召回,真机评测 20/20。", Tag: "独家"},
					{Icon: "brain", Title: "认知内核 Cogni", Text: "具备规划、反思与主动学习能力,能按场景切换性情与能力,像人一样灵活进化。", Tag: "AI大脑"},
					{Icon: "rocket", Title: "原生创作", Text: "一句话生成 PPT、Word、表格,设计对标 Gamma 与 Kimi;无需模板、零依赖。", Tag: "高效"},
					{Icon: "shield", Title: "隐私安全", Text: "多租户严格隔离、本地优先存储;数据加密传输,只有你能访问自己的记忆。", Tag: "安全"},
					{Icon: "route", Title: "多模型智能路由", Text: "动态选择最优 AI 模型,兼顾性能与成本,让每一次响应又快又省。", Tag: "智能调度"},
					{Icon: "users", Title: "情感化陪伴", Text: "感知情绪、调整语调与风格,不只是工具,更能共情与陪伴,让交互充满温度。", Tag: "温暖"},
				}},
			{Type: "section", Num: "02", Title: "原生创作", Desc: "文档、演示、表格,一句话生成。"},
			{Type: "cols", Title: "海量能力,一个内核", Left: &deckCol{Title: "懂你", Items: []string{"长期语义记忆", "情绪与意图感知", "主动关心与提醒", "跨设备一致人格"}},
				Right: &deckCol{Title: "替你做", Items: []string{"原创 PPT / Word / 表格", "联网研究与提炼", "多模型智能路由", "工具编排与执行"}}},
			{Type: "steps", Title: "它如何为你工作", Steps: []deckStep{
				{Title: "听懂", Text: "理解你的目标与上下文,必要时主动追问。"},
				{Title: "规划", Text: "Cogni 拆解任务、选模型、定策略。"},
				{Title: "执行", Text: "调用记忆与工具,产出可用成果。"},
				{Title: "记住", Text: "把这次的偏好沉淀进长期记忆。"},
			}},
			{Type: "quote", Quote: "好的陪伴,先证明它真的记得你。", By: "云雀产品理念"},
			{Type: "closing", Kicker: "谢谢观看", Title: "把握转折,稳健落地", Subtitle: "云雀 Yunque · 原生生成 · 用户零依赖"},
		},
	}
	out := deckTestDir()
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for _, st := range []string{"aurora", "clean", "sunset", "mono"} {
		spec.Style = st
		htmlDoc := renderDeckHTML(spec, nil)
		if err := os.WriteFile(filepath.Join(out, st+".html"), []byte(htmlDoc), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s.html (%d bytes)", st, len(htmlDoc))
	}
}

// TestRenderDeckImages verifies the image layouts (cover bg / imagesplit /
// imagefull / gallery) render with real embedded images. Skipped unless
// YQ_DECK_SAMPLE=1 and C:\Temp\yqdeck3\uploads holds images.
func TestRenderDeckImages(t *testing.T) {
	if os.Getenv("YQ_DECK_SAMPLE") == "" {
		t.Skip("set YQ_DECK_SAMPLE=1")
	}
	uploads := filepath.Join(deckTestDir(), "uploads")
	skill := NewDeckCreateSkill([]string{uploads}, []string{uploads})
	imgs := skill.collectImages(uploads)
	if len(imgs) == 0 {
		t.Skipf("no images found in %s", uploads)
	}
	t.Logf("collected %d images: %s", len(imgs), imageManifest(imgs))

	spec := &deckSpec{
		Title: "云雀 Yunque · 配图版式", Subtitle: "图片上传/解压后自动嵌入", Footer: "云雀 Yunque", Style: "aurora",
		Slides: []deckSlide{
			{Type: "cover", Kicker: "图片能力 · DEMO", Title: "云雀 Yunque", Subtitle: "上传图片或压缩包,自动嵌入精美演示。", Image: 1,
				Badges: []string{"封面背景图", "图文分栏", "全幅大图", "画廊"}},
			{Type: "imagesplit", Kicker: "图文分栏", Title: "懂你的陪伴", Image: 2, ImgRight: true,
				Bullets: []string{"长期语义记忆,跨会话记住你", "情绪与意图感知", "主动关心与提醒", "跨设备一致人格"}},
			{Type: "imagefull", Image: 3, Title: "原生创作引擎", Caption: "一句话生成精美 PPT/Word/表格,设计对标 Gamma 与 Kimi。"},
			{Type: "gallery", Title: "成果一览", Images: []int{1, 2, 3}},
			{Type: "closing", Kicker: "谢谢观看", Title: "把握转折,稳健落地", Subtitle: "云雀 Yunque · 原生生成 · 用户零依赖"},
		},
	}
	htmlDoc := renderDeckHTML(spec, imgs)
	out := filepath.Join(deckTestDir(), "images.html")
	if err := os.WriteFile(out, []byte(htmlDoc), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote images.html (%d bytes)", len(htmlDoc))
}
