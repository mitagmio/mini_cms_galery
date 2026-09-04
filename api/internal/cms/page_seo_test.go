package cms

import "testing"

func TestFlattenSEOPatchDoesNotOverwriteTitle(t *testing.T) {
	patch := map[string]any{
		"title": "EDITORIAL I",
		"seo": map[string]any{
			"meta_title":       "Custom SEO",
			"meta_description": "Desc",
			"keywords":         "beauty retoucher, Daria Sheyanova",
			"canonical_path":   "/editorial-i",
		},
	}
	FlattenSEOPatch(patch)
	if patch["title"] != "EDITORIAL I" {
		t.Fatalf("title=%v", patch["title"])
	}
	if patch["meta_title"] != "Custom SEO" {
		t.Fatalf("meta_title=%v", patch["meta_title"])
	}
	if patch["meta_description"] != "Desc" {
		t.Fatalf("meta_description=%v", patch["meta_description"])
	}
	if patch["meta_keywords"] != "beauty retoucher, Daria Sheyanova" {
		t.Fatalf("meta_keywords=%v", patch["meta_keywords"])
	}
	if patch["canonical_path"] != "/editorial-i" {
		t.Fatalf("canonical_path=%v", patch["canonical_path"])
	}

	clear := map[string]any{
		"title": "EDITORIAL I",
		"seo":   map[string]any{"meta_title": ""},
	}
	FlattenSEOPatch(clear)
	if clear["title"] != "EDITORIAL I" {
		t.Fatalf("cleared title=%v", clear["title"])
	}
	if clear["meta_title"] != "" {
		t.Fatalf("empty meta_title should persist as empty, got %v", clear["meta_title"])
	}
}

func TestNormalizeAliasesDoesNotCopyTitleIntoSEO(t *testing.T) {
	p := Page{Title: "EDITORIAL I"}
	p.NormalizeAliases()
	if p.SEO == nil {
		t.Fatal("seo missing")
	}
	if p.SEO.MetaTitle != "" {
		t.Fatalf("meta_title should stay empty, got %q", p.SEO.MetaTitle)
	}
	if p.Title != "EDITORIAL I" {
		t.Fatalf("title=%q", p.Title)
	}

	p.MetaTitle = "Custom SEO"
	p.NormalizeAliases()
	if p.SEO.MetaTitle != "Custom SEO" {
		t.Fatalf("seo.meta_title=%q", p.SEO.MetaTitle)
	}
}

func TestPatchPageSEOIndependentOfTitle(t *testing.T) {
	s := testStore(t)
	p := mustPage(t, s, "editorial-i", "EDITORIAL I", false)

	out, err := s.PatchPage(p.ID, map[string]any{
		"title": "EDITORIAL I",
		"slug":  "editorial-i",
		"seo": map[string]any{
			"meta_title":       "Custom SEO title",
			"meta_description": "Custom desc",
			"canonical_path":   "/editorial-i",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "EDITORIAL I" {
		t.Fatalf("title=%q", out.Title)
	}
	if out.MetaTitle != "Custom SEO title" {
		t.Fatalf("meta_title=%q", out.MetaTitle)
	}
	if out.SEO == nil || out.SEO.MetaTitle != "Custom SEO title" {
		t.Fatalf("seo=%+v", out.SEO)
	}
	if out.CanonicalPath != "/editorial-i" {
		t.Fatalf("canonical_path=%q", out.CanonicalPath)
	}

	out, err = s.PatchPage(p.ID, map[string]any{
		"title": "EDITORIAL-I",
		"seo": map[string]any{
			"meta_title":       "Custom SEO title",
			"meta_description": "Custom desc",
			"canonical_path":   "/editorial-i",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "EDITORIAL-I" {
		t.Fatalf("renamed title=%q", out.Title)
	}
	if out.MetaTitle != "Custom SEO title" {
		t.Fatalf("meta_title overwritten: %q", out.MetaTitle)
	}

	out, err = s.PatchPage(p.ID, map[string]any{
		"title": "EDITORIAL I",
		"seo":   map[string]any{"meta_title": ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.MetaTitle != "" {
		t.Fatalf("expected empty meta_title, got %q", out.MetaTitle)
	}
	if out.SEO != nil && out.SEO.MetaTitle != "" {
		t.Fatalf("seo.meta_title should stay empty, got %q", out.SEO.MetaTitle)
	}

	got, err := s.GetPage(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MetaTitle != "" {
		t.Fatalf("reload meta_title=%q", got.MetaTitle)
	}
	if got.Title != "EDITORIAL I" {
		t.Fatalf("reload title=%q", got.Title)
	}
}
