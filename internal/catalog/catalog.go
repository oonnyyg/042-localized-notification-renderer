package catalog

import (
	"fmt"
	"sort"

	"example.com/localized-notification-renderer/internal/msg"
)

// Entry is one registered message text for a template id and language.
type Entry struct {
	Template string `json:"template"`
	Language string `json:"language"`
	Text     string `json:"text"`
}

// Catalog is the source JSON template directory.
type Catalog struct {
	DefaultLanguage string           `json:"default_language"`
	Languages       map[string]*Lang `json:"languages"`
	Entries         []Entry          `json:"entries"`
}

// Lang describes a language and its optional single fallback language.
type Lang struct {
	Fallback string `json:"fallback,omitempty"`
}

// Template is a compiled message: parsed segments plus its placeholder set.
type Template struct {
	Segments     []msg.Segment
	Placeholders map[string]struct{}
}

// Compiled is the validated in-memory representation of a catalog.
type Compiled struct {
	DefaultLanguage string
	// Fallback[lang] is the language to consult next (empty for the default).
	Fallback map[string]string
	// Messages[templateID][lang] is the parsed template for that language.
	Messages map[string]map[string]Template
	// TemplateIDs lists all registered template ids, sorted.
	TemplateIDs []string
}

// Compile validates the whole catalog and returns its compiled form.
func Compile(c *Catalog) (*Compiled, error) {
	if c.DefaultLanguage == "" {
		return nil, fmt.Errorf("catalog: default_language is required")
	}
	if c.Languages == nil {
		return nil, fmt.Errorf("catalog: languages map is required")
	}
	defaultLang, ok := c.Languages[c.DefaultLanguage]
	if !ok {
		return nil, fmt.Errorf("catalog: default language %q is not declared in languages", c.DefaultLanguage)
	}
	if defaultLang.Fallback != "" {
		return nil, fmt.Errorf("catalog: default language %q must not declare a fallback", c.DefaultLanguage)
	}

	fallback := make(map[string]string, len(c.Languages))
	langs := make([]string, 0, len(c.Languages))
	for lang, spec := range c.Languages {
		langs = append(langs, lang)
		if spec == nil {
			spec = &Lang{}
		}
		if lang == c.DefaultLanguage {
			fallback[lang] = ""
			continue
		}
		if spec.Fallback == "" {
			fallback[lang] = c.DefaultLanguage
		} else {
			if _, known := c.Languages[spec.Fallback]; !known {
				return nil, fmt.Errorf("catalog: language %q references unknown fallback language %q", lang, spec.Fallback)
			}
			fallback[lang] = spec.Fallback
		}
	}
	sort.Strings(langs)

	for _, lang := range langs {
		if err := checkChain(lang, fallback); err != nil {
			return nil, err
		}
	}

	seen := make(map[string]map[string]bool)
	messages := make(map[string]map[string]Template)
	for i, e := range c.Entries {
		if e.Template == "" {
			return nil, fmt.Errorf("catalog: entry %d: template id is required", i)
		}
		if e.Language == "" {
			return nil, fmt.Errorf("catalog: entry %d (%q): language is required", i, e.Template)
		}
		if _, known := c.Languages[e.Language]; !known {
			return nil, fmt.Errorf("catalog: entry %d (%q): unknown language %q", i, e.Template, e.Language)
		}
		byLang := seen[e.Template]
		if byLang == nil {
			byLang = make(map[string]bool)
			seen[e.Template] = byLang
		}
		if byLang[e.Language] {
			return nil, fmt.Errorf("catalog: duplicate entry for template %q language %q", e.Template, e.Language)
		}
		byLang[e.Language] = true

		segments, err := msg.Parse(e.Text)
		if err != nil {
			return nil, fmt.Errorf("catalog: template %q language %q: %w", e.Template, e.Language, err)
		}
		tpl := Template{Segments: segments, Placeholders: msg.Placeholders(segments)}
		if messages[e.Template] == nil {
			messages[e.Template] = make(map[string]Template)
		}
		messages[e.Template][e.Language] = tpl
	}

	ids := make([]string, 0, len(messages))
	for id := range messages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		defaultTpl, ok := messages[id][c.DefaultLanguage]
		if !ok {
			return nil, fmt.Errorf("catalog: template %q has no text in default language %q", id, c.DefaultLanguage)
		}
		for _, lang := range langs {
			tpl, ok := messages[id][lang]
			if !ok {
				continue
			}
			if !sameSet(tpl.Placeholders, defaultTpl.Placeholders) {
				return nil, fmt.Errorf("catalog: template %q language %q placeholders %v do not match default %v",
					id, lang, sortedKeys(tpl.Placeholders), sortedKeys(defaultTpl.Placeholders))
			}
		}
	}

	return &Compiled{
		DefaultLanguage: c.DefaultLanguage,
		Fallback:        fallback,
		Messages:        messages,
		TemplateIDs:     ids,
	}, nil
}

func checkChain(start string, fallback map[string]string) error {
	visited := map[string]bool{start: true}
	lang := fallback[start]
	for lang != "" {
		if visited[lang] {
			return fmt.Errorf("catalog: fallback cycle detected involving %q (chain from %q)", lang, start)
		}
		visited[lang] = true
		lang = fallback[lang]
	}
	return nil
}

func sameSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
