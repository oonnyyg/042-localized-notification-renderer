package render

import (
	"bytes"
	"encoding/json"
	"fmt"

	"example.com/localized-notification-renderer/internal/catalog"
	"example.com/localized-notification-renderer/internal/tmpl"
)

// Request is one JSONL render request.
type Request struct {
	Template string            `json:"template"`
	Language string            `json:"language"`
	Params   map[string]string `json:"params"`
}

// Result is one JSONL render result. Exactly one of Output or Error is set.
type Result struct {
	Line     int      `json:"line"`
	Template string   `json:"template,omitempty"`
	Language string   `json:"language,omitempty"`
	Output   string   `json:"output,omitempty"`
	UsedLang string   `json:"used_language,omitempty"`
	Path     []string `json:"path,omitempty"`
	Fallback bool     `json:"fallback,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Summary is the aggregate report emitted after all requests.
type Summary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Fallbacks int `json:"fallbacks"`
}

// DecodeRequest parses one JSON object while enforcing that every params
// value is a string (numbers, booleans, null, objects and arrays rejected).
func DecodeRequest(line []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return req, fmt.Errorf("invalid JSON: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return req, fmt.Errorf("invalid JSON: %w", err)
	}
	if rawParams, ok := raw["params"]; ok {
		var paramsMap map[string]json.RawMessage
		if err := json.Unmarshal(rawParams, &paramsMap); err != nil {
			return req, fmt.Errorf("params must be an object of strings: %w", err)
		}
		if req.Params == nil {
			req.Params = map[string]string{}
		}
		for name, value := range paramsMap {
			trimmed := bytes.TrimSpace(value)
			if bytes.Equal(trimmed, []byte("null")) {
				return req, fmt.Errorf("param %q must be a string", name)
			}
			var s string
			if err := json.Unmarshal(value, &s); err != nil {
				return req, fmt.Errorf("param %q must be a string", name)
			}
			req.Params[name] = s
		}
	}
	return req, nil
}

// Render resolves and renders a single request against the package.
func Render(p *catalog.Package, req Request) Result {
	res := Result{Template: req.Template, Language: req.Language}
	if req.Template == "" {
		res.Error = "template id is required"
		return res
	}
	if req.Language == "" {
		res.Error = "target language is required"
		return res
	}
	if _, ok := p.Templates[req.Template]; !ok {
		res.Error = fmt.Sprintf("unknown template %q", req.Template)
		return res
	}
	if !p.KnowsLanguage(req.Language) {
		res.Error = fmt.Sprintf("unknown target language %q", req.Language)
		return res
	}

	path := p.Chain(req.Language)
	res.Path = path
	hit := -1
	for i, lang := range path {
		if p.Segments(req.Template, lang) != nil {
			hit = i
			break
		}
	}
	if hit < 0 {
		res.Error = fmt.Sprintf("template %q has no text along fallback path %v", req.Template, path)
		return res
	}
	res.UsedLang = path[hit]
	res.Fallback = hit > 0

	segs := p.Segments(req.Template, res.UsedLang)
	required := tmpl.Placeholders(segs)
	if err := checkParams(required, req.Params); err != nil {
		res.Error = err.Error()
		return res
	}
	res.Output = tmpl.Render(segs, req.Params)
	return res
}

func checkParams(required map[string]bool, provided map[string]string) error {
	for name := range required {
		if _, ok := provided[name]; !ok {
			return fmt.Errorf("missing required param %q", name)
		}
	}
	for name := range provided {
		if !required[name] {
			return fmt.Errorf("unexpected param %q", name)
		}
	}
	return nil
}
