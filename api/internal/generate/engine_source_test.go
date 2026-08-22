package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func TestValidateEngineSource(t *testing.T) {
	if err := ValidateEngineSource(cms.ThemeTextContent, ""); err != nil {
		t.Fatal(err)
	}
	src, err := EngineSource(cms.ThemeTextContent)
	if err != nil || !strings.Contains(src, `{{define "text_content"}}`) {
		t.Fatalf("file source: %v %s", err, src[:min(80, len(src))])
	}
	if err := ValidateEngineSource(cms.ThemeTextContent, src); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEngineSource(cms.ThemeTextContent, `{{define "nope"}}x{{end}}`); err == nil {
		t.Fatal("expected define name mismatch")
	}
	if err := ValidateEngineSource(cms.ThemeTextContent, `{{define "text_content"}}{{bogus }}{{end}}`); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestWritePageUsesDBSourceThenFallsBack(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:   "about-code",
		Title:  "ABOUT",
		Theme:  cms.ThemeTextContent,
		Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	override := `{{define "text_content"}}<!DOCTYPE html><html><body>CUSTOM_OVERRIDE_MARKER{{.HTMLTitle}}</body></html>{{end}}`
	if _, err := s.PatchTemplate(cms.ThemeTextContent, map[string]any{"source": override}); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	g, err := New(s, Config{OutDir: out, UploadDir: filepath.Join(t.TempDir(), "up"), PreviewBase: "/preview", PathPrefix: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	g.loadTemplateOverrides()
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(out, "about-code", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlb), "CUSTOM_OVERRIDE_MARKER") {
		t.Fatalf("want override in html: %s", htmlb)
	}

	if _, err := s.PatchTemplate(cms.ThemeTextContent, map[string]any{
		"source": `{{define "text_content"}}{{bogus }}{{end}}`,
	}); err != nil {
		t.Fatal(err)
	}
	g.loadTemplateOverrides()
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err = os.ReadFile(filepath.Join(out, "about-code", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(htmlb), "CUSTOM_OVERRIDE_MARKER") {
		t.Fatal("invalid override must fall back to embedded file")
	}
	if !strings.Contains(string(htmlb), "ABOUT") {
		t.Fatalf("fallback html missing title: %s", htmlb)
	}
}
