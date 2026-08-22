package cms

import (
	"strings"
	"testing"
)

func TestValidThemeAbout(t *testing.T) {
	if !ValidTheme(ThemeAboutContent) {
		t.Fatal("about_content must be a valid theme")
	}
	if !IsReservedTemplateID(ThemeAboutContent) {
		t.Fatal("about_content id is reserved")
	}
	if !IsArticleTheme(ThemeAboutContent) || !IsArticleTheme(ThemeTextContent) {
		t.Fatal("about and text are article themes")
	}
	if IsArticleTheme(ThemePanoramaGallery) {
		t.Fatal("panorama is not an article theme")
	}
	allowed := DefaultAllowedBlocks(ThemeAboutContent)
	if len(allowed) != 2 || allowed[0] != BlockGalleryImage || allowed[1] != BlockRichText {
		t.Fatalf("about allowed=%v", allowed)
	}
	text := DefaultAllowedBlocks(ThemeTextContent)
	if !containsString(text, BlockGalleryImage) || !containsString(text, BlockRichText) {
		t.Fatalf("text allowed=%v", text)
	}
}

func TestEnsureSystemTemplatesAbout(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	tmpl, err := s.GetTemplate(ThemeAboutContent)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Name != "About" || !tmpl.IsSystem || tmpl.Theme != ThemeAboutContent {
		t.Fatalf("%+v", tmpl)
	}
	if len(tmpl.AllowedBlocks) != 2 || tmpl.AllowedBlocks[0] != BlockGalleryImage {
		t.Fatalf("allowed=%v", tmpl.AllowedBlocks)
	}
	text, err := s.GetTemplate(ThemeTextContent)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(text.AllowedBlocks, BlockGalleryImage) {
		t.Fatalf("text allowed=%v", text.AllowedBlocks)
	}
}

func TestEnsureArticleImageAllowedMigratesExisting(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PatchTemplate(ThemeTextContent, map[string]any{
		"allowed_blocks": []string{BlockRichText, BlockContactForm},
	}); err != nil {
		t.Fatal(err)
	}
	blank, err := s.CreateTemplate(Template{
		Name:          "Blank page",
		Theme:         ThemeTextContent,
		AllowedBlocks: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(blank.AllowedBlocks, BlockGalleryImage) {
		t.Fatalf("create empty allowed should fill defaults, got %v", blank.AllowedBlocks)
	}
	if _, err := s.PatchTemplate(blank.ID, map[string]any{"allowed_blocks": []string{}}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	text, _ := s.GetTemplate(ThemeTextContent)
	if !containsString(text.AllowedBlocks, BlockGalleryImage) {
		t.Fatalf("migrated text allowed=%v", text.AllowedBlocks)
	}
	again, _ := s.GetTemplate(blank.ID)
	if !containsString(again.AllowedBlocks, BlockGalleryImage) || !containsString(again.AllowedBlocks, BlockRichText) {
		t.Fatalf("blank allowed=%v", again.AllowedBlocks)
	}
}

func TestEnsureAboutPageKeepsCopy(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(Page{Slug: "about", Title: "ABOUT", Theme: ThemeTextContent, Status: "published"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceBlocks(p.ID, []Block{
		{Type: BlockRichText, Data: MustJSON(map[string]any{"html": "<p>Hello from Daria</p>"})},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureAboutPage(); err != nil {
		t.Fatal(err)
	}
	out, err := s.GetPageBySlug("about")
	if err != nil {
		t.Fatal(err)
	}
	if out.Theme != ThemeAboutContent {
		t.Fatalf("theme=%s", out.Theme)
	}
	if len(out.Blocks) != 1 || out.Blocks[0].Type != BlockRichText {
		t.Fatalf("blocks wiped: %+v", out.Blocks)
	}
	if !strings.Contains(string(out.Blocks[0].Data), "Hello from Daria") {
		t.Fatalf("copy lost: %s", out.Blocks[0].Data)
	}
}

func TestEnsureAboutPageSeedsEmpty(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePage(Page{Slug: "about", Title: "ABOUT", Theme: ThemePanoramaGallery, Status: "draft"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureAboutPage(); err != nil {
		t.Fatal(err)
	}
	out, err := s.GetPageBySlug("about")
	if err != nil {
		t.Fatal(err)
	}
	if out.Theme != ThemeAboutContent {
		t.Fatalf("theme=%s", out.Theme)
	}
	if len(out.Blocks) != 2 || out.Blocks[0].Type != BlockGalleryImage || out.Blocks[1].Type != BlockRichText {
		t.Fatalf("want portrait+bio starters, got %+v", out.Blocks)
	}
}
