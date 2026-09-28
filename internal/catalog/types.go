package catalog

// Catalog is the on-disk template directory in JSON form.
type Catalog struct {
	// DefaultLang is the language every template must provide a copy in.
	DefaultLang string `json:"default_language"`
	// Languages maps each non-default language to its fallback language.
	// An empty string (or an omitted entry) means "fall back to the default".
	Languages map[string]string `json:"languages"`
	// Templates maps a template ID to language -> template text.
	Templates map[string]map[string]string `json:"templates"`
}

// Package is the compiled, self-contained JSON template package. It stores
// parsed segments so that repeated batch renders never re-parse the source
// catalog.
type Package struct {
	Version      int                                `json:"version"`
	DefaultLang  string                             `json:"default_language"`
	Fallbacks    map[string]string                  `json:"fallbacks"`
	Placeholders map[string][]string                `json:"placeholders"`
	Templates    map[string]map[string][]RawSegment `json:"templates"`
}

// RawSegment is the JSON encoding of a parsed template segment.
type RawSegment struct {
	Literal string `json:"literal,omitempty"`
	Param   string `json:"param,omitempty"`
}
