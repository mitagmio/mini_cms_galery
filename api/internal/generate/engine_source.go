package generate

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"sheyanova.art/api/internal/cms"
)

var defineNameRe = regexp.MustCompile(`\{\{-?\s*define\s+["']([^"']+)["']`)

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"trimSlash": func(s string) string {
			return strings.Trim(strings.TrimPrefix(s, "/"), "/")
		},
	}
}

// EngineSource returns the embedded gohtml for a page engine (ba_content, …).
func EngineSource(theme string) (string, error) {
	theme = strings.TrimSpace(theme)
	if !cms.ValidTheme(theme) {
		return "", fmt.Errorf("unknown engine")
	}
	b, err := templateFS.ReadFile("templates/" + theme + ".gohtml")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ValidateEngineSource checks a DB override parses and defines the engine name.
func ValidateEngineSource(theme, src string) error {
	theme = strings.TrimSpace(theme)
	src = strings.TrimSpace(src)
	if src == "" {
		return nil
	}
	if !cms.ValidTheme(theme) {
		return fmt.Errorf("unknown engine")
	}
	if !hasDefineName(src, theme) {
		return fmt.Errorf("generate template must contain {{define %q}}", theme)
	}
	t, err := template.New("root").Funcs(templateFuncMap()).Parse(src)
	if err != nil {
		return fmt.Errorf("template parse: %w", err)
	}
	if t.Lookup(theme) == nil {
		return fmt.Errorf("template must define %q", theme)
	}
	return nil
}

func hasDefineName(src, name string) bool {
	for _, m := range defineNameRe.FindAllStringSubmatch(src, -1) {
		if len(m) > 1 && m[1] == name {
			return true
		}
	}
	return false
}
