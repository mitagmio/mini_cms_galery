package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func TestGenerateAboutTwoColumnHero(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:   "about",
		Title:  "ABOUT",
		Theme:  cms.ThemeAboutContent,
		Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ReplaceBlocks(p.ID, []cms.Block{
		{Type: cms.BlockRichText, Data: cms.MustJSON(map[string]any{"html": "<p>Bio first on canvas</p>"})},
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/portrait.jpg", "alt": "Portrait", "caption": "Studio"})},
		{Type: cms.BlockRichText, Data: cms.MustJSON(map[string]any{"html": "<p>More copy below</p>"})},
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/extra.jpg", "alt": "Extra"})},
	})
	if err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	g, err := New(s, Config{OutDir: outDir, UploadDir: t.TempDir(), PreviewBase: "/preview", PathPrefix: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writeArticleCSS(); err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(outDir, "about", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	for _, need := range []string{
		`body class="about about-page content content_page simple simple_page"`,
		`class="hide-for-small theme_menu"`,
		`data-mobile-nav=`,
		`mobile-nav.js`,
		`about.css`,
		`class="about-shell"`,
		`class="about-hero"`,
		`class="about-hero__photo"`,
		`class="about-hero__copy"`,
		`src="/assets/cdn/portrait.jpg"`,
		`Bio first on canvas`,
		`More copy below`,
		`src="/assets/cdn/extra.jpg"`,
		`Studio`,
		`article-figure`,
	} {
		if !strings.Contains(html, need) {
			t.Fatalf("missing %q", need)
		}
	}
	if strings.Contains(html, `class="asset image`) || strings.Contains(html, `gallery-harden`) || strings.Contains(html, `lookbook-grid`) {
		t.Fatal("about must not use panorama/lookbook chrome")
	}
	if strings.Contains(html, `data-src="/assets/cdn/portrait.jpg"`) && !strings.Contains(html, `src="/assets/cdn/portrait.jpg"`) {
		t.Fatal("about portrait needs a real src")
	}
	photoAt := strings.Index(html, `about-hero__photo`)
	copyAt := strings.Index(html, `about-hero__copy`)
	restAt := strings.Index(html, `about-rest`)
	extraAt := strings.Index(html, `extra.jpg`)
	if photoAt < 0 || copyAt < 0 || photoAt > copyAt {
		t.Fatal("photo column must come before bio column")
	}
	if restAt < 0 || extraAt < restAt {
		t.Fatal("extra image must sit below the hero pair")
	}
	if !strings.Contains(html[restAt:], "More copy below") {
		t.Fatal("second rich_text belongs under the pair")
	}
	if _, err := os.Stat(filepath.Join(outDir, "assets", "theme", "about.css")); err != nil {
		t.Fatal("about.css should be written into the output tree")
	}
}

func TestGenerateTextContentArticleFigure(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{Slug: "note", Title: "Note", Theme: cms.ThemeTextContent, Status: "draft"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceBlocks(p.ID, []cms.Block{
		{Type: cms.BlockRichText, Data: cms.MustJSON(map[string]any{"html": "<p>Hello</p>"})},
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/shot.jpg", "alt": "Shot"})},
	}); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	g, err := New(s, Config{OutDir: outDir, UploadDir: t.TempDir(), PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(outDir, "note", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	if !strings.Contains(html, `src="/assets/cdn/shot.jpg"`) || !strings.Contains(html, `article-figure`) {
		t.Fatal("text_content should render gallery_image as an article figure")
	}
	if strings.Contains(html, `class="asset image`) {
		t.Fatal("text_content must not emit panorama frames")
	}
}

func TestEngineSourceAbout(t *testing.T) {
	src, err := EngineSource(cms.ThemeAboutContent)
	if err != nil || !strings.Contains(src, `{{define "about_content"}}`) {
		t.Fatalf("about source: %v", err)
	}
	if err := ValidateEngineSource(cms.ThemeAboutContent, src); err != nil {
		t.Fatal(err)
	}
}
