package cms

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir, filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(); _ = os.RemoveAll(dir) })
	return s
}

func mustPage(t *testing.T, s *Store, slug, title string, home bool) Page {
	t.Helper()
	p, err := s.CreatePage(Page{Slug: slug, Title: title, Theme: ThemeTextContent, Status: "published", IsHomepage: home})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReplaceNavFillsHrefFromPageID(t *testing.T) {
	s := testStore(t)
	about := mustPage(t, s, "about", "ABOUT", false)
	home := mustPage(t, s, "before-after", "BEFORE | AFTER", true)

	tree, err := s.ReplaceNav([]NavItem{
		{Label: "About", Kind: NavKindLink, PageID: about.ID, Visible: true},
		{Label: "Home", Kind: NavKindLink, PageID: home.ID, Visible: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 {
		t.Fatalf("len=%d", len(tree))
	}
	if tree[0].Href != "/about" {
		t.Fatalf("about href=%q", tree[0].Href)
	}
	if tree[1].Href != "/" {
		t.Fatalf("home href=%q", tree[1].Href)
	}
}

func TestReplaceNavCategoryChildren(t *testing.T) {
	s := testStore(t)
	ed := mustPage(t, s, "editorial-i", "EDITORIAL I", false)
	tree, err := s.ReplaceNav([]NavItem{
		{
			Label: "BEAUTY", Kind: NavKindCategory, Visible: true,
			Children: []NavItem{
				{Label: "EDITORIAL I", Kind: NavKindLink, PageID: ed.ID, Visible: true},
			},
		},
		{Label: "Custom", Kind: NavKindLink, Href: "https://example.com", Visible: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tree[0].Kind != NavKindCategory || tree[0].Href != "" {
		t.Fatalf("category=%+v", tree[0])
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Href != "/editorial-i" {
		t.Fatalf("children=%+v", tree[0].Children)
	}
	if tree[1].Href != "https://example.com" {
		t.Fatalf("external href=%q", tree[1].Href)
	}
}

func TestReplaceNavRejectsBadKindAndNestedCategory(t *testing.T) {
	s := testStore(t)
	_, err := s.ReplaceNav([]NavItem{{Label: "X", Kind: "folder", Visible: true}})
	if !errors.Is(err, ErrInvalidNav) {
		t.Fatalf("want ErrInvalidNav, got %v", err)
	}
	_, err = s.ReplaceNav([]NavItem{{
		Label: "BEAUTY", Kind: NavKindCategory, Visible: true,
		Children: []NavItem{{
			Label: "nested", Kind: NavKindCategory, Visible: true,
			Children: []NavItem{{Label: "leaf", Kind: NavKindLink, Href: "/x", Visible: true}},
		}},
	}})
	if !errors.Is(err, ErrInvalidNav) {
		t.Fatalf("nested: want ErrInvalidNav, got %v", err)
	}
}

func TestReplaceNavUnknownPageID(t *testing.T) {
	s := testStore(t)
	_, err := s.ReplaceNav([]NavItem{{Label: "X", Kind: NavKindLink, PageID: "missing", Visible: true}})
	if !errors.Is(err, ErrInvalidNav) {
		t.Fatalf("want ErrInvalidNav, got %v", err)
	}
}

func TestParseNavPayloadWrappedAndArray(t *testing.T) {
	a, err := parseNavPayload([]byte(`{"nav":[{"label":"A","kind":"link","visible":true}]}`))
	if err != nil || len(a) != 1 || a[0].Label != "A" || !a[0].Visible {
		t.Fatalf("wrapped: %+v %v", a, err)
	}
	b, err := parseNavPayload([]byte(`[{"label":"B","kind":"link","visible":false}]`))
	if err != nil || len(b) != 1 || b[0].Visible {
		t.Fatalf("array: %+v %v", b, err)
	}
	c, err := parseNavPayload([]byte(`{"nav":[{"label":"C"}]}`))
	if err != nil || !c[0].Visible {
		t.Fatalf("default visible: %+v %v", c, err)
	}
}
