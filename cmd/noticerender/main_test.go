package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodCatalog = `{
  "default_language": "en",
  "languages": {"zh-CN": "zh", "zh": ""},
  "templates": {
    "welcome": {
      "en": "Hello {name}!",
      "zh-CN": "{name}，你好！"
    }
  }
}`

const badCatalog = `{
  "default_language": "en",
  "languages": {"zh-CN": "zh", "zh": "zh-CN"},
  "templates": {
    "welcome": {"en": "Hello {name}!"}
  }
}`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompileFailureKeepsOldPackage(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	pkgPath := filepath.Join(dir, "package.json")

	writeFile(t, catalogPath, goodCatalog)
	if err := runCompile([]string{catalogPath, pkgPath}); err != nil {
		t.Fatalf("first compile: %v", err)
	}
	old, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, catalogPath, badCatalog)
	if err := runCompile([]string{catalogPath, pkgPath}); err == nil {
		t.Fatal("expected recompilation to fail on fallback cycle")
	}
	now, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("old package must still exist: %v", err)
	}
	if string(now) != string(old) {
		t.Fatal("old package was modified by the failed compile")
	}

	// The retained package still renders correctly, proving package
	// independence from later catalog edits.
	reqPath := filepath.Join(dir, "req.jsonl")
	writeFile(t, reqPath,
		`{"template":"welcome","language":"zh-CN","params":{"name":"A"}}`+"\n")
	resPath := filepath.Join(dir, "res.jsonl")
	sumPath := filepath.Join(dir, "sum.json")
	if err := runRender([]string{pkgPath, reqPath, resPath, sumPath}); err != nil {
		t.Fatal(err)
	}
	res := readFile(t, resPath)
	if !containsAll(res, `"used_language":"zh-CN"`, `"A，你好！"`) {
		t.Fatalf("unexpected results: %s", res)
	}
}

func TestRenderEmptyRequestFile(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	pkgPath := filepath.Join(dir, "package.json")
	writeFile(t, catalogPath, goodCatalog)
	if err := runCompile([]string{catalogPath, pkgPath}); err != nil {
		t.Fatal(err)
	}

	reqPath := filepath.Join(dir, "empty.jsonl")
	writeFile(t, reqPath, "")
	resPath := filepath.Join(dir, "res.jsonl")
	sumPath := filepath.Join(dir, "sum.json")
	if err := runRender([]string{pkgPath, reqPath, resPath, sumPath}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, resPath); got != "" {
		t.Fatalf("empty request file must produce empty results, got %q", got)
	}
	sum := readFile(t, sumPath)
	if !containsAll(sum, `"total": 0`, `"succeeded": 0`, `"failed": 0`, `"fallbacks": 0`) {
		t.Fatalf("unexpected summary: %s", sum)
	}
}

func TestCompileInvalidPlaceholderFails(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	pkgPath := filepath.Join(dir, "package.json")
	writeFile(t, catalogPath, `{
	  "default_language": "en",
	  "languages": {},
	  "templates": {"t": {"en": "Hi {1bad}"}}
	}`)
	if err := runCompile([]string{catalogPath, pkgPath}); err == nil {
		t.Fatal("expected compile error")
	}
	if _, err := os.Stat(pkgPath); !os.IsNotExist(err) {
		t.Fatalf("package must not be created, stat err = %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
