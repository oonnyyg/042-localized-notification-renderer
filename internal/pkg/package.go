package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"example.com/localized-notification-renderer/internal/catalog"
	"example.com/localized-notification-renderer/internal/msg"
)

// PackageVersion is the template package format version.
const PackageVersion = 1

// packedTemplate is the on-disk form of one localized message.
type packedTemplate struct {
	Segments []msg.Segment `json:"segments"`
}

// Package is the standalone compiled JSON template package.
type Package struct {
	Version         int                                  `json:"version"`
	DefaultLanguage string                               `json:"default_language"`
	Fallback        map[string]string                    `json:"fallback"`
	Templates       map[string]map[string]packedTemplate `json:"templates"`
}

// FromCompiled converts a validated compiled catalog into a serializable package.
func FromCompiled(c *catalog.Compiled) *Package {
	templates := make(map[string]map[string]packedTemplate, len(c.Messages))
	for id, byLang := range c.Messages {
		langs := make(map[string]packedTemplate, len(byLang))
		for lang, tpl := range byLang {
			langs[lang] = packedTemplate{Segments: tpl.Segments}
		}
		templates[id] = langs
	}
	return &Package{
		Version:         PackageVersion,
		DefaultLanguage: c.DefaultLanguage,
		Fallback:        c.Fallback,
		Templates:       templates,
	}
}

// WriteFile writes the package to path atomically: it is encoded to a temp
// file in the same directory and renamed into place only on success, so a
// failed compile never creates or overwrites an existing package.
func (p *Package) WriteFile(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode template package: %w", err)
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".noticerender-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp package file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp package file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp package file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install template package: %w", err)
	}
	return nil
}

// ReadFile loads and validates a template package.
func ReadFile(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template package: %w", err)
	}
	var p Package
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode template package: %w", err)
	}
	if p.Version != PackageVersion {
		return nil, fmt.Errorf("template package version %d is unsupported (want %d)", p.Version, PackageVersion)
	}
	if p.DefaultLanguage == "" {
		return nil, fmt.Errorf("template package: default_language is required")
	}
	if p.Templates == nil {
		return nil, fmt.Errorf("template package: templates are required")
	}
	return &p, nil
}
