package catalog

import (
	"strings"
	"testing"
)

func validCatalog() string {
	return `{
	  "default_language": "en",
	  "languages": {
	    "zh-CN": "zh",
	    "zh": "",
	    "fr": "en"
	  },
	  "templates": {
	    "welcome": {
	      "en": "Hello {name}!",
	      "zh-CN": "{name}，你好！",
	      "fr": "Bonjour {name} !"
	    },
	    "plain": {
	      "en": "Use {{ and }} literally"
	    }
	  }
	}`
}

func compileJSON(t *testing.T, data string) *Package {
	t.Helper()
	c, err := Decode([]byte(data))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	p, err := Compile(c)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return p
}

func TestCompileValid(t *testing.T) {
	p := compileJSON(t, validCatalog())
	if p.DefaultLang != "en" {
		t.Fatalf("default = %q", p.DefaultLang)
	}
	if p.Fallbacks["zh"] != "en" || p.Fallbacks["zh-CN"] != "zh" {
		t.Fatalf("fallbacks = %#v", p.Fallbacks)
	}
	chain := p.Chain("zh-CN")
	if strings.Join(chain, ",") != "zh-CN,zh,en" {
		t.Fatalf("chain = %v", chain)
	}
}

func TestCompileErrors(t *testing.T) {
	base := validCatalog()
	tests := []struct {
		name string
		edit func(string) string
		want string
	}{
		{
			name: "duplicate language key",
			edit: func(s string) string {
				return strings.Replace(s, `"fr": "en"`, `"fr": "en", "fr": "en"`, 1)
			},
			want: "duplicate key",
		},
		{
			name: "duplicate template-language registration",
			edit: func(s string) string {
				return strings.Replace(s, `"en": "Hello {name}!"`,
					`"en": "Hello {name}!", "en": "Hi {name}!"`, 1)
			},
			want: "duplicate key",
		},
		{
			name: "missing default text",
			edit: func(s string) string {
				return strings.Replace(s, `"en": "Use {{ and }} literally"`, `"fr": "x"`, 1)
			},
			want: "missing the default",
		},
		{
			name: "unknown language in texts",
			edit: func(s string) string {
				return strings.Replace(s, `"fr": "Bonjour {name} !"`,
					`"fr": "Bonjour {name} !", "de": "Hallo {name}!"`, 1)
			},
			want: "unknown language",
		},
		{
			name: "unknown fallback target",
			edit: func(s string) string {
				return strings.Replace(s, `"fr": "en"`, `"fr": "it"`, 1)
			},
			want: "unknown language",
		},
		{
			name: "fallback cycle",
			edit: func(s string) string {
				return strings.Replace(s, `"zh-CN": "zh",
	    "zh": ""`,
					`"zh-CN": "zh",
	    "zh": "zh-CN"`, 1)
			},
			want: "cycle",
		},
		{
			name: "default language fallback",
			edit: func(s string) string {
				return strings.Replace(s, `"languages": {`,
					`"languages": { "en": "",`, 1)
			},
			want: "cannot declare a fallback",
		},
		{
			name: "placeholder mismatch",
			edit: func(s string) string {
				return strings.Replace(s, `{name}，你好！`, `{user}，你好！`, 1)
			},
			want: "placeholders disagree",
		},
		{
			name: "illegal brace",
			edit: func(s string) string {
				return strings.Replace(s, "Hello {name}!", "Hello {name!", 1)
			},
			want: "not allowed in a name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.edit(base)
			c, err := Decode([]byte(data))
			if err == nil {
				_, err = Compile(c)
			}
			if err == nil {
				t.Fatal("expected compile error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestPackageRoundTrip(t *testing.T) {
	p := compileJSON(t, validCatalog())
	data, err := p.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	segs := loaded.Segments("plain", "en")
	if len(segs) != 1 || segs[0].Literal != "Use { and } literally" {
		t.Fatalf("segments = %#v", segs)
	}
}
