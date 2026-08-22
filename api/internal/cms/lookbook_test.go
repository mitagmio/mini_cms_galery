package cms

import "testing"

func TestValidThemeLookbook(t *testing.T) {
	if !ValidTheme(ThemeLookbookGallery) {
		t.Fatal("lookbook_gallery must be a valid theme")
	}
	if !IsReservedTemplateID(ThemeLookbookGallery) {
		t.Fatal("lookbook_gallery id is reserved")
	}
}

func TestShuffleSeedAndMergeSettings(t *testing.T) {
	if ShuffleSeed(nil) != 0 {
		t.Fatal("empty seed")
	}
	raw := MustJSON(map[string]any{"shuffle_seed": 42})
	if ShuffleSeed(raw) != 42 {
		t.Fatalf("got %d", ShuffleSeed(raw))
	}
	merged := MergeSettings(raw, map[string]any{"shuffle_seed": 99, "extra": "x"})
	if ShuffleSeed(merged) != 99 {
		t.Fatalf("merged seed %d", ShuffleSeed(merged))
	}
	m := PageSettingsMap(merged)
	if m["extra"] != "x" {
		t.Fatalf("extra=%v", m["extra"])
	}
}

func TestEnsureSystemTemplatesLookbook(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	tmpl, err := s.GetTemplate(ThemeLookbookGallery)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Theme != ThemeLookbookGallery || !tmpl.IsSystem {
		t.Fatalf("%+v", tmpl)
	}
	if len(tmpl.AllowedBlocks) != 1 || tmpl.AllowedBlocks[0] != BlockGalleryImage {
		t.Fatalf("allowed=%v", tmpl.AllowedBlocks)
	}
	// Second call must not overwrite.
	tmpl.Name = "changed"
	if _, err := s.PatchTemplate(tmpl.ID, map[string]any{"name": "changed"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	again, _ := s.GetTemplate(ThemeLookbookGallery)
	if again.Name != "changed" {
		t.Fatalf("should keep edits, got %q", again.Name)
	}
}

func TestPageSettingsJSON(t *testing.T) {
	s := testStore(t)
	p, err := s.CreatePage(Page{
		Slug:     "look-test",
		Title:    "Look",
		Theme:    ThemeLookbookGallery,
		Status:   "draft",
		Settings: MustJSON(map[string]any{"shuffle_seed": 7}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if ShuffleSeed(p.Settings) != 7 {
		t.Fatalf("create seed %s", p.Settings)
	}
	out, err := s.PatchPage(p.ID, map[string]any{
		"settings": map[string]any{"shuffle_seed": 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ShuffleSeed(out.Settings) != 8 {
		t.Fatalf("patch seed %s", out.Settings)
	}
	if err := s.MergePageSettings(p.ID, map[string]any{"shuffle_seed": 9}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPage(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ShuffleSeed(got.Settings) != 9 {
		t.Fatalf("merge seed %s", got.Settings)
	}
	if got.Status != "draft" {
		t.Fatalf("status %s", got.Status)
	}
}
