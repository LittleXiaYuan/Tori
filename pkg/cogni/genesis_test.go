package cogni

import (
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "plain json",
			input: `{"id": "test", "activation": {}}`,
			want:  true,
		},
		{
			name: "markdown code block",
			input: "```json\n{\"id\": \"test\"}\n```",
			want: true,
		},
		{
			name: "text before json",
			input: "Here is the result:\n{\"id\": \"test\"}",
			want: true,
		},
		{
			name:  "no json",
			input: "no json here",
			want:  false,
		},
		{
			name:  "empty",
			input: "",
			want:  false,
		},
		{
			name:  "nested braces",
			input: `{"id": "test", "context": {"static": "hello {name}"}}`,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if tt.want && result == "" {
				t.Error("expected JSON to be extracted, got empty")
			}
			if !tt.want && result != "" {
				t.Errorf("expected empty, got %q", result)
			}
		})
	}
}
