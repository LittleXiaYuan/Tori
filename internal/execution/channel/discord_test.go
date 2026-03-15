package channel

import (
	"testing"
)

func TestStripMention(t *testing.T) {
	cases := []struct {
		text, botID, want string
	}{
		{"<@123456> hello", "123456", "hello"},
		{"<@!123456> hi there", "123456", "hi there"},
		{"hello <@123456> world", "123456", "hello  world"},
		{"no mention here", "123456", "no mention here"},
		{"", "123456", ""},
	}
	for _, c := range cases {
		got := stripMention(c.text, c.botID)
		if got != c.want {
			t.Errorf("stripMention(%q, %q) = %q, want %q", c.text, c.botID, got, c.want)
		}
	}
}

func TestSplitDiscordMessage(t *testing.T) {
	// Short message
	chunks := splitDiscordMessage("hello")
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Fatalf("short message: expected 1 chunk 'hello', got %v", chunks)
	}

	// Long message
	long := ""
	for i := 0; i < 300; i++ {
		long += "abcdefghij" // 3000 chars
	}
	chunks = splitDiscordMessage(long)
	if len(chunks) < 2 {
		t.Fatalf("long message: expected multiple chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len([]rune(chunk)) > discordMaxLength {
			t.Fatalf("chunk exceeds max length: %d", len([]rune(chunk)))
		}
	}
}

func TestSplitDiscordMessageNewline(t *testing.T) {
	// Build a message with newlines that should split at newline boundary
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "This is a line of text that is about fifty characters long.")
	}
	text := ""
	for _, l := range lines {
		text += l + "\n"
	}

	chunks := splitDiscordMessage(text)
	for _, chunk := range chunks {
		if len([]rune(chunk)) > discordMaxLength {
			t.Fatalf("chunk exceeds max length: %d", len([]rune(chunk)))
		}
	}
}

func TestDiscordType(t *testing.T) {
	d := NewDiscord("fake-token")
	if d.Type() != "discord" {
		t.Fatalf("expected 'discord', got %s", d.Type())
	}
}

func TestDiscordIsDuplicate(t *testing.T) {
	d := NewDiscord("fake-token")
	if d.isDuplicate("msg1") {
		t.Fatal("first call should not be duplicate")
	}
	if !d.isDuplicate("msg1") {
		t.Fatal("second call should be duplicate")
	}
	if d.isDuplicate("msg2") {
		t.Fatal("different message should not be duplicate")
	}
}
