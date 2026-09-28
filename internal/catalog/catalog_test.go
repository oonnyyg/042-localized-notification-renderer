package catalog

import "testing"

func validCatalog() *Catalog {
	return &Catalog{
		DefaultLanguage: "en",
		Languages: map[string]*Lang{
			"en": {},
			"zh": {Fallback: "en"},
			"fr": {},
		},
		Entries: []Entry{
			{Template: "welcome", Language: "en", Text: "Hi {name}"},
			{Template: "welcome", Language: "zh", Text: "你好 {name}"},
			{Template: "farewell", Language: "en", Text: "Bye {name}"},
		},
	}
}

func TestCompileOK(t *testing.T) {
	c, err := Compile(validCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if c.Fallback["fr"] != "en" {
		t.Fatalf("fr fallback = %q, want en", c.Fallback["fr"])
	}
	if c.Fallback["zh"] != "en" {
		t.Fatalf("zh fallback = %q, want en", c.Fallback["zh"])
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Catalog)
	}{
		{"missing default language", func(c *Catalog) { c.DefaultLanguage = "" }},
		{"default not declared", func(c *Catalog) { c.DefaultLanguage = "de" }},
		{"default has fallback", func(c *Catalog) { c.Languages["en"] = &Lang{Fallback: "fr"} }},
		{"unknown fallback", func(c *Catalog) { c.Languages["zh"] = &Lang{Fallback: "de"} }},
		{"direct fallback cycle", func(c *Catalog) {
			c.Languages["zh"] = &Lang{Fallback: "fr"}
			c.Languages["fr"] = &Lang{Fallback: "zh"}
		}},
		{"multi level cycle", func(c *Catalog) {
			c.Languages["de"] = &Lang{Fallback: "fr"}
			c.Languages["zh"] = &Lang{Fallback: "de"}
			c.Languages["fr"] = &Lang{Fallback: "zh"}
		}},
		{"duplicate entry", func(c *Catalog) {
			c.Entries = append(c.Entries, Entry{Template: "welcome", Language: "en", Text: "again"})
		}},
		{"entry unknown language", func(c *Catalog) {
			c.Entries = append(c.Entries, Entry{Template: "welcome", Language: "de", Text: "x"})
		}},
		{"missing default text", func(c *Catalog) {
			c.Entries = []Entry{{Template: "welcome", Language: "zh", Text: "你好 {name}"}}
		}},
		{"placeholder mismatch", func(c *Catalog) {
			c.Entries[1].Text = "你好 {who}"
		}},
		{"placeholder extra", func(c *Catalog) {
			c.Entries = append(c.Entries, Entry{Template: "farewell", Language: "fr", Text: "Bye {name} {date}"})
		}},
		{"illegal braces", func(c *Catalog) {
			c.Entries[0].Text = "Hi {name"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCatalog()
			tt.mut(c)
			if _, err := Compile(c); err == nil {
				t.Fatal("expected compile error, got nil")
			}
		})
	}
}

func TestMultiLevelFallback(t *testing.T) {
	c := &Catalog{
		DefaultLanguage: "en",
		Languages: map[string]*Lang{
			"en": {},
			"de": {Fallback: "en"},
			"at": {Fallback: "de"},
		},
		Entries: []Entry{
			{Template: "t", Language: "en", Text: "EN {x}"},
			{Template: "t", Language: "de", Text: "DE {x}"},
		},
	}
	compiled, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Fallback["at"] != "de" || compiled.Fallback["de"] != "en" {
		t.Fatalf("unexpected fallbacks: %v", compiled.Fallback)
	}
}
