package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"

	"example.com/localized-notification-renderer/internal/tmpl"
)

// Decode parses a catalog JSON document, rejecting duplicate object keys
// (in particular duplicate template ID/language registrations).
func Decode(data []byte) (*Catalog, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return nil, err
	}
	var c Catalog
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &c, nil
}

// rejectDuplicateKeys walks the whole JSON value and fails on repeated keys
// within any single object.
func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var check func() error
	check = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch d := tok.(type) {
		case json.Delim:
			if d == '{' {
				seen := map[string]bool{}
				for dec.More() {
					keyTok, err := dec.Token()
					if err != nil {
						return err
					}
					key := keyTok.(string)
					if seen[key] {
						return fmt.Errorf("duplicate key %q in JSON object", key)
					}
					seen[key] = true
					if err := check(); err != nil {
						return err
					}
				}
				if _, err := dec.Token(); err != nil {
					return err
				}
			} else if d == '[' {
				for dec.More() {
					if err := check(); err != nil {
						return err
					}
				}
				if _, err := dec.Token(); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return check()
}

// Compile validates the whole catalog and builds an independent template
// package. It returns an error if any rule is violated.
func Compile(c *Catalog) (*Package, error) {
	if c.DefaultLang == "" {
		return nil, fmt.Errorf("default_language is required")
	}
	if len(c.Templates) == 0 {
		return nil, fmt.Errorf("catalog registers no templates")
	}

	// Validate language declarations and resolve explicit fallbacks.
	known := map[string]bool{c.DefaultLang: true}
	fallbacks := map[string]string{}
	for lang, fb := range c.Languages {
		if lang == "" {
			return nil, fmt.Errorf("language code must not be empty")
		}
		if lang == c.DefaultLang {
			return nil, fmt.Errorf("default language %q cannot declare a fallback", lang)
		}
		if known[lang] {
			return nil, fmt.Errorf("language %q declared more than once", lang)
		}
		known[lang] = true
		if fb == "" {
			fb = c.DefaultLang
		}
		fallbacks[lang] = fb
	}
	for lang, fb := range fallbacks {
		if !known[fb] {
			return nil, fmt.Errorf("language %q falls back to unknown language %q", lang, fb)
		}
		if err := checkCycle(lang, fallbacks, c.DefaultLang); err != nil {
			return nil, err
		}
	}

	pkg := &Package{
		Version:      1,
		DefaultLang:  c.DefaultLang,
		Fallbacks:    fallbacks,
		Placeholders: map[string][]string{},
		Templates:    map[string]map[string][]RawSegment{},
	}

	for id, texts := range c.Templates {
		if id == "" {
			return nil, fmt.Errorf("template id must not be empty")
		}
		if len(texts) == 0 {
			return nil, fmt.Errorf("template %q has no texts", id)
		}
		defaultText, ok := texts[c.DefaultLang]
		if !ok {
			return nil, fmt.Errorf("template %q is missing the default %q text", id, c.DefaultLang)
		}
		defaultSegs, err := tmpl.Parse(defaultText)
		if err != nil {
			return nil, fmt.Errorf("template %q [%s]: %w", id, c.DefaultLang, err)
		}
		defaultSet := tmpl.Placeholders(defaultSegs)
		pkg.Placeholders[id] = tmpl.SortedNames(defaultSet)
		pkg.Templates[id] = map[string][]RawSegment{}

		for lang, text := range texts {
			if !known[lang] {
				return nil, fmt.Errorf("template %q references unknown language %q", id, lang)
			}
			segs, err := tmpl.Parse(text)
			if err != nil {
				return nil, fmt.Errorf("template %q [%s]: %w", id, lang, err)
			}
			if !tmpl.SamePlaceholders(defaultSegs, segs) {
				missing := setDiff(defaultSet, tmpl.Placeholders(segs))
				extra := setDiff(tmpl.Placeholders(segs), defaultSet)
				return nil, fmt.Errorf("template %q [%s] placeholders disagree with default: missing %v, extra %v",
					id, lang, missing, extra)
			}
			pkg.Templates[id][lang] = toRaw(segs)
		}
	}
	return pkg, nil
}

func checkCycle(start string, fallbacks map[string]string, def string) error {
	seen := map[string]bool{start: true}
	lang := start
	for {
		fb := fallbacks[lang]
		if fb == "" || fb == def {
			return nil
		}
		if seen[fb] {
			return fmt.Errorf("fallback cycle detected involving %q (chain reaches %q again)", start, fb)
		}
		seen[fb] = true
		lang = fb
	}
}

func setDiff(a, b map[string]bool) []string {
	diff := map[string]bool{}
	for name := range a {
		if !b[name] {
			diff[name] = true
		}
	}
	return tmpl.SortedNames(diff)
}

func toRaw(segs []tmpl.Segment) []RawSegment {
	raw := make([]RawSegment, len(segs))
	for i, seg := range segs {
		raw[i] = RawSegment{Literal: seg.Literal, Param: seg.Param}
	}
	return raw
}
