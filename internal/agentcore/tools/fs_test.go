package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func setupFS(t *testing.T) (*FileSystem, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "tori-fs-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return NewFileSystem(dir), dir
}

func TestWrite(t *testing.T) {
	fs, _ := setupFS(t)
	res, err := fs.Write("hello.txt", "hello world\n")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created {
		t.Fatal("expected created=true")
	}
	if res.Written != 12 {
		t.Fatalf("expected 12 bytes, got %d", res.Written)
	}

	// Overwrite
	res2, err := fs.Write("hello.txt", "bye")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Created {
		t.Fatal("expected created=false on overwrite")
	}
}

func TestWriteNested(t *testing.T) {
	fs, _ := setupFS(t)
	_, err := fs.Write("sub/dir/file.txt", "nested")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	fs, _ := setupFS(t)
	fs.Write("test.txt", "line1\nline2\nline3\n")

	res, err := fs.Read("test.txt", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines != 4 { // 3 lines + trailing empty
		t.Fatalf("expected 4 lines, got %d", res.Lines)
	}
}

func TestReadWithOffsetLimit(t *testing.T) {
	fs, _ := setupFS(t)
	fs.Write("nums.txt", "1\n2\n3\n4\n5")

	res, err := fs.Read("nums.txt", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "2\n3" {
		t.Fatalf("expected '2\\n3', got %q", res.Content)
	}
}

func TestReadNotFound(t *testing.T) {
	fs, _ := setupFS(t)
	_, err := fs.Read("nope.txt", 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEdit(t *testing.T) {
	fs, _ := setupFS(t)
	fs.Write("code.go", "fmt.Println(\"old\")\nfmt.Println(\"old\")")

	res, err := fs.Edit("code.go", "old", "new", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Replaced != 1 {
		t.Fatalf("expected 1 replacement, got %d", res.Replaced)
	}

	r, _ := fs.Read("code.go", 0, 0)
	if r.Content != "fmt.Println(\"new\")\nfmt.Println(\"old\")" {
		t.Fatalf("unexpected content: %s", r.Content)
	}
}

func TestEditReplaceAll(t *testing.T) {
	fs, _ := setupFS(t)
	fs.Write("code.go", "foo bar foo baz foo")

	res, err := fs.Edit("code.go", "foo", "qux", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Replaced != 3 {
		t.Fatalf("expected 3, got %d", res.Replaced)
	}
}

func TestEditNotFound(t *testing.T) {
	fs, _ := setupFS(t)
	fs.Write("a.txt", "hello")

	_, err := fs.Edit("a.txt", "xyz", "abc", false)
	if err == nil {
		t.Fatal("expected error for not found string")
	}
}

func TestGrep(t *testing.T) {
	fs, dir := setupFS(t)
	fs.Write("a.go", "package main\nfunc Hello() {}\nfunc World() {}")
	fs.Write("b.txt", "nothing here")

	res, err := fs.Grep("func.*Hello", dir, false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 match, got %d", res.Total)
	}
	if res.Matches[0].Line != 2 {
		t.Fatalf("expected line 2, got %d", res.Matches[0].Line)
	}
}

func TestGrepFixedString(t *testing.T) {
	fs, dir := setupFS(t)
	fs.Write("log.txt", "ERROR: something failed\nINFO: all good\nERROR: again")

	res, err := fs.Grep("ERROR", dir, true, 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 {
		t.Fatalf("expected 2 matches, got %d", res.Total)
	}
}

func TestFind(t *testing.T) {
	fs, dir := setupFS(t)
	fs.Write("main.go", "package main")
	fs.Write("util.go", "package util")
	fs.Write("readme.md", "# readme")
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	fs.Write("sub/helper.go", "package sub")

	res, err := fs.Find("*.go", ".", 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 {
		t.Fatalf("expected 3 go files, got %d", res.Total)
	}
}

func TestLs(t *testing.T) {
	fs, dir := setupFS(t)
	fs.Write("file1.txt", "a")
	fs.Write("file2.txt", "bb")
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	res, err := fs.Ls(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(res.Entries))
	}

	dirCount := 0
	for _, e := range res.Entries {
		if e.IsDir {
			dirCount++
		}
	}
	if dirCount != 1 {
		t.Fatalf("expected 1 dir, got %d", dirCount)
	}
}

func TestLsNotFound(t *testing.T) {
	fs, _ := setupFS(t)
	_, err := fs.Ls("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveEscape(t *testing.T) {
	fs, _ := setupFS(t)
	_, err := fs.Read("../../etc/passwd", 0, 0)
	if err == nil {
		t.Fatal("expected path escape error")
	}
}
