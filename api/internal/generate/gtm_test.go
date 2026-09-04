package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func TestGTMPublishInjectsPreviewOmits(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{
		SiteName:       "Test",
		CanonicalBase:  "https://www.sheyanova.art",
		GTMEnabled:     true,
		GTMContainerID: cms.DefaultGTMContainerID,
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug: "contact", Title: "Contact", Theme: cms.ThemeTextContent, Status: "published",
	})
	if err != nil {
		t.Fatal(err)
	}

	publishDir := t.TempDir()
	g, err := New(s, Config{OutDir: publishDir, UploadDir: t.TempDir(), PreviewBase: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(publishDir, "contact", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	for _, need := range []string{
		"Google Tag Manager",
		"googletagmanager.com/gtm.js",
		"'" + cms.DefaultGTMContainerID + "'",
		"googletagmanager.com/ns.html?id=" + cms.DefaultGTMContainerID,
		`property="og:title"`,
		`rel="icon"`,
	} {
		if !strings.Contains(html, need) {
			t.Fatalf("publish html missing %q", need)
		}
	}

	previewDir := t.TempDir()
	g.Cfg.OutDir = previewDir
	g.Cfg.PathPrefix = "/preview"
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	prevb, err := os.ReadFile(filepath.Join(previewDir, "contact", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(prevb), "googletagmanager.com/gtm.js") {
		t.Fatal("preview must omit GTM")
	}
}

func TestGTMDisabledOmitsOnPublish(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{
		SiteName:       "Test",
		CanonicalBase:  "https://www.sheyanova.art",
		GTMEnabled:     false,
		GTMContainerID: cms.DefaultGTMContainerID,
	}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug: "about", Title: "About", Theme: cms.ThemeAboutContent, Status: "published",
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
	htmlb, err := os.ReadFile(filepath.Join(out, "about", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(htmlb), "googletagmanager.com") {
		t.Fatal("disabled GTM must not appear")
	}
}
