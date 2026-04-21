package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandBaseName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"rm", "rm"},
		{"RM", "rm"},
		{"/usr/bin/rm", "rm"},
		{"C:\\Windows\\System32\\cmd.exe", "cmd"},
		{"cmd.exe", "cmd"},
		{"echo.bat", "echo"},
		{"setup.cmd", "setup"},
		{"msdos.com", "msdos"},
		{filepath.Join("a", "b", "Python3"), "python3"},
	}
	for _, c := range cases {
		got := commandBaseName(c.in)
		if got != c.want {
			t.Errorf("commandBaseName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDangerousArgvCombo_RmRecursiveRoot(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		args    []string
		wantHit bool
	}{
		{"rm -rf /", "rm", []string{"-rf", "/"}, true},
		{"rm -fr /", "rm", []string{"-fr", "/"}, true},
		{"rm --recursive --force /", "rm", []string{"--recursive", "--force", "/"}, true},
		{"rm -r /", "rm", []string{"-r", "/"}, true},
		{"rm -rf /*", "rm", []string{"-rf", "/*"}, true},
		{"rm -rf /.", "rm", []string{"-rf", "/."}, true},
		{"rm -rf C:\\", "rm", []string{"-rf", "C:\\"}, true},
		{"rm -rf c:\\windows (case)", "rm", []string{"-rf", "C:\\Windows"}, true},
		{"rm /tmp/scratch (no -r)", "rm", []string{"/tmp/scratch"}, false},
		{"rm -rf /tmp/scratch (not root)", "rm", []string{"-rf", "/tmp/scratch"}, false},
		{"rm -rf ./build (relative)", "rm", []string{"-rf", "./build"}, false},
		{"cat /tmp/notes-about-rm-rf.txt (substring red herring)", "cat", []string{"/tmp/notes-about-rm-rf.txt"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := dangerousArgvCombo(c.base, c.args)
			gotHit := reason != ""
			if gotHit != c.wantHit {
				t.Fatalf("dangerousArgvCombo(%q, %v) = %q (hit=%v), want hit=%v", c.base, c.args, reason, gotHit, c.wantHit)
			}
		})
	}
}

func TestDangerousArgvCombo_WindowsDelRd(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		args    []string
		wantHit bool
	}{
		{"del /s /q C:\\", "del", []string{"/s", "/q", "C:\\"}, true},
		{"del /s C:\\*", "del", []string{"/s", "C:\\*"}, true},
		{"erase /s /q C:\\", "erase", []string{"/s", "/q", "C:\\"}, true},
		{"rd /s /q C:\\", "rd", []string{"/s", "/q", "C:\\"}, true},
		{"rmdir /s /q C:\\", "rmdir", []string{"/s", "/q", "C:\\"}, true},
		{"del /q C:\\file (no /s)", "del", []string{"/q", "C:\\file"}, false},
		{"del /s D:\\scratch (not C root)", "del", []string{"/s", "/q", "D:\\scratch"}, false},
		{"rd /s C:\\scratch (not root)", "rd", []string{"/s", "/q", "C:\\scratch"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := dangerousArgvCombo(c.base, c.args)
			gotHit := reason != ""
			if gotHit != c.wantHit {
				t.Fatalf("dangerousArgvCombo(%q, %v) = %q (hit=%v), want hit=%v", c.base, c.args, reason, gotHit, c.wantHit)
			}
		})
	}
}

// TestSandbox_BlockedByBaseName verifies the base-name blocklist matches the
// resolved binary name regardless of full path or .exe suffix.
func TestSandbox_BlockedByBaseName(t *testing.T) {
	policy := DefaultPolicy()
	policy.BlockCommands = []string{"curl"}
	policy.AllowCommands = nil
	sb, err := New(os.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	for _, cmd := range []string{"curl", "/usr/bin/curl", "C:\\Tools\\curl.exe", "CURL"} {
		res, err := sb.Exec(context.Background(), cmd, "https://example.com")
		if err != nil {
			t.Fatalf("Exec(%q) err: %v", cmd, err)
		}
		if res.ExitCode != -1 || !strings.Contains(res.Stderr, "blocked command") {
			t.Fatalf("Exec(%q) expected blocked, got exit=%d stderr=%q", cmd, res.ExitCode, res.Stderr)
		}
	}
}

// TestSandbox_SubstringFalsePositiveFixed ensures the old substring-based
// blocklist no longer triggers false positives — running `cat` on a path that
// merely *mentions* "rm" (or "rm -rf /") should not be blocked.
func TestSandbox_SubstringFalsePositiveFixed(t *testing.T) {
	tmp := t.TempDir()
	scratch := filepath.Join(tmp, "notes-about-rm-rf.txt")
	if err := os.WriteFile(scratch, []byte("just a note\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultPolicy()
	policy.HostReadPaths = []string{tmp}
	sb, err := New(os.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	res, err := sb.Exec(context.Background(), "cat", scratch)
	if err != nil {
		t.Fatalf("Exec err: %v", err)
	}

	// `cat` is in DefaultPolicy AllowCommands; on Windows it may not exist on
	// PATH so the binary lookup will fail with non-zero exit. What we
	// specifically care about is that the policy layer did not return our
	// "blocked" sentinel — i.e. the substring "rm -rf" inside the path did
	// not trip the blocklist anymore.
	if strings.Contains(res.Stderr, "blocked command") {
		t.Fatalf("substring-style false positive still present: %q", res.Stderr)
	}
	if strings.Contains(res.Stderr, "blocked dangerous argv combination") {
		t.Fatalf("argv-combo check misfired on read-only `cat`: %q", res.Stderr)
	}
}
