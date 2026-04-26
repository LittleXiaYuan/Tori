package cogni

import (
	"strings"
	"testing"
)

func TestDeclaration_Validate(t *testing.T) {
	tests := []struct {
		name    string
		decl    *Declaration
		wantErr string
	}{
		{
			name:    "nil declaration is rejected",
			decl:    nil,
			wantErr: "declaration is nil",
		},
		{
			name: "missing id is rejected",
			decl: &Declaration{
				DisplayName: "no-id",
			},
			wantErr: "cogni.id is required",
		},
		{
			name: "blank id (whitespace only) is rejected",
			decl: &Declaration{
				ID: "   \t",
			},
			wantErr: "cogni.id is required",
		},
		{
			name: "invalid regex is rejected",
			decl: &Declaration{
				ID: "bad-regex",
				Activation: ActivationRules{
					Regex: []string{"[invalid"},
				},
			},
			wantErr: "cogni.activation.regex",
		},
		{
			name: "min_score below 0 is rejected",
			decl: &Declaration{
				ID: "bad-min",
				Activation: ActivationRules{
					MinScore: -0.1,
				},
			},
			wantErr: "cogni.activation.min_score",
		},
		{
			name: "min_score above 1 is rejected",
			decl: &Declaration{
				ID: "bad-min-2",
				Activation: ActivationRules{
					MinScore: 1.5,
				},
			},
			wantErr: "cogni.activation.min_score",
		},
		{
			name: "minimal valid declaration",
			decl: &Declaration{ID: "ok"},
		},
		{
			name: "rich valid declaration",
			decl: &Declaration{
				ID:          "code-reviewer",
				DisplayName: "Code Reviewer",
				Capsule:     "code-reviewer",
				Priority:    50,
				Exclusive:   "review",
				Activation: ActivationRules{
					MinScore:      0.4,
					Keywords:      []string{"review", "审查"},
					KeywordWeight: 0.3,
					Regex:         []string{"^review\\s+#\\d+"},
					RegexWeight:   0.5,
					Channels:      []string{"webchat"},
					Tenants:       []string{"team-a"},
					HandoverOn:    []string{"need-review"},
				},
				Surface: ToolSurface{
					Only:         []string{"github_get_diff"},
					Include:      []string{"github_post_comment"},
					Exclude:      []string{"github_delete_comment"},
					FromCapsules: []string{"code-reviewer"},
					MaxTools:     8,
				},
				Context: ContextInjection{
					Static:      "你是一名资深代码审查员",
					MemoryQuery: "code review for {message}",
					MemoryTopK:  5,
					Template:    "上下文: {{.Message}}",
				},
				Memory: MemoryPolicy{
					Namespace: "code-reviewer",
					DropKeys:  []string{"secret"},
					TagAll:    map[string]string{"source": "review"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.decl.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}
