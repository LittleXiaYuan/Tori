package channel

import (
	"testing"
)

func TestRichMessage_TextOnly(t *testing.T) {
	rm := NewRichMessage().AddText("hello").AddText("world")
	if rm.ToPlainText() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", rm.ToPlainText())
	}
	if rm.TextContent() != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got '%s'", rm.TextContent())
	}
}

func TestRichMessage_Mixed(t *testing.T) {
	rm := NewRichMessage().
		AddText("看这张图").
		AddImage("https://example.com/pic.jpg", "风景").
		Add(NewAt("u123", "张三")).
		Add(NewButton("确认", "ok", "primary"))

	plain := rm.ToPlainText()
	if plain != "看这张图 [图片: 风景] @张三 [确认]" {
		t.Errorf("unexpected plain text: '%s'", plain)
	}

	if !rm.HasType(ComponentImage) {
		t.Error("should have image")
	}
	if rm.HasType(ComponentVideo) {
		t.Error("should not have video")
	}

	img := rm.GetFirst(ComponentImage)
	if img == nil {
		t.Fatal("image not found")
	}
	if img.(*ImageComponent).URL != "https://example.com/pic.jpg" {
		t.Error("wrong image URL")
	}
}

func TestRichMessage_JSON_Roundtrip(t *testing.T) {
	rm := NewRichMessage().
		AddText("hello").
		AddImage("https://example.com/img.png", "test").
		Add(NewAudio("https://example.com/audio.ogg", 15)).
		Add(NewFile("https://example.com/doc.pdf", "report.pdf")).
		Add(NewAt("user1", "Alice")).
		Add(NewAtAll()).
		Add(NewReply("msg123")).
		Add(NewButton("Go", "go", "primary")).
		Add(NewLink("Click", "https://example.com")).
		Add(NewEmoji("smile123", "smile")).
		Add(func() Component { s := NewSticker("11537", "52002734"); s.Platform = "line"; return s }())

	jsonStr := rm.ToJSON()
	parsed, err := ParseRichMessage(jsonStr)
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed.Components) != len(rm.Components) {
		t.Errorf("expected %d components, got %d", len(rm.Components), len(parsed.Components))
	}

	// Verify types match
	for i, c := range rm.Components {
		if parsed.Components[i].Type() != c.Type() {
			t.Errorf("component %d: expected %s, got %s", i, c.Type(), parsed.Components[i].Type())
		}
	}
}

func TestComponentTypes(t *testing.T) {
	tests := []struct {
		component Component
		typ       ComponentType
		plain     string
	}{
		{NewText("hi"), ComponentText, "hi"},
		{NewImageFromURL("u", "pic"), ComponentImage, "[图片: pic]"},
		{NewImageFromBase64("data", ""), ComponentImage, "[图片]"},
		{NewAudio("u", 10), ComponentAudio, "[语音 10s]"},
		{NewVideo("u", 30), ComponentVideo, "[视频 30s]"},
		{NewFile("u", "test.pdf"), ComponentFile, "[文件: test.pdf]"},
		{NewAt("uid", "name"), ComponentAt, "@name"},
		{NewAtAll(), ComponentAtAll, "@全体成员"},
		{NewReply("mid"), ComponentReply, ""},
		{NewButton("OK", "ok", "default"), ComponentButton, "[OK]"},
		{NewLinkButton("Go", "https://example.com"), ComponentButton, "[Go]"},
		{NewLink("Title", "https://url"), ComponentLink, "[Title](https://url)"},
		{NewEmoji("id", "smile"), ComponentEmoji, "[smile]"},
		{NewSticker("11537", "52002734"), ComponentSticker, "[贴图: packageId=11537, stickerId=52002734]"},
		{NewWechatEmoji("abc123"), ComponentWechatEmoji, "[微信表情]"},
		{NewFace(14, "微笑"), ComponentFace, "[微笑]"},
		{NewFace(0, ""), ComponentFace, "[表情:0]"},
	}

	for _, tt := range tests {
		if tt.component.Type() != tt.typ {
			t.Errorf("wrong type for %s", tt.typ)
		}
		if tt.component.ToPlainText() != tt.plain {
			t.Errorf("wrong plain text for %s: got '%s', want '%s'",
				tt.typ, tt.component.ToPlainText(), tt.plain)
		}
	}
}

func TestImageComponent_DecodeBase64(t *testing.T) {
	// Valid base64 data
	img := NewImageFromBase64("aGVsbG8=", "")
	data, err := img.DecodeBase64()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(data))
	}

	// No data
	img2 := NewImageFromURL("https://example.com", "")
	_, err = img2.DecodeBase64()
	if err == nil {
		t.Error("expected error for no base64 data")
	}
}

func TestFileComponent_Extension(t *testing.T) {
	f := NewFile("https://example.com/report.pdf", "report.pdf")
	if f.Extension() != ".pdf" {
		t.Errorf("expected '.pdf', got '%s'", f.Extension())
	}
}

func TestStickerComponent(t *testing.T) {
	s := NewSticker("11537", "52002734")
	s.Platform = "line"

	if s.Type() != ComponentSticker {
		t.Errorf("expected sticker type, got %s", s.Type())
	}

	expectedURL := "https://stickershop.line-scdn.net/stickershop/v1/sticker/52002734/iPhone/sticker.png"
	if s.StickerURL() != expectedURL {
		t.Errorf("StickerURL() = %q, want %q", s.StickerURL(), expectedURL)
	}

	// Custom URL takes precedence
	s.URL = "https://custom.cdn/sticker.png"
	if s.StickerURL() != "https://custom.cdn/sticker.png" {
		t.Errorf("custom URL should take precedence")
	}

	// Unknown platform returns empty
	s2 := NewSticker("pkg", "stk")
	if s2.StickerURL() != "" {
		t.Errorf("unknown platform should return empty URL")
	}

	// JSON roundtrip
	jsonMap := s.ToJSON()
	if jsonMap["package_id"] != "11537" || jsonMap["sticker_id"] != "52002734" {
		t.Errorf("JSON fields incorrect: %v", jsonMap)
	}
	if jsonMap["platform"] != "line" {
		t.Errorf("platform should be 'line' in JSON")
	}
}

func TestWechatEmojiComponent(t *testing.T) {
	we := NewWechatEmoji("abc123def456")
	we.MD5Len = 12
	we.CDNURL = "https://cdn.example.com/emoji.gif"

	if we.Type() != ComponentWechatEmoji {
		t.Errorf("expected wechat_emoji type, got %s", we.Type())
	}
	if we.ToPlainText() != "[微信表情]" {
		t.Errorf("plain text = %q, want [微信表情]", we.ToPlainText())
	}

	jsonMap := we.ToJSON()
	if jsonMap["md5"] != "abc123def456" {
		t.Errorf("md5 incorrect: %v", jsonMap)
	}
	if jsonMap["md5_len"] != 12 {
		t.Errorf("md5_len incorrect: %v", jsonMap)
	}
	if jsonMap["cdnurl"] != "https://cdn.example.com/emoji.gif" {
		t.Errorf("cdnurl incorrect: %v", jsonMap)
	}

	// JSON roundtrip
	rm := NewRichMessage().Add(we)
	jsonStr := rm.ToJSON()
	parsed, err := ParseRichMessage(jsonStr)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Components) != 1 || parsed.Components[0].Type() != ComponentWechatEmoji {
		t.Error("roundtrip failed for wechat_emoji")
	}
}

func TestFaceComponent(t *testing.T) {
	f := NewFace(14, "微笑")
	if f.Type() != ComponentFace {
		t.Errorf("expected face type, got %s", f.Type())
	}
	if f.ToPlainText() != "[微笑]" {
		t.Errorf("plain text = %q, want [微笑]", f.ToPlainText())
	}

	// Without name
	f2 := NewFace(0, "")
	if f2.ToPlainText() != "[表情:0]" {
		t.Errorf("plain text = %q, want [表情:0]", f2.ToPlainText())
	}

	// JSON roundtrip
	rm := NewRichMessage().Add(f)
	jsonStr := rm.ToJSON()
	parsed, err := ParseRichMessage(jsonStr)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Components) != 1 || parsed.Components[0].Type() != ComponentFace {
		t.Error("roundtrip failed for face")
	}
}

func TestMessageWithRich(t *testing.T) {
	msg := Message{
		ChannelType: "feishu",
		Content:     "hello",
		Rich:        NewRichMessage().AddText("hello").AddImage("https://example.com/img.png", "test"),
	}
	if msg.Rich == nil {
		t.Fatal("rich message should not be nil")
	}
	if len(msg.Rich.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(msg.Rich.Components))
	}
}
