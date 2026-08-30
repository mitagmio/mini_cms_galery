package generate

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func TestGeneratePanoramaGalleryLoad(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:   "beauty",
		Title:  "Beauty",
		Theme:  cms.ThemePanoramaGallery,
		Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	var blocks []cms.Block
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"} {
		blocks = append(blocks, cms.Block{
			Type: cms.BlockGalleryImage,
			Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/" + name, "alt": name}),
		})
	}
	if _, err := s.ReplaceBlocks(p.ID, blocks); err != nil {
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
	htmlb, err := os.ReadFile(filepath.Join(outDir, "beauty", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	for _, need := range []string{
		`class="theme_header`,
		`class="hide-for-small theme_menu"`,
		`data-mobile-nav=`,
		`mobile-nav.js`,
		`button-mobile-toggler`,
		`gallery-load.js`,
		`gallery-load.css`,
		`gallery-load-mark`,
		`asset-await`,
		`decoding="async"`,
		`data-src="/assets/cdn/a.jpg"`,
		`id="content"`,
	} {
		if !strings.Contains(html, need) {
			t.Fatalf("missing %q", need)
		}
	}
	if strings.Contains(html, `gallery_content gallery-await`) {
		t.Fatal("full-strip overlay retired; per-frame marks only")
	}
	if strings.Count(html, `class="gallery-load-mark"`) != 5 {
		t.Fatalf("want 5 per-frame loaders, got %d", strings.Count(html, `class="gallery-load-mark"`))
	}
	if strings.Contains(html, ` src="/assets/cdn/a.jpg"`) {
		t.Fatal("strip originals must not have src (avoid 40 parallel downloads)")
	}
	if strings.Contains(html, `loading="eager"`) {
		t.Fatal("strip photos must not emit loading=eager; JS assigns first-3")
	}
	menuAt := strings.Index(html, `id="menu"`)
	loaderAt := strings.Index(html, `class="gallery-load-mark"`)
	if menuAt < 0 || loaderAt < 0 || menuAt > loaderAt {
		t.Fatal("menu must paint in the header before the content loader")
	}
	if strings.Contains(html, `document.querySelectorAll('.asset img').forEach(function(img){img.style.opacity='1';})`) {
		t.Fatal("must not force-reveal photos before first-3 decode")
	}
}

func TestGenerateBAFirstPairLoad(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:       "before-after",
		Title:      "BA",
		Theme:      cms.ThemeBAContent,
		Status:     "draft",
		IsHomepage: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ReplaceBlocks(p.ID, []cms.Block{
		{Type: cms.BlockComparisonSlider, Data: cms.MustJSON(map[string]any{
			"before_url": "/assets/cdn/b1.jpg", "after_url": "/assets/cdn/a1.jpg",
		})},
		{Type: cms.BlockComparisonSlider, Data: cms.MustJSON(map[string]any{
			"before_url": "/assets/cdn/b2.jpg", "after_url": "/assets/cdn/a2.jpg",
		})},
	})
	if err != nil {
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
	htmlb, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	if !strings.Contains(html, `gallery-load.js`) || !strings.Contains(html, `gallery-load.css`) {
		t.Fatal("BA must include gallery-load")
	}
	if !strings.Contains(html, `ba-first`) || !strings.Contains(html, `ba-await`) {
		t.Fatal("first visible pair needs decode gate")
	}
	if strings.Count(html, `ba-pair ba-await`) != 2 {
		t.Fatalf("every BA pair needs a placeholder, got %d", strings.Count(html, `ba-pair ba-await`))
	}
	if strings.Count(html, `class="gallery-load-mark"`) != 2 {
		t.Fatalf("want per-pair loader, got %d", strings.Count(html, `class="gallery-load-mark"`))
	}
	if strings.Count(html, `fetchpriority="high"`) != 2 {
		t.Fatalf("first pair both sides should be high priority, got %d", strings.Count(html, `fetchpriority="high"`))
	}
	if strings.Count(html, `loading="lazy"`) != 2 {
		t.Fatalf("remaining pair should be lazy, got %d", strings.Count(html, `loading="lazy"`))
	}
	if !strings.Contains(html, `class="theme_header`) {
		t.Fatal("nav chrome required")
	}
}

func TestGeneratePanoramaUsesDisplayWebP(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	up := filepath.Join(t.TempDir(), "up")
	if err := os.MkdirAll(up, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "gallimg01deadbeef"
	orig := id + ".png"
	writeSolidPNG(t, filepath.Join(up, orig), color.NRGBA{R: 90, G: 110, B: 130, A: 255}, 3600, 4800)
	st, err := os.Stat(filepath.Join(up, orig))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMedia(cms.Media{
		ID: id, Filename: orig, URL: "/media/" + orig, SizeBytes: st.Size(), Mime: "image/png",
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug: "beauty-opt", Title: "Beauty", Theme: cms.ThemePanoramaGallery, Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceBlocks(p.ID, []cms.Block{
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{
			"media_id": id, "url": "/media/" + orig, "alt": "shot",
		})},
	}); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	g, err := New(s, Config{OutDir: outDir, UploadDir: up, PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(outDir, "beauty-opt", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	want := `/assets/cdn/` + id + `_display.webp`
	if !strings.Contains(html, want) {
		t.Fatalf("gallery must use display webp %q", want)
	}
	if strings.Contains(html, `/assets/cdn/`+orig) {
		t.Fatal("must not ship full original in panorama gallery")
	}
	cdn := filepath.Join(outDir, "assets", "cdn", id+"_display.webp")
	// copyMedia runs during GenerateSite; writePage alone may not copy — ensure generate created display in uploads
	disp := filepath.Join(up, id+"_display.webp")
	if _, err := os.Stat(disp); err != nil {
		t.Fatalf("display variant missing in uploads: %v", err)
	}
	_ = cdn
}

func TestGenerateResolvesDisplayByFilenameWhenIDMissing(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	up := filepath.Join(t.TempDir(), "up")
	if err := os.MkdirAll(up, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "orphan01deadbeef"
	orig := "140_subject-legacy.jpg"
	writeSolidPNG(t, filepath.Join(up, orig), color.NRGBA{R: 40, G: 80, B: 120, A: 255}, 2400, 3200)
	st, err := os.Stat(filepath.Join(up, orig))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMedia(cms.Media{
		ID: id, Filename: orig, OriginalName: orig, URL: "/media/" + orig, SizeBytes: st.Size(), Mime: "image/png",
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug: "product-opt", Title: "Product", Theme: cms.ThemePanoramaGallery, Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Stale media_id + legacy URL — resolve must still pick display via filename.
	if _, err := s.ReplaceBlocks(p.ID, []cms.Block{
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{
			"media_id": "deadbeef00000000", "url": "/media/" + orig, "alt": "legacy",
		})},
	}); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	g, err := New(s, Config{OutDir: outDir, UploadDir: up, PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(outDir, "product-opt", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	want := `/assets/cdn/` + id + `_display.webp`
	if !strings.Contains(html, want) {
		t.Fatalf("must resolve display via filename, want %q in html", want)
	}
	if strings.Contains(html, `/assets/cdn/`+orig) {
		t.Fatal("must not ship legacy original when display exists")
	}
}

