package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func testStore(t *testing.T) *cms.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := cms.Open(dir, filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestFisherYatesStable(t *testing.T) {
	blocks := make([]cms.Block, 6)
	for i := range blocks {
		blocks[i] = cms.Block{
			ID:   string(rune('a' + i)),
			Type: cms.BlockGalleryImage,
			Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/" + string(rune('a'+i)) + ".jpg", "alt": string(rune('a' + i))}),
		}
	}
	a := permuteGalleryBlocks(blocks, 42)
	b := permuteGalleryBlocks(blocks, 42)
	if len(a) != 6 {
		t.Fatalf("len %d", len(a))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("unstable at %d: %s vs %s", i, a[i].ID, b[i].ID)
		}
	}
	c := permuteGalleryBlocks(blocks, 43)
	same := true
	for i := range a {
		if a[i].ID != c[i].ID {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds should permute differently")
	}
}

func TestPermuteSkipsEmptyGalleryImages(t *testing.T) {
	blocks := []cms.Block{
		{ID: "empty", Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"media_id": nil, "alt": ""})},
		{ID: "ok", Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/x.jpg"})},
		{ID: "text", Type: cms.BlockRichText, Data: cms.MustJSON(map[string]any{"html": "<p>x</p>"})},
	}
	out := permuteGalleryBlocks(blocks, 1)
	if len(out) != 1 || out[0].ID != "ok" {
		t.Fatalf("%+v", out)
	}
}

func TestGenerateLookbookHTML(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:   "qa-lookbook",
		Title:  "Lookbook QA",
		Theme:  cms.ThemeLookbookGallery,
		Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ReplaceBlocks(p.ID, []cms.Block{
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/one.jpg", "alt": "one"})},
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"media_id": nil, "url": "", "alt": "empty"})},
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/two.jpg", "alt": "two"})},
		{Type: cms.BlockGalleryImage, Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/three.jpg", "alt": "three"})},
	})
	if err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	g, err := New(s, Config{OutDir: outDir, UploadDir: filepath.Join(t.TempDir(), "up"), PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}

	htmlb, err := os.ReadFile(filepath.Join(outDir, "qa-lookbook", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	for _, need := range []string{
		`body class="lookbook simple simple_page"`,
		`class="lookbook-grid"`,
		`class="theme_header`,
		`class="hide-for-small theme_menu"`,
		`stylesheet.css`,
		`id="lookbook-overlay"`,
		`id="assets"`,
		`id="content"`,
		`lookbook.css`,
		`lookbook-harden.css`,
		`lookbook.js`,
		`gallery-wheel.js`,
		`data-shuffle-seed=`,
		`class="lookbook-tile"`,
		`lookbook-nav-prev`,
		`lookbook-nav-next`,
		`class="asset image"`,
		`alt="one"`,
		`alt="two"`,
		`alt="three"`,
	} {
		if !strings.Contains(html, need) {
			t.Fatalf("missing %q in html", need)
		}
	}
	if strings.Contains(html, "gallery-harden.css") {
		t.Fatal("must not load unscoped gallery-harden.css")
	}
	if strings.Count(html, `id="assets"`) != 1 {
		t.Fatalf("want exactly one #assets, got %d", strings.Count(html, `id="assets"`))
	}
	if strings.Count(html, `id="content"`) != 1 {
		t.Fatalf("want exactly one #content, got %d", strings.Count(html, `id="content"`))
	}
	if strings.Contains(html, `alt="empty"`) {
		t.Fatal("empty gallery_image must be skipped")
	}
	if strings.Count(html, `class="lookbook-tile"`) != 3 {
		t.Fatalf("tiles=%d", strings.Count(html, `class="lookbook-tile"`))
	}
	if strings.Count(html, `class="asset image"`) != 3 {
		t.Fatalf("assets=%d", strings.Count(html, `class="asset image"`))
	}

	got, err := s.GetPage(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	seed := cms.ShuffleSeed(got.Settings)
	if seed == 0 {
		t.Fatal("generate should persist shuffle_seed")
	}

	out2 := t.TempDir()
	g2, err := New(s, Config{OutDir: out2, UploadDir: filepath.Join(t.TempDir(), "up"), PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g2.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	html2, _ := os.ReadFile(filepath.Join(out2, "qa-lookbook", "index.html"))
	if tileOrder(string(htmlb)) != tileOrder(string(html2)) {
		t.Fatal("same seed must keep tile order")
	}
}

func TestGenerateLookbookSameSeedStable(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:     "stable-look",
		Title:    "S",
		Theme:    cms.ThemeLookbookGallery,
		Status:   "draft",
		Settings: cms.MustJSON(map[string]any{"shuffle_seed": int64(12345)}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"}
	var blocks []cms.Block
	for _, id := range ids {
		blocks = append(blocks, cms.Block{
			Type: cms.BlockGalleryImage,
			Data: cms.MustJSON(map[string]any{"url": "/assets/cdn/" + id, "alt": id}),
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
	html, _ := os.ReadFile(filepath.Join(outDir, "stable-look", "index.html"))
	order := tileOrder(string(html))
	if strings.Count(order, ",") != 4 {
		t.Fatalf("order=%v", order)
	}
	// Not the editor order for this seed (very likely). If it happens to match, still OK
	// as long as a second generate matches.
	outDir2 := t.TempDir()
	g2, _ := New(s, Config{OutDir: outDir2, UploadDir: t.TempDir(), PreviewBase: "/preview"})
	_ = g2.writePage(mustGet(t, s, p.ID))
	html2, _ := os.ReadFile(filepath.Join(outDir2, "stable-look", "index.html"))
	if tileOrder(string(html2)) != order {
		t.Fatal("order changed")
	}
	got, _ := s.GetPage(p.ID)
	if cms.ShuffleSeed(got.Settings) != 12345 {
		t.Fatal("must not replace existing seed")
	}
}

func mustGet(t *testing.T, s *cms.Store, id string) cms.Page {
	t.Helper()
	p, err := s.GetPage(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func tileOrder(html string) string {
	const mark = `alt="`
	var b strings.Builder
	seen := map[string]bool{}
	for i := 0; i < len(html); {
		j := strings.Index(html[i:], mark)
		if j < 0 {
			break
		}
		i += j + len(mark)
		k := strings.Index(html[i:], `"`)
		if k < 0 {
			break
		}
		alt := html[i : i+k]
		if strings.HasSuffix(alt, ".jpg") && !seen[alt] {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			b.WriteString(alt)
			seen[alt] = true
		}
		i += k + 1
	}
	return b.String()
}
