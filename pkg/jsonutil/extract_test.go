package jsonutil

import (
	"testing"
)

// These tests codify the shared behaviour that used to be scattered
// across nine near-duplicate extractJSON helpers. New call sites should
// be able to rely on this contract without having to pick which
// lineage's corner cases they inherit.

func TestExtract_PureObject(t *testing.T) {
	got := Extract(`{"a":1,"b":2}`)
	want := `{"a":1,"b":2}`
	if got != want {
		t.Fatalf("Extract pure object: got %q want %q", got, want)
	}
}

func TestExtract_PureArray(t *testing.T) {
	got := Extract(`[1,2,3]`)
	want := `[1,2,3]`
	if got != want {
		t.Fatalf("Extract pure array: got %q want %q", got, want)
	}
}

func TestExtract_StripsJSONFence(t *testing.T) {
	got := Extract("Sure! Here you go:\n```json\n{\"a\":1}\n```")
	if got != `{"a":1}` {
		t.Fatalf("expected stripped ```json fence, got %q", got)
	}
}

func TestExtract_StripsGenericFence(t *testing.T) {
	got := Extract("```\n{\"a\":1}\n```")
	if got != `{"a":1}` {
		t.Fatalf("expected stripped ``` fence, got %q", got)
	}
}

func TestExtract_PrefersEarlierOpener(t *testing.T) {
	// Object comes first — should win even though an array follows.
	got := Extract(`noise {"a":1} then [2,3]`)
	if got != `{"a":1}` {
		t.Fatalf("expected object to win, got %q", got)
	}
	// Array comes first — should win.
	got = Extract(`noise [1,2] then {"a":1}`)
	if got != `[1,2]` {
		t.Fatalf("expected array to win, got %q", got)
	}
}

func TestExtract_NestedObjects(t *testing.T) {
	got := Extract(`prefix {"outer":{"inner":{"x":1}}} suffix`)
	want := `{"outer":{"inner":{"x":1}}}`
	if got != want {
		t.Fatalf("nested depth tracking wrong: got %q want %q", got, want)
	}
}

func TestExtract_UnbalancedReturnsFromOpener(t *testing.T) {
	// Missing closing brace — the helper should hand back everything from
	// the first opener so the caller's decoder can emit a deterministic
	// "unexpected EOF" error instead of silently succeeding on "{}".
	got := Extract(`prefix {"a":1`)
	if got != `{"a":1` {
		t.Fatalf("unbalanced: got %q", got)
	}
}

func TestExtract_NoJSONReturnsInput(t *testing.T) {
	in := "no braces here"
	if got := Extract(in); got != in {
		t.Fatalf("no-json Extract should return input, got %q", got)
	}
}

func TestExtract_EmptyString(t *testing.T) {
	if got := Extract(""); got != "" {
		t.Fatalf("empty input: got %q", got)
	}
}

func TestExtractObject_NoObjectReturnsEmptyObjectLiteral(t *testing.T) {
	if got := ExtractObject("no braces here"); got != `{}` {
		t.Fatalf("expected {} fallback, got %q", got)
	}
	if got := ExtractObject(""); got != `{}` {
		t.Fatalf("expected {} on empty, got %q", got)
	}
}

func TestExtractObject_IgnoresArrays(t *testing.T) {
	// ExtractObject must be strict: an input containing only an array
	// is "no object found" → "{}".
	if got := ExtractObject(`[1,2,3]`); got != `{}` {
		t.Fatalf("ExtractObject on array-only input: got %q", got)
	}
}

func TestExtractArray_NoArrayReturnsEmptyArrayLiteral(t *testing.T) {
	if got := ExtractArray("no brackets here"); got != `[]` {
		t.Fatalf("expected [] fallback, got %q", got)
	}
	if got := ExtractArray(""); got != `[]` {
		t.Fatalf("expected [] on empty, got %q", got)
	}
}

func TestExtractArray_NestedArrays(t *testing.T) {
	got := ExtractArray(`before [1,[2,[3]]] after`)
	want := `[1,[2,[3]]]`
	if got != want {
		t.Fatalf("nested array depth tracking: got %q want %q", got, want)
	}
}

type demoTarget struct {
	A int    `json:"a"`
	B string `json:"b"`
}

func TestUnmarshal_DirectPath(t *testing.T) {
	var got demoTarget
	if err := Unmarshal(`{"a":1,"b":"x"}`, &got); err != nil {
		t.Fatalf("direct unmarshal: unexpected error %v", err)
	}
	if got.A != 1 || got.B != "x" {
		t.Fatalf("direct unmarshal produced wrong value: %+v", got)
	}
}

func TestUnmarshal_FencePath(t *testing.T) {
	var got demoTarget
	in := "Sure:\n```json\n{\"a\":42,\"b\":\"y\"}\n```"
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("fenced unmarshal: unexpected error %v", err)
	}
	if got.A != 42 || got.B != "y" {
		t.Fatalf("fenced unmarshal wrong value: %+v", got)
	}
}

func TestUnmarshal_ProsePath(t *testing.T) {
	var got demoTarget
	if err := Unmarshal(`The answer is: {"a":7,"b":"z"} — hope this helps.`, &got); err != nil {
		t.Fatalf("prose unmarshal: unexpected error %v", err)
	}
	if got.A != 7 || got.B != "z" {
		t.Fatalf("prose unmarshal wrong value: %+v", got)
	}
}

func TestUnmarshal_ReturnsOriginalErrorOnFailure(t *testing.T) {
	var got demoTarget
	err := Unmarshal("this is not json at all", &got)
	if err == nil {
		t.Fatalf("expected an error for non-JSON input")
	}
}

func TestUnmarshal_NilPointerIsJSONError(t *testing.T) {
	// Passing target=nil should surface encoding/json's own error rather
	// than panic — the package contract promises "usual json.Unmarshal"
	// semantics for invalid targets.
	err := Unmarshal(`{"a":1}`, nil)
	if err == nil {
		t.Fatalf("expected error for nil target")
	}
}
