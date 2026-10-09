package config

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Template is a named prompt template for sessions started from a grapes
// issue. Its text is a Go text/template; package prompt renders it.
type Template struct {
	Name string `toml:"name"`
	Text string `toml:"text,multiline"`
}

// examples are the templates a new templates directory starts with.
//
//go:embed templates/*.md
var examples embed.FS

// TemplatesDir returns the templates directory in dir.
func TemplatesDir(dir string) string { return filepath.Join(dir, "templates") }

// writeExamples creates templatesDir with the example templates, unless it
// exists.
func writeExamples(templatesDir string) error {
	if _, err := os.Stat(templatesDir); !os.IsNotExist(err) {
		return err
	}
	if err := os.Mkdir(templatesDir, 0o755); err != nil {
		return err
	}
	entries, err := examples.ReadDir("templates")
	if err != nil {
		return err
	}
	for _, e := range entries {
		content, err := examples.ReadFile("templates/" + e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(templatesDir, e.Name()), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// LoadTemplates returns the templates in dir's templates directory, one per
// <name>.md file, together with inline, the templates in the configuration,
// sorted by name. A name defined twice, or an empty one, is an error.
func LoadTemplates(dir string, inline []Template) ([]Template, error) {
	var templates []Template
	where := map[string]string{}
	add := func(t Template, source string) error {
		if t.Name == "" {
			return fmt.Errorf("a template in %s has no name", source)
		}
		if first, ok := where[t.Name]; ok {
			return fmt.Errorf("template %q is defined twice: in %s and %s", t.Name, first, source)
		}
		where[t.Name] = source
		templates = append(templates, t)
		return nil
	}

	entries, err := os.ReadDir(TemplatesDir(dir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok || e.IsDir() {
			continue
		}
		path := filepath.Join(TemplatesDir(dir), e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := add(Template{Name: name, Text: string(content)}, path); err != nil {
			return nil, err
		}
	}
	for _, t := range inline {
		if err := add(t, Path(dir)); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(templates, func(a, b Template) int { return strings.Compare(a.Name, b.Name) })
	return templates, nil
}
