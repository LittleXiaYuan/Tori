package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	content := `{
  "mcpServers": {
    "local-tool": {
      "command": "echo",
      "args": ["hello"],
      "active": true
    },
    "remote-sse": {
      "transport": "sse",
      "url": "http://example.com/sse",
      "headers": {"Authorization": "Bearer tok"},
      "timeout": 60
    },
    "remote-http": {
      "transport": "streamable_http",
      "url": "http://example.com/mcp"
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Servers) != 3 {
		t.Fatalf("servers count: got %d, want 3", len(cfg.Servers))
	}

	local := cfg.Servers["local-tool"]
	if local.Command != "echo" {
		t.Errorf("local-tool command: got %q", local.Command)
	}
	if len(local.Args) != 1 || local.Args[0] != "hello" {
		t.Errorf("local-tool args: got %v", local.Args)
	}

	sse := cfg.Servers["remote-sse"]
	if sse.Transport != "sse" {
		t.Errorf("remote-sse transport: got %q", sse.Transport)
	}
	if sse.URL != "http://example.com/sse" {
		t.Errorf("remote-sse url: got %q", sse.URL)
	}
	if sse.Timeout != 60 {
		t.Errorf("remote-sse timeout: got %d", sse.Timeout)
	}
	if sse.Headers["Authorization"] != "Bearer tok" {
		t.Errorf("remote-sse headers: got %v", sse.Headers)
	}

	http := cfg.Servers["remote-http"]
	if http.Transport != "streamable_http" {
		t.Errorf("remote-http transport: got %q", http.Transport)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/mcp.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid"), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadConfigActiveFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	content := `{
  "mcpServers": {
    "disabled": {
      "transport": "stdio",
      "command": "echo",
      "active": false
    },
    "enabled-explicit": {
      "transport": "stdio",
      "command": "echo",
      "active": true
    },
    "enabled-default": {
      "transport": "stdio",
      "command": "echo"
    }
  }
}`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	disabled := cfg.Servers["disabled"]
	if disabled.Active == nil || *disabled.Active != false {
		t.Error("disabled server should have active=false")
	}

	enabled := cfg.Servers["enabled-explicit"]
	if enabled.Active == nil || *enabled.Active != true {
		t.Error("enabled-explicit server should have active=true")
	}

	defaulted := cfg.Servers["enabled-default"]
	if defaulted.Active != nil {
		t.Error("enabled-default server should have nil active (defaults to active)")
	}
}

func TestConnectAllSkipsInactive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	// All servers inactive — ConnectAll should return 0 without trying to connect
	content := `{
  "mcpServers": {
    "disabled1": {
      "transport": "sse",
      "url": "http://localhost:19999/sse",
      "active": false
    },
    "disabled2": {
      "transport": "streamable_http",
      "url": "http://localhost:19999/mcp",
      "active": false
    }
  }
}`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	gw := NewGateway(nil, 5)
	n := ConnectAll(context.Background(), cfg, gw)
	if n != 0 {
		t.Fatalf("connected: got %d, want 0", n)
	}
}

func TestConnectAllNil(t *testing.T) {
	gw := NewGateway(nil, 5)
	if n := ConnectAll(context.Background(), nil, gw); n != 0 {
		t.Fatalf("nil cfg: got %d, want 0", n)
	}
	if n := ConnectAll(context.Background(), &MCPConfig{}, gw); n != 0 {
		t.Fatalf("empty cfg: got %d, want 0", n)
	}
}

func TestCreateProviderAutoDetect(t *testing.T) {
	// Auto-detect: URL → streamable_http
	_, err := createProvider(context.Background(), "test", ServerConfig{
		URL: "http://localhost:19999/nonexistent",
	})
	// Will fail to connect but should not fail on transport detection
	if err == nil {
		t.Log("provider created (unexpected success, maybe something listening)")
	}
	// The important thing is it didn't fail with "transport not specified"

	// Auto-detect: Command → stdio (will fail to start since "nonexistent-cmd" doesn't exist)
	_, err = createProvider(context.Background(), "test2", ServerConfig{
		Command: "nonexistent-cmd-that-does-not-exist",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}

	// No URL, no command → error
	_, err = createProvider(context.Background(), "test3", ServerConfig{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestCreateProviderUnknownTransport(t *testing.T) {
	_, err := createProvider(context.Background(), "test", ServerConfig{
		Transport: "grpc",
	})
	if err == nil {
		t.Fatal("expected error for unknown transport")
	}
}

func TestCreateProviderMissingFields(t *testing.T) {
	// stdio without command
	_, err := createProvider(context.Background(), "test", ServerConfig{
		Transport: "stdio",
	})
	if err == nil {
		t.Fatal("expected error for stdio without command")
	}

	// sse without url
	_, err = createProvider(context.Background(), "test", ServerConfig{
		Transport: "sse",
	})
	if err == nil {
		t.Fatal("expected error for sse without url")
	}

	// streamable_http without url
	_, err = createProvider(context.Background(), "test", ServerConfig{
		Transport: "streamable_http",
	})
	if err == nil {
		t.Fatal("expected error for streamable_http without url")
	}
}
