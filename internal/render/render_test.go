package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"example.com/localized-notification-renderer/internal/catalog"
	tplpkg "example.com/localized-notification-renderer/internal/pkg"
)

func testRenderer(t *testing.T) *Renderer {
	t.Helper()
	c := &catalog.Catalog{
		DefaultLanguage: "en",
		Languages: map[string]*catalog.Lang{
			"en": {},
			"de": {Fallback: "en"},
			"at": {Fallback: "de"},
			"fr": {},
		},
		Entries: []catalog.Entry{
			{Template: "welcome", Language: "en", Text: "Hi {name}, code {{x}}"},
			{Template: "welcome", Language: "de", Text: "Hallo {name}, code {{x}}"},
			{Template: "plain", Language: "en", Text: "No params"},
		},
	}
	compiled, err := catalog.Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	return New(tplpkg.FromCompiled(compiled))
}

func TestRenderOne(t *testing.T) {
	r := testRenderer(t)

	res := r.RenderOne(1, Request{Template: "welcome", Language: "en", Params: map[string]string{"name": "Ada"}})
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if res.Output != "Hi Ada, code {x}" {
		t.Fatalf("output = %q", res.Output)
	}
	if res.Used != "en" || res.Fallback || len(res.Path) != 1 || res.Path[0] != "en" {
		t.Fatalf("unexpected metadata: %+v", res)
	}

	// Direct fallback: fr has no text, falls back to en.
	res = r.RenderOne(2, Request{Template: "welcome", Language: "fr", Params: map[string]string{"name": "Bé"}})
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if res.Used != "en" || !res.Fallback {
		t.Fatalf("expected fallback to en, got %+v", res)
	}
	if strings.Join(res.Path, ">") != "fr>en" {
		t.Fatalf("path = %v", res.Path)
	}

	// Multi-level fallback: at -> de (hit), stops and does not reach en.
	res = r.RenderOne(3, Request{Template: "welcome", Language: "at", Params: map[string]string{"name": "C"}})
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if res.Used != "de" || strings.Join(res.Path, ">") != "at>de" {
		t.Fatalf("expected at>de hit, got %+v", res)
	}
}

func TestRenderParamErrors(t *testing.T) {
	r := testRenderer(t)

	cases := []struct {
		name string
		req  Request
	}{
		{"missing", Request{Template: "welcome", Language: "en"}},
		{"extra", Request{Template: "welcome", Language: "en", Params: map[string]string{"name": "a", "bad": "b"}}},
		{"extra on plain", Request{Template: "plain", Language: "en", Params: map[string]string{"x": "y"}}},
		{"unknown template", Request{Template: "nope", Language: "en"}},
		{"unknown language", Request{Template: "welcome", Language: "zz"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := r.RenderOne(1, tc.req)
			if res.Error == "" {
				t.Fatalf("expected error, got %+v", res)
			}
		})
	}
}

func TestBracesInParamsAreLiteral(t *testing.T) {
	r := testRenderer(t)
	res := r.RenderOne(1, Request{
		Template: "welcome",
		Language: "en",
		Params:   map[string]string{"name": "{evil}{{x}}"},
	})
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if res.Output != "Hi {evil}{{x}}, code {x}" {
		t.Fatalf("output = %q", res.Output)
	}
}

func TestStreamMixed(t *testing.T) {
	r := testRenderer(t)
	input := strings.Join([]string{
		`{"template":"welcome","language":"at","params":{"name":"A"}}`,
		`{"template":"welcome","language":"en","params":{}}`,
		`{not json`,
		`{"template":"plain","language":"en","params":{}}`,
		``,
		`{"template":"welcome","language":"fr","params":{"name":"B"}}`,
	}, "\n")

	var out bytes.Buffer
	summary, err := r.Stream(strings.NewReader(input), &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary != (Summary{Total: 5, Success: 3, Failed: 2, FallbackUsed: 2}) {
		t.Fatalf("summary = %+v", summary)
	}

	var results []Result
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var res Result
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			t.Fatalf("bad result line %q: %v", line, err)
		}
		results = append(results, res)
	}
	if len(results) != 5 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].Line != 1 || results[1].Line != 2 || results[2].Line != 3 {
		t.Fatalf("line numbers not preserved: %+v", results)
	}
	if results[2].Error == "" {
		t.Fatalf("invalid JSON line should fail: %+v", results[2])
	}
}

func TestStreamEmpty(t *testing.T) {
	r := testRenderer(t)
	var out bytes.Buffer
	summary, err := r.Stream(strings.NewReader(""), &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary != (Summary{}) {
		t.Fatalf("summary = %+v, want zero", summary)
	}
	if out.Len() != 0 {
		t.Fatalf("expected empty output, got %q", out.String())
	}
}

func TestNonStringParamsRejected(t *testing.T) {
	r := testRenderer(t)
	var out bytes.Buffer
	input := `{"template":"welcome","language":"en","params":{"name":42}}` + "\n" +
		`{"template":"welcome","language":"en","params":{"name":null}}`
	summary, err := r.Stream(strings.NewReader(input), &out)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Failed != 2 || summary.Success != 0 {
		t.Fatalf("summary = %+v", summary)
	}
}
