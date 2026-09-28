package catalog

import (
	"encoding/json"
	"fmt"

	"example.com/localized-notification-renderer/internal/tmpl"
)

// Marshal serializes the compiled package to indented JSON.
func (p *Package) Marshal() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// LoadPackage reads and minimally validates a compiled template package.
func LoadPackage(data []byte) (*Package, error) {
	var p Package
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid template package JSON: %w", err)
	}
	if p.Version != 1 {
		return nil, fmt.Errorf("unsupported template package version %d", p.Version)
	}
	if p.DefaultLang == "" {
		return nil, fmt.Errorf("template package is missing default_language")
	}
	return &p, nil
}

// Segments returns the parsed segments for a registered copy.
func (p *Package) Segments(id, lang string) []tmpl.Segment {
	texts, ok := p.Templates[id]
	if !ok {
		return nil
	}
	raw, ok := texts[lang]
	if !ok {
		return nil
	}
	segs := make([]tmpl.Segment, len(raw))
	for i, r := range raw {
		segs[i] = tmpl.Segment{Literal: r.Literal, Param: r.Param}
	}
	return segs
}

// KnowsLanguage reports whether lang is the default or a declared language.
func (p *Package) KnowsLanguage(lang string) bool {
	return lang == p.DefaultLang || p.HasLang(lang)
}

// HasLang reports whether lang appears in the language declarations.
func (p *Package) HasLang(lang string) bool {
	_, ok := p.Fallbacks[lang]
	return ok
}

// Chain returns the fallback lookup path starting at lang: the requested
// language first, then each declared fallback, ending at the default.
func (p *Package) Chain(lang string) []string {
	path := []string{lang}
	seen := map[string]bool{lang: true}
	for lang != p.DefaultLang {
		fb := p.Fallbacks[lang]
		if fb == "" {
			fb = p.DefaultLang
		}
		if seen[fb] { // defensive: Compile guarantees no cycles
			break
		}
		path = append(path, fb)
		if fb == p.DefaultLang {
			break
		}
		seen[fb] = true
		lang = fb
	}
	return path
}
