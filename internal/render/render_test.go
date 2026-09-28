package render

import (
	"strings"
	"testing"

	"example.com/localized-notification-renderer/internal/catalog"
)

func testPackage(t *testing.T) *catalog.Package {
	t.Helper()
	data := `{
	  "default_language": "en",
	  "languages": {"zh-CN": "zh", "zh": "", "de": "en"},
	  "templates": {
	    "welcome": {
	      "en": "Hello {name}!",
	      "zh": "{name}，你好！"
	    },
	    "code": {"en": "{{ {value} }}"}
	  }
	}`
	c, err := catalog.Decode([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	p, err := catalog.Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRenderFallbackChain(t *testing.T) {
	p := testPackage(t)

	res := Render(p, Request{Template: "welcome", Language: "zh-CN",
		Params: map[string]string{"name": "张三"}})
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if res.UsedLang != "zh" {
		t.Fatalf("used = %q, want zh", res.UsedLang)
	}
	if !res.Fallback || strings.Join(res.Path, ",") != "zh-CN,zh,en" {
		t.Fatalf("fallback=%v path=%v", res.Fallback, res.Path)
	}
	if res.Output != "张三，你好！" {
		t.Fatalf("output = %q", res.Output)
	}

	res = Render(p, Request{Template: "welcome", Language: "de",
		Params: map[string]string{"name": "Max"}})
	if res.Error != "" || res.UsedLang != "en" || !res.Fallback {
		t.Fatalf("unexpected: %+v", res)
	}
	if res.Output != "Hello Max!" {
		t.Fatalf("output = %q", res.Output)
	}

	res = Render(p, Request{Template: "welcome", Language: "en",
		Params: map[string]string{"name": "x"}})
	if res.Fallback || res.UsedLang != "en" {
		t.Fatalf("default language must not count as fallback: %+v", res)
	}
}

func TestRenderParamErrors(t *testing.T) {
	p := testPackage(t)

	res := Render(p, Request{Template: "welcome", Language: "en", Params: map[string]string{}})
	if !strings.Contains(res.Error, `missing required param "name"`) {
		t.Fatalf("got %q", res.Error)
	}

	res = Render(p, Request{Template: "welcome", Language: "en",
		Params: map[string]string{"name": "a", "extra": "b"}})
	if !strings.Contains(res.Error, `unexpected param "extra"`) {
		t.Fatalf("got %q", res.Error)
	}

	res = Render(p, Request{Template: "nope", Language: "en"})
	if !strings.Contains(res.Error, "unknown template") {
		t.Fatalf("got %q", res.Error)
	}

	res = Render(p, Request{Template: "welcome", Language: "ja"})
	if !strings.Contains(res.Error, "unknown target language") {
		t.Fatalf("got %q", res.Error)
	}
}

func TestDecodeRequestRejectsNonStringParams(t *testing.T) {
	if _, err := DecodeRequest([]byte(`{"template":"welcome","language":"en","params":{"name":7}}`)); err == nil {
		t.Fatal("expected error for number param")
	}
	if _, err := DecodeRequest([]byte(`{"template":"welcome","language":"en","params":{"name":null}}`)); err == nil {
		t.Fatal("expected error for null param")
	}
	req, err := DecodeRequest([]byte(`{"template":"welcome","language":"en","params":{"name":"{x}{{y}}"}}`))
	if err != nil {
		t.Fatal(err)
	}
	res := Render(testPackage(t), req)
	if res.Output != "Hello {x}{{y}}!" {
		t.Fatalf("braces in params must stay literal, got %q", res.Output)
	}
}

func TestBatchMixedResultsAndLineNumbers(t *testing.T) {
	p := testPackage(t)
	input := strings.Join([]string{
		`{"template":"welcome","language":"en","params":{"name":"A"}}`,
		``,
		`{"template":"welcome","language":"zh-CN","params":{"name":"B"}}`,
		`{"template":"welcome","language":"en","params":{}}`,
		`not json`,
	}, "\n")

	results, summary := Batch(p, strings.NewReader(input))
	if len(results) != 4 {
		t.Fatalf("got %d results", len(results))
	}
	wantLines := []int{1, 3, 4, 5}
	for i, res := range results {
		if res.Line != wantLines[i] {
			t.Fatalf("result %d line = %d, want %d", i, res.Line, wantLines[i])
		}
	}
	if summary.Total != 4 || summary.Succeeded != 2 || summary.Failed != 2 || summary.Fallbacks != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestBatchEmpty(t *testing.T) {
	p := testPackage(t)
	results, summary := Batch(p, strings.NewReader(""))
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
	if summary != (Summary{}) {
		t.Fatalf("expected zero summary, got %+v", summary)
	}
}
