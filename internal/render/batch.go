package render

import (
	"bufio"
	"bytes"
	"io"

	"example.com/localized-notification-renderer/internal/catalog"
)

// Batch reads JSONL requests and renders each one in order. Blank lines are
// skipped but still consume a line number. The package is parsed once and
// reused for every request.
func Batch(p *catalog.Package, r io.Reader) ([]Result, Summary) {
	var results []Result
	var summary Summary

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		summary.Total++
		req, err := DecodeRequest(raw)
		var res Result
		if err != nil {
			res = Result{Line: lineNo, Error: err.Error()}
		} else {
			res = Render(p, req)
			res.Line = lineNo
		}
		if res.Error != "" {
			summary.Failed++
		} else {
			summary.Succeeded++
			if res.Fallback {
				summary.Fallbacks++
			}
		}
		results = append(results, res)
	}
	return results, summary
}
