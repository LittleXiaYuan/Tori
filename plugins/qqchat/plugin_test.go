package qqchat

import (
	"testing"
	"time"
)

func TestParseQQChat(t *testing.T) {
	input := `2024-01-15 14:32:05 小明
你好啊
最近怎么样

2024-01-15 14:32:30 小红(12345678)
挺好的，你呢？

2024-01-15 14:33:00 小明
我也不错，周末一起吃饭？

2024-01-15 14:33:15 小红(12345678)
好的
去哪吃呢
`
	records := ParseQQChat(input)

	if len(records) != 4 {
		t.Fatalf("expected 4 records, got %d", len(records))
	}

	// Check first record
	if records[0].Sender != "小明" {
		t.Errorf("record[0] sender = %q, want 小明", records[0].Sender)
	}
	if records[0].Content != "你好啊\n最近怎么样" {
		t.Errorf("record[0] content = %q", records[0].Content)
	}
	expect0 := time.Date(2024, 1, 15, 14, 32, 5, 0, time.UTC)
	if !records[0].Timestamp.Equal(expect0) {
		t.Errorf("record[0] timestamp = %v, want %v", records[0].Timestamp, expect0)
	}

	// Check second record (with QQ number)
	if records[1].Sender != "小红" {
		t.Errorf("record[1] sender = %q, want 小红", records[1].Sender)
	}
	if records[1].Content != "挺好的，你呢？" {
		t.Errorf("record[1] content = %q", records[1].Content)
	}

	// Check multiline in last record
	if records[3].Content != "好的\n去哪吃呢" {
		t.Errorf("record[3] content = %q, want '好的\\n去哪吃呢'", records[3].Content)
	}
}

func TestParseQQChatEmpty(t *testing.T) {
	records := ParseQQChat("这不是一个有效的聊天记录")
	if len(records) != 0 {
		t.Errorf("expected 0 records for invalid input, got %d", len(records))
	}
}

func TestParseQQChatSingleDigitHour(t *testing.T) {
	input := `2024-03-01 9:05:10 测试用户
早上好
`
	records := ParseQQChat(input)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Sender != "测试用户" {
		t.Errorf("sender = %q", records[0].Sender)
	}
}
