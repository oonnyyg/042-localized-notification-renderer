package pkg

import (
	"path/filepath"
	"testing"

	"example.com/localized-notification-renderer/internal/catalog"
)

func sampleCompiled(t *testing.T) *catalog.Compiled {
	t.Helper()
	c := &catalog.Catalog{
		DefaultLanguage: "en",
		Languages: map[string]*catalog.Lang{
			"en": {},
			"zh": {Fallback: "en"},
		},
		Entries: []catalog.Entry{
			{Template: "welcome", Language: "en", Text: "Hi {name}"},
			{Template: "welcome", Language: "zh", Text: "你好 {name}"},
		},
	}
	compiled, err := catalog.Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func TestWriteReadRoundtrip(t *testing.T) {
	p := FromCompiled(sampleCompiled(t))
	path := filepath.Join(t.TempDir(), "pkg.json")
	if err := p.WriteFile(path); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tpl := got.Templates["welcome"]["zh"]
	if len(tpl.Segments) != 2 || tpl.Segments[1].Name != "name" {
		t.Fatalf("segments not preserved: %+v", tpl.Segments)
	}
}

func TestReadRejectsBadVersion(t *testing.T) {
	p := FromCompiled(sampleCompiled(t))
	p.Version = 99
	path := filepath.Join(t.TempDir(), "pkg.json")
	if err := p.WriteFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); err == nil {
		t.Fatal("expected version error")
	}
}
