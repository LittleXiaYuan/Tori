package channel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// ComponentType 消息组件类型
type ComponentType string

const (
	ComponentText        ComponentType = "text"
	ComponentImage       ComponentType = "image"
	ComponentAudio       ComponentType = "audio"
	ComponentVideo       ComponentType = "video"
	ComponentFile        ComponentType = "file"
	ComponentAt          ComponentType = "at"
	ComponentAtAll       ComponentType = "at_all"
	ComponentReply       ComponentType = "reply"
	ComponentCard        ComponentType = "card"
	ComponentButton      ComponentType = "button"
	ComponentLink        ComponentType = "link"
	ComponentEmoji       ComponentType = "emoji"
	ComponentSticker     ComponentType = "sticker"
	ComponentWechatEmoji ComponentType = "wechat_emoji"
	ComponentFace        ComponentType = "face"
)

// Component 富消息组件接口
type Component interface {
	Type() ComponentType
	// ToPlainText 降级为纯文本 (当平台不支持该组件时)
	ToPlainText() string
	// ToJSON returns JSON-serializable representation
	ToJSON() map[string]any
}

// ────────────────────────────────────────
// Text 文本组件
// ────────────────────────────────────────

type TextComponent struct {
	Content string `json:"content"`
}

func NewText(content string) *TextComponent  { return &TextComponent{Content: content} }
func (c *TextComponent) Type() ComponentType { return ComponentText }
func (c *TextComponent) ToPlainText() string { return c.Content }
func (c *TextComponent) ToJSON() map[string]any {
	return map[string]any{"type": string(c.Type()), "content": c.Content}
}

// ────────────────────────────────────────
// Image 图片组件
// ────────────────────────────────────────

type ImageComponent struct {
	URL    string `json:"url,omitempty"`     // HTTP URL or file:// path
	Base64 string `json:"base64,omitempty"`  // base64 encoded data
	Alt    string `json:"alt,omitempty"`     // alt text
	FileID string `json:"file_id,omitempty"` // platform media ID (after upload)
}

func NewImageFromURL(url, alt string) *ImageComponent {
	return &ImageComponent{URL: url, Alt: alt}
}

func NewImageFromBase64(data, alt string) *ImageComponent {
	return &ImageComponent{Base64: data, Alt: alt}
}

func (c *ImageComponent) Type() ComponentType { return ComponentImage }
func (c *ImageComponent) ToPlainText() string {
	if c.Alt != "" {
		return fmt.Sprintf("[图片: %s]", c.Alt)
	}
	return "[图片]"
}
func (c *ImageComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type())}
	if c.URL != "" {
		m["url"] = c.URL
	}
	if c.Base64 != "" {
		m["base64"] = c.Base64
	}
	if c.Alt != "" {
		m["alt"] = c.Alt
	}
	if c.FileID != "" {
		m["file_id"] = c.FileID
	}
	return m
}

// HasData checks if the image has content (local or remote).
func (c *ImageComponent) HasData() bool {
	return c.URL != "" || c.Base64 != ""
}

// DecodeBase64 returns raw bytes from base64 data.
func (c *ImageComponent) DecodeBase64() ([]byte, error) {
	if c.Base64 == "" {
		return nil, fmt.Errorf("no base64 data")
	}
	return base64.StdEncoding.DecodeString(c.Base64)
}

// ────────────────────────────────────────
// Audio 音频组件
// ────────────────────────────────────────

type AudioComponent struct {
	URL      string `json:"url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Duration int    `json:"duration,omitempty"` // seconds
	Format   string `json:"format,omitempty"`   // "amr", "ogg", "wav"
}

func NewAudio(url string, duration int) *AudioComponent {
	return &AudioComponent{URL: url, Duration: duration}
}

func (c *AudioComponent) Type() ComponentType { return ComponentAudio }
func (c *AudioComponent) ToPlainText() string { return fmt.Sprintf("[语音 %ds]", c.Duration) }
func (c *AudioComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type())}
	if c.URL != "" {
		m["url"] = c.URL
	}
	if c.FileID != "" {
		m["file_id"] = c.FileID
	}
	if c.Duration > 0 {
		m["duration"] = c.Duration
	}
	if c.Format != "" {
		m["format"] = c.Format
	}
	return m
}

// ────────────────────────────────────────
// Video 视频组件
// ────────────────────────────────────────

type VideoComponent struct {
	URL      string `json:"url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	CoverURL string `json:"cover_url,omitempty"` // thumbnail
	Duration int    `json:"duration,omitempty"`  // seconds
}

func NewVideo(url string, duration int) *VideoComponent {
	return &VideoComponent{URL: url, Duration: duration}
}

func (c *VideoComponent) Type() ComponentType { return ComponentVideo }
func (c *VideoComponent) ToPlainText() string { return fmt.Sprintf("[视频 %ds]", c.Duration) }
func (c *VideoComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type())}
	if c.URL != "" {
		m["url"] = c.URL
	}
	if c.FileID != "" {
		m["file_id"] = c.FileID
	}
	if c.CoverURL != "" {
		m["cover_url"] = c.CoverURL
	}
	if c.Duration > 0 {
		m["duration"] = c.Duration
	}
	return m
}

// ────────────────────────────────────────
// File 文件组件
// ────────────────────────────────────────

type FileComponent struct {
	URL      string `json:"url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"` // bytes
	MimeType string `json:"mime_type,omitempty"`
}

func NewFile(url, fileName string) *FileComponent {
	return &FileComponent{URL: url, FileName: fileName}
}

func (c *FileComponent) Type() ComponentType { return ComponentFile }
func (c *FileComponent) ToPlainText() string {
	name := c.FileName
	if name == "" {
		name = "file"
	}
	return fmt.Sprintf("[文件: %s]", name)
}
func (c *FileComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type())}
	if c.URL != "" {
		m["url"] = c.URL
	}
	if c.FileID != "" {
		m["file_id"] = c.FileID
	}
	if c.FileName != "" {
		m["file_name"] = c.FileName
	}
	if c.FileSize > 0 {
		m["file_size"] = c.FileSize
	}
	if c.MimeType != "" {
		m["mime_type"] = c.MimeType
	}
	return m
}

// Extension returns the file extension (e.g., ".pdf").
func (c *FileComponent) Extension() string {
	return filepath.Ext(c.FileName)
}

// ────────────────────────────────────────
// At (@提及) 组件
// ────────────────────────────────────────

type AtComponent struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name,omitempty"`
}

func NewAt(userID, userName string) *AtComponent {
	return &AtComponent{UserID: userID, UserName: userName}
}

func (c *AtComponent) Type() ComponentType { return ComponentAt }
func (c *AtComponent) ToPlainText() string {
	if c.UserName != "" {
		return "@" + c.UserName
	}
	return "@" + c.UserID
}
func (c *AtComponent) ToJSON() map[string]any {
	return map[string]any{"type": string(c.Type()), "user_id": c.UserID, "user_name": c.UserName}
}

// ────────────────────────────────────────
// AtAll (@全体) 组件
// ────────────────────────────────────────

type AtAllComponent struct{}

func NewAtAll() *AtAllComponent               { return &AtAllComponent{} }
func (c *AtAllComponent) Type() ComponentType { return ComponentAtAll }
func (c *AtAllComponent) ToPlainText() string { return "@全体成员" }
func (c *AtAllComponent) ToJSON() map[string]any {
	return map[string]any{"type": string(c.Type())}
}

// ────────────────────────────────────────
// Reply (引用回复) 组件
// ────────────────────────────────────────

type ReplyComponent struct {
	MessageID string `json:"message_id"`
}

func NewReply(messageID string) *ReplyComponent { return &ReplyComponent{MessageID: messageID} }
func (c *ReplyComponent) Type() ComponentType   { return ComponentReply }
func (c *ReplyComponent) ToPlainText() string   { return "" } // invisible in plain text
func (c *ReplyComponent) ToJSON() map[string]any {
	return map[string]any{"type": string(c.Type()), "message_id": c.MessageID}
}

// ────────────────────────────────────────
// Button 按钮组件
// ────────────────────────────────────────

type ButtonComponent struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Style string `json:"style,omitempty"` // "primary", "danger", "default"
	URL   string `json:"url,omitempty"`   // link button
}

func NewButton(label, value, style string) *ButtonComponent {
	return &ButtonComponent{Label: label, Value: value, Style: style}
}

func NewLinkButton(label, url string) *ButtonComponent {
	return &ButtonComponent{Label: label, URL: url, Style: "default"}
}

func (c *ButtonComponent) Type() ComponentType { return ComponentButton }
func (c *ButtonComponent) ToPlainText() string { return fmt.Sprintf("[%s]", c.Label) }
func (c *ButtonComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type()), "label": c.Label}
	if c.Value != "" {
		m["value"] = c.Value
	}
	if c.Style != "" {
		m["style"] = c.Style
	}
	if c.URL != "" {
		m["url"] = c.URL
	}
	return m
}

// ────────────────────────────────────────
// Link 链接组件
// ────────────────────────────────────────

type LinkComponent struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Desc  string `json:"desc,omitempty"`
}

func NewLink(title, url string) *LinkComponent { return &LinkComponent{Title: title, URL: url} }
func (c *LinkComponent) Type() ComponentType   { return ComponentLink }
func (c *LinkComponent) ToPlainText() string   { return fmt.Sprintf("[%s](%s)", c.Title, c.URL) }
func (c *LinkComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type()), "title": c.Title, "url": c.URL}
	if c.Desc != "" {
		m["desc"] = c.Desc
	}
	return m
}

// ────────────────────────────────────────
// Emoji 表情组件
// ────────────────────────────────────────

type EmojiComponent struct {
	ID   string `json:"id"`   // platform emoji ID
	Name string `json:"name"` // display text (e.g., "smile")
}

func NewEmoji(id, name string) *EmojiComponent { return &EmojiComponent{ID: id, Name: name} }
func (c *EmojiComponent) Type() ComponentType  { return ComponentEmoji }
func (c *EmojiComponent) ToPlainText() string  { return "[" + c.Name + "]" }
func (c *EmojiComponent) ToJSON() map[string]any {
	return map[string]any{"type": string(c.Type()), "id": c.ID, "name": c.Name}
}

// ────────────────────────────────────────
// Sticker 贴图/表情包组件
// ────────────────────────────────────────

type StickerComponent struct {
	PackageID  string `json:"package_id"`            // sticker package/set ID (LINE)
	StickerID  string `json:"sticker_id"`            // individual sticker ID
	URL        string `json:"url,omitempty"`         // direct image URL
	Platform   string `json:"platform,omitempty"`    // source platform (e.g., "line", "telegram")
	FileID     string `json:"file_id,omitempty"`     // Telegram file_id / platform media ID
	SetName    string `json:"set_name,omitempty"`    // Telegram sticker set name
	Emoji      string `json:"emoji,omitempty"`       // unicode emoji fallback
	IsAnimated bool   `json:"is_animated,omitempty"` // true for animated stickers (Telegram TGS)
	IsVideo    bool   `json:"is_video,omitempty"`    // true for video stickers (Telegram WEBM)
}

func NewSticker(packageID, stickerID string) *StickerComponent {
	return &StickerComponent{PackageID: packageID, StickerID: stickerID}
}

func (c *StickerComponent) Type() ComponentType { return ComponentSticker }
func (c *StickerComponent) ToPlainText() string {
	if c.Emoji != "" {
		return c.Emoji
	}
	return fmt.Sprintf("[贴图: packageId=%s, stickerId=%s]", c.PackageID, c.StickerID)
}
func (c *StickerComponent) ToJSON() map[string]any {
	m := map[string]any{
		"type":       string(c.Type()),
		"package_id": c.PackageID,
		"sticker_id": c.StickerID,
	}
	if c.URL != "" {
		m["url"] = c.URL
	}
	if c.Platform != "" {
		m["platform"] = c.Platform
	}
	if c.FileID != "" {
		m["file_id"] = c.FileID
	}
	if c.SetName != "" {
		m["set_name"] = c.SetName
	}
	if c.Emoji != "" {
		m["emoji"] = c.Emoji
	}
	if c.IsAnimated {
		m["is_animated"] = true
	}
	if c.IsVideo {
		m["is_video"] = true
	}
	return m
}

// StickerURL returns a renderable URL for the sticker. If URL is set, returns it;
// otherwise constructs one from the platform + IDs.
func (c *StickerComponent) StickerURL() string {
	if c.URL != "" {
		return c.URL
	}
	switch c.Platform {
	case "line":
		return fmt.Sprintf("https://stickershop.line-scdn.net/stickershop/v1/sticker/%s/iPhone/sticker.png", c.StickerID)
	case "telegram":
		// Telegram stickers use file_id API, not direct URLs; return empty for now.
		// The Telegram adapter handles file_id-based sending via sendSticker API.
		return ""
	default:
		return ""
	}
}

// ────────────────────────────────────────
// WechatEmoji 微信表情组件
// ────────────────────────────────────────

type WechatEmojiComponent struct {
	MD5    string `json:"md5"`
	MD5Len int    `json:"md5_len,omitempty"`
	CDNURL string `json:"cdnurl,omitempty"`
}

func NewWechatEmoji(md5 string) *WechatEmojiComponent {
	return &WechatEmojiComponent{MD5: md5}
}

func (c *WechatEmojiComponent) Type() ComponentType { return ComponentWechatEmoji }
func (c *WechatEmojiComponent) ToPlainText() string { return "[微信表情]" }
func (c *WechatEmojiComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type()), "md5": c.MD5}
	if c.MD5Len > 0 {
		m["md5_len"] = c.MD5Len
	}
	if c.CDNURL != "" {
		m["cdnurl"] = c.CDNURL
	}
	return m
}

// ────────────────────────────────────────
// Face QQ表情组件
// ────────────────────────────────────────

type FaceComponent struct {
	FaceID int    `json:"face_id"`        // QQ face emoji ID
	Name   string `json:"name,omitempty"` // display name
}

func NewFace(faceID int, name string) *FaceComponent {
	return &FaceComponent{FaceID: faceID, Name: name}
}

func (c *FaceComponent) Type() ComponentType { return ComponentFace }
func (c *FaceComponent) ToPlainText() string {
	if c.Name != "" {
		return "[" + c.Name + "]"
	}
	return fmt.Sprintf("[表情:%d]", c.FaceID)
}
func (c *FaceComponent) ToJSON() map[string]any {
	m := map[string]any{"type": string(c.Type()), "face_id": c.FaceID}
	if c.Name != "" {
		m["name"] = c.Name
	}
	return m
}

// ────────────────────────────────────────
// RichMessage 富消息 (组件列表)
// ────────────────────────────────────────

// RichMessage is a message composed of multiple components.
type RichMessage struct {
	Components []Component `json:"-"`
}

// NewRichMessage creates an empty rich message.
func NewRichMessage() *RichMessage { return &RichMessage{} }

// Add appends a component.
func (m *RichMessage) Add(c Component) *RichMessage {
	m.Components = append(m.Components, c)
	return m
}

// AddText is a convenience method to add plain text.
func (m *RichMessage) AddText(text string) *RichMessage {
	return m.Add(NewText(text))
}

// AddImage is a convenience method.
func (m *RichMessage) AddImage(url, alt string) *RichMessage {
	return m.Add(NewImageFromURL(url, alt))
}

// ToPlainText converts all components to a single plain text string.
func (m *RichMessage) ToPlainText() string {
	var sb strings.Builder
	for _, c := range m.Components {
		text := c.ToPlainText()
		if text != "" {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(text)
		}
	}
	return sb.String()
}

// TextContent extracts all text content from text components.
func (m *RichMessage) TextContent() string {
	var sb strings.Builder
	for _, c := range m.Components {
		if c.Type() == ComponentText {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(c.ToPlainText())
		}
	}
	return sb.String()
}

// HasType checks if the message contains a component of the given type.
func (m *RichMessage) HasType(t ComponentType) bool {
	for _, c := range m.Components {
		if c.Type() == t {
			return true
		}
	}
	return false
}

// GetFirst returns the first component of the given type, or nil.
func (m *RichMessage) GetFirst(t ComponentType) Component {
	for _, c := range m.Components {
		if c.Type() == t {
			return c
		}
	}
	return nil
}

// GetAll returns all components of the given type.
func (m *RichMessage) GetAll(t ComponentType) []Component {
	var result []Component
	for _, c := range m.Components {
		if c.Type() == t {
			result = append(result, c)
		}
	}
	return result
}

// ToJSON serializes the rich message to JSON.
func (m *RichMessage) ToJSON() string {
	var items []map[string]any
	for _, c := range m.Components {
		items = append(items, c.ToJSON())
	}
	b, _ := json.Marshal(items)
	return string(b)
}

// ParseRichMessage parses a JSON component array back.
func ParseRichMessage(data string) (*RichMessage, error) {
	var items []map[string]any
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		return nil, err
	}
	rm := NewRichMessage()
	for _, item := range items {
		t, _ := item["type"].(string)
		switch ComponentType(t) {
		case ComponentText:
			content, _ := item["content"].(string)
			rm.Add(NewText(content))
		case ComponentImage:
			url, _ := item["url"].(string)
			alt, _ := item["alt"].(string)
			img := NewImageFromURL(url, alt)
			if b64, ok := item["base64"].(string); ok {
				img.Base64 = b64
			}
			if fid, ok := item["file_id"].(string); ok {
				img.FileID = fid
			}
			rm.Add(img)
		case ComponentAudio:
			url, _ := item["url"].(string)
			dur, _ := item["duration"].(float64)
			a := NewAudio(url, int(dur))
			if fid, ok := item["file_id"].(string); ok {
				a.FileID = fid
			}
			rm.Add(a)
		case ComponentVideo:
			url, _ := item["url"].(string)
			dur, _ := item["duration"].(float64)
			rm.Add(NewVideo(url, int(dur)))
		case ComponentFile:
			url, _ := item["url"].(string)
			name, _ := item["file_name"].(string)
			rm.Add(NewFile(url, name))
		case ComponentAt:
			uid, _ := item["user_id"].(string)
			uname, _ := item["user_name"].(string)
			rm.Add(NewAt(uid, uname))
		case ComponentAtAll:
			rm.Add(NewAtAll())
		case ComponentReply:
			mid, _ := item["message_id"].(string)
			rm.Add(NewReply(mid))
		case ComponentButton:
			label, _ := item["label"].(string)
			value, _ := item["value"].(string)
			style, _ := item["style"].(string)
			rm.Add(NewButton(label, value, style))
		case ComponentLink:
			title, _ := item["title"].(string)
			url, _ := item["url"].(string)
			rm.Add(NewLink(title, url))
		case ComponentEmoji:
			id, _ := item["id"].(string)
			name, _ := item["name"].(string)
			rm.Add(NewEmoji(id, name))
		case ComponentSticker:
			pkgID, _ := item["package_id"].(string)
			stkID, _ := item["sticker_id"].(string)
			s := NewSticker(pkgID, stkID)
			if u, ok := item["url"].(string); ok {
				s.URL = u
			}
			if p, ok := item["platform"].(string); ok {
				s.Platform = p
			}
			if fid, ok := item["file_id"].(string); ok {
				s.FileID = fid
			}
			if sn, ok := item["set_name"].(string); ok {
				s.SetName = sn
			}
			if em, ok := item["emoji"].(string); ok {
				s.Emoji = em
			}
			if anim, ok := item["is_animated"].(bool); ok {
				s.IsAnimated = anim
			}
			if vid, ok := item["is_video"].(bool); ok {
				s.IsVideo = vid
			}
			rm.Add(s)
		case ComponentWechatEmoji:
			md5, _ := item["md5"].(string)
			we := NewWechatEmoji(md5)
			if ml, ok := item["md5_len"].(float64); ok {
				we.MD5Len = int(ml)
			}
			if cdn, ok := item["cdnurl"].(string); ok {
				we.CDNURL = cdn
			}
			rm.Add(we)
		case ComponentFace:
			fid, _ := item["face_id"].(float64)
			name, _ := item["name"].(string)
			rm.Add(NewFace(int(fid), name))
		}
	}
	return rm, nil
}
