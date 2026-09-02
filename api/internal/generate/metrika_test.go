package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func TestYandexMetrikaPublishInjectsPreviewOmits(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{
		SiteName:             "Test",
		CanonicalBase:        "https://www.sheyanova.art",
		YandexMetrikaEnabled: true,
		YandexMetrikaID:      cms.DefaultYandexMetrikaID,
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
		"Yandex.Metrika counter",
		"mc.yandex.ru/metrika/tag.js",
		"ym(95095785, 'init'",
		"mc.yandex.ru/watch/95095785",
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
	if strings.Contains(string(prevb), "mc.yandex.ru") {
		t.Fatal("preview must not include Yandex.Metrika")
	}
}

func TestYandexMetrikaDisabledOmitsOnPublish(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{
		SiteName:             "Test",
		CanonicalBase:        "https://www.sheyanova.art",
		YandexMetrikaEnabled: false,
		YandexMetrikaID:      cms.DefaultYandexMetrikaID,
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
	if strings.Contains(string(htmlb), "mc.yandex.ru") {
		t.Fatal("disabled metrika must not appear on publish")
	}
}

func TestYandexMetrikaSnippetExact(t *testing.T) {
	s := string(YandexMetrikaSnippet("95095785"))
	if !strings.Contains(s, "ym(95095785, 'init', {clickmap:true, referrer: document.referrer, url: location.href, accurateTrackBounce:true, trackLinks:true})") {
		t.Fatalf("unexpected init line: %s", s)
	}
}
