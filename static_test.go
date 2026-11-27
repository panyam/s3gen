package s3gen

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestRebuildCopiesStaticFolders(t *testing.T) {
	root := t.TempDir()
	static := filepath.Join(root, "static")
	writeFile(t, filepath.Join(static, "css", "main.css"), "body{}")
	writeFile(t, filepath.Join(static, "js", "gen", "app.js"), "x")
	out := filepath.Join(root, "dist")
	writeFile(t, filepath.Join(out, "static", "stale.js"), "old")
	if err := os.MkdirAll(filepath.Join(root, "content"), 0o755); err != nil {
		t.Fatal(err)
	}

	site := &Site{
		ContentRoot:   filepath.Join(root, "content"),
		OutputDir:     out,
		StaticFolders: []string{"/static/", static},
	}
	site.Rebuild(nil)

	if got := readFile(t, filepath.Join(out, "static", "css", "main.css")); got != "body{}" {
		t.Fatalf("static/css/main.css = %q", got)
	}
	if got := readFile(t, filepath.Join(out, "static", "js", "gen", "app.js")); got != "x" {
		t.Fatalf("static/js/gen/app.js = %q", got)
	}
	if _, err := os.Stat(filepath.Join(out, "static", "stale.js")); !os.IsNotExist(err) {
		t.Fatalf("stale.js from an earlier build is still there (err=%v)", err)
	}
}

func TestCopyStaticAtRootKeepsOtherOutput(t *testing.T) {
	root := t.TempDir()
	assets := filepath.Join(root, "public")
	writeFile(t, filepath.Join(assets, "robots.txt"), "User-agent: *")
	out := filepath.Join(root, "dist")
	writeFile(t, filepath.Join(out, "index.html"), "<p>page</p>")

	site := &Site{OutputDir: out, StaticFolders: []string{"/", assets}}
	if err := site.CopyStatic(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(out, "robots.txt")); got != "User-agent: *" {
		t.Fatalf("robots.txt = %q", got)
	}
	if got := readFile(t, filepath.Join(out, "index.html")); got != "<p>page</p>" {
		t.Fatalf("index.html was changed: %q", got)
	}
}

func TestCopyStaticSkipsMissingFolders(t *testing.T) {
	site := &Site{OutputDir: t.TempDir(), StaticFolders: []string{"/static/", filepath.Join(t.TempDir(), "nope")}}
	if err := site.CopyStatic(); err != nil {
		t.Fatalf("CopyStatic with a missing folder = %v, want nil", err)
	}
}

func TestCopyStaticNeverDeletesItsSource(t *testing.T) {
	root := t.TempDir()
	static := filepath.Join(root, "static")
	writeFile(t, filepath.Join(static, "keep.css"), "x")

	for _, out := range []string{"", root} {
		site := &Site{OutputDir: out, StaticFolders: []string{"/static/", static}}
		if err := site.CopyStatic(); err == nil {
			t.Errorf("OutputDir %q: CopyStatic = nil, want an error", out)
		}
		if got := readFile(t, filepath.Join(static, "keep.css")); got != "x" {
			t.Fatalf("OutputDir %q: source file changed: %q", out, got)
		}
	}
}
