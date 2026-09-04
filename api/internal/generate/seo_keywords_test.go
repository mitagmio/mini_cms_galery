package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func TestMetaKeywordsRenderedWhenSet(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{
		SiteName:      "Daria Sheyanova",
		CanonicalBase: "https://sheyanova.art",
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:         "beauty",
		Title:        "Beauty",
		Theme:        cms.ThemePanoramaGallery,
		Status:       "published",
		MetaKeywords: "beauty retoucher, cosmetic brand retouching, Daria Sheyanova",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	g, err := New(s, Config{OutDir: out, UploadDir: t.TempDir(), PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(out, "beauty", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	need := `<meta name="keywords" content="beauty retoucher, cosmetic brand retouching, Daria Sheyanova"/>`
	if !strings.Contains(html, need) {
		t.Fatalf("missing keywords meta, html head snippet:\n%s", html[:min(800, len(html))])
	}
}

func TestMetaKeywordsOmittedWhenEmpty(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{
		SiteName:      "Daria Sheyanova",
		CanonicalBase: "https://sheyanova.art",
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug: "contact", Title: "Contact", Theme: cms.ThemeTextContent, Status: "published",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	g, err := New(s, Config{OutDir: out, UploadDir: t.TempDir(), PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(out, "contact", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(htmlb), `name="keywords"`) {
		t.Fatal("empty keywords must not emit meta tag")
	}
}

func TestMetaKeywordsFallsBackToSiteDefault(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{
		SiteName:        "Daria Sheyanova",
		CanonicalBase:   "https://sheyanova.art",
		DefaultKeywords: "high-end retoucher, Daria Sheyanova",
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug: "rates", Title: "Rates", Theme: cms.ThemeRatesContent, Status: "published",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	g, err := New(s, Config{OutDir: out, UploadDir: t.TempDir(), PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(out, "rates", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	need := `<meta name="keywords" content="high-end retoucher, Daria Sheyanova"/>`
	if !strings.Contains(string(htmlb), need) {
		t.Fatalf("expected default keywords fallback")
	}
}
