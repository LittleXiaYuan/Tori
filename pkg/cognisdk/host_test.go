package cognisdk

import (
	"context"
	"strings"
	"testing"
)

func TestHostAdapterBuildContext(t *testing.T) {
	adapter := NewHostAdapter(Config{})
	ctx := adapter.BuildContext(context.Background(), "你会永远陪我吗？", "tenant-a", "chat")
	if ctx == "" {
		t.Fatal("expected non-empty belief context")
	}
	if !strings.Contains(ctx, "## 内心状态") {
		t.Fatalf("missing markdown heading: %s", ctx)
	}
	if !strings.Contains(ctx, "comfort_with_truth") {
		t.Fatalf("missing disposition mode: %s", ctx)
	}
	if !strings.Contains(ctx, "永远不会离开你") {
		t.Fatalf("missing boundary phrase: %s", ctx)
	}
}
