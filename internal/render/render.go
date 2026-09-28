package render

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"example.com/localized-notification-renderer/internal/msg"
	tplpkg "example.com/localized-notification-renderer/internal/pkg"
)

// Request is one JSONL render request.
type Request struct {
	Template string            `json:"template"`
	Language string            `json:"language"`
	Params   map[string]string `json:"params"`
}

// UnmarshalJSON enforces that every parameter value is a JSON string:
// numbers, booleans, objects and null are rejected rather than coerced.
func (req *Request) UnmarshalJSON(data []byte) error {
	var raw struct {
		Template string                 `json:"template"`
		Language string                 `json:"language"`
		Params   map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	req.Template = raw.Template
	req.Language = raw.Language
	req.Params = make(map[string]string, len(raw.Params))
	for name, value := range raw.Params {
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("param %q must be a string", name)
		}
		req.Params[name] = s
	}
	return nil
}

// Result is one JSONL render result, emitted in input order.
type Result struct {
	Line     int      `json:"line"`
	Template string   `json:"template,omitempty"`
	Language string   `json:"language,omitempty"`
	Used     string   `json:"used_language,omitempty"`
	Path     []string `json:"path,omitempty"`
	Fallback bool     `json:"fallback,omitempty"`
	Output   string   `json:"output,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Summary is the final aggregate report.
type Summary struct {
	Total        int `json:"total"`
	Success      int `json:"success"`
	Failed       int `json:"failed"`
	FallbackUsed int `json:"fallback_used"`
}

// Renderer renders requests against one loaded, immutable template package.
// It is safe to reuse for batch rendering: package data is never modified.
type Renderer struct {
	pkg *tplpkg.Package
}

// New creates a Renderer from a loaded package.
func New(p *tplpkg.Package) *Renderer {
	return &Renderer{pkg: p}
}

// RenderOne renders a single request and returns its result.
func (r *Renderer) RenderOne(line int, req Request) Result {
	res := Result{Line: line, Template: req.Template, Language: req.Language}

	byLang, ok := r.pkg.Templates[req.Template]
	if !ok {
		res.Error = fmt.Sprintf("unknown template %q", req.Template)
		return res
	}
	if _, known := r.pkg.Fallback[req.Language]; !known {
		res.Error = fmt.Sprintf("unknown language %q", req.Language)
		return res
	}

	path := []string{req.Language}
	lang := req.Language
	for {
		if tpl, ok := byLang[lang]; ok {
			res.Used = lang
			res.Path = path
			res.Fallback = lang != req.Language
			out, err := substitute(tpl.Segments, req.Params)
			if err != nil {
				res.Error = err.Error()
				res.Used = ""
				res.Path = nil
				res.Fallback = false
				return res
			}
			res.Output = out
			return res
		}
		next := r.pkg.Fallback[lang]
		if next == "" {
			res.Error = fmt.Sprintf("template %q has no text reachable from language %q", req.Template, req.Language)
			return res
		}
		path = append(path, next)
		lang = next
	}
}

func substitute(segments []msg.Segment, params map[string]string) (string, error) {
	required := msg.Placeholders(segments)
	if len(params) != len(required) {
		missing, extra := diffParams(required, params)
		switch {
		case len(missing) > 0 && len(extra) > 0:
			return "", fmt.Errorf("missing params %v and unexpected params %v", missing, extra)
		case len(missing) > 0:
			return "", fmt.Errorf("missing params %v", missing)
		default:
			return "", fmt.Errorf("unexpected params %v", extra)
		}
	}
	for name := range required {
		if _, ok := params[name]; !ok {
			return "", fmt.Errorf("missing params %v", []string{name})
		}
	}

	var b strings.Builder
	for _, seg := range segments {
		if seg.IsPlaceholder() {
			// Parameter values are inserted verbatim; their braces are never
			// reinterpreted as template syntax.
			b.WriteString(params[seg.Name])
		} else {
			b.WriteString(seg.Text)
		}
	}
	return b.String(), nil
}

func diffParams(required map[string]struct{}, params map[string]string) (missing, extra []string) {
	for name := range required {
		if _, ok := params[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range params {
		if _, ok := required[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

// Stream reads JSONL requests from r and writes one result JSON object per
// request line to w, in input order. Blank lines are ignored. It returns the
// aggregate summary.
func (r *Renderer) Stream(input io.Reader, output io.Writer) (Summary, error) {
	scanner := bufio.NewScanner(input)
	// Allow reasonably large lines (up to 4 MiB).
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(output)

	var summary Summary
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Bytes()
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}
		summary.Total++

		var req Request
		var res Result
		if err := json.Unmarshal(raw, &req); err != nil {
			res = Result{Line: lineNo, Error: fmt.Sprintf("invalid request JSON: %v", err)}
		} else {
			res = r.RenderOne(lineNo, req)
		}
		if res.Error != "" {
			summary.Failed++
		} else {
			summary.Success++
			if res.Fallback {
				summary.FallbackUsed++
			}
		}
		if err := enc.Encode(res); err != nil {
			return summary, fmt.Errorf("write result: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return summary, fmt.Errorf("read requests: %w", err)
	}
	return summary, nil
}
