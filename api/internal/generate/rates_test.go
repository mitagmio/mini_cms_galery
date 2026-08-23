package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sheyanova.art/api/internal/cms"
)

func TestGenerateRatesPreviewIncludesDraftPublishOmits(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	about, err := s.CreatePage(cms.Page{Slug: "about", Title: "ABOUT", Theme: cms.ThemeTextContent, Status: "published"})
	if err != nil {
		t.Fatal(err)
	}
	contact, err := s.CreatePage(cms.Page{Slug: "contact", Title: "CONTACT", Theme: cms.ThemeTextContent, Status: "published"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceNav([]cms.NavItem{
		{Label: "ABOUT", Kind: cms.NavKindLink, PageID: about.ID, Visible: true},
		{Label: "CONTACT", Kind: cms.NavKindLink, PageID: contact.ID, Visible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureRatesPageAndNav(); err != nil {
		t.Fatal(err)
	}

	previewDir := t.TempDir()
	g, err := New(s, Config{
		OutDir:       previewDir,
		UploadDir:    filepath.Join(t.TempDir(), "up"),
		PreviewBase:  "/preview",
		PathPrefix:   "/preview",
		PublicAPIURL: "https://api.sheyanova.art",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.GenerateSite(); err != nil {
		t.Fatal(err)
	}

	htmlb, err := os.ReadFile(filepath.Join(previewDir, "rates", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	for _, need := range []string{
		`body class="rates rates-page content content_page simple simple_page"`,
		`class="hide-for-small theme_menu"`,
		`class="rates-banners"`,
		`class="rate-banner`,
		`id="fashion"`,
		`id="beauty"`,
		`id="lookbook"`,
		`id="editorial"`,
		`id="product"`,
		`id="manual"`,
		`id="rates_form_fashion"`,
		`id="rates_form_beauty"`,
		`id="rates_form_lookbook"`,
		`id="rates_form_editorial"`,
		`id="rates_form_product"`,
		`id="rates_form_manual"`,
		`id="rate-banner-fashion"`,
		`name="Imagelink"`,
		`name="Retouch_level"`,
		`name="task"`,
		`data-rate-date`,
		`placeholder="YYYY-MM-DD"`,
		`rates-dialog-close`,
		`/assets/theme/rates/retouch-level-1.webp`,
		`/assets/theme/rates/retouch-level-4.webp`,
		`rate-retouch-grid`,
		`rate-retouch__frame`,
		`rate-retouch__plaque`,
		`LIGHT / RAW`,
		`FULL TOUCH UP`,
		`rates.css?v=14`,
		`--rate-banner-aspect: 3 / 4`,
		`rates-dialog`,
		`rates-kicker`,
		`rates.js?v=5`,
		`form-submit.js?v=4`,
		`action="https://api.sheyanova.art/api/contact"`,
		`>RATES</a>`,
		`>ABOUT</a>`,
		`role="dialog"`,
		`aria-modal="true"`,
	} {
		if !strings.Contains(html, need) {
			t.Fatalf("missing %q in preview html", need)
		}
	}
	for _, forbid := range []string{
		`name="models_in_frame"`,
		`name="product_category"`,
		`name="usage"`,
		`calculator`,
		`Total*100`,
		`type="date"`,
		`static.tildacdn`,
		`#ff0000`,
	} {
		if strings.Contains(html, forbid) {
			t.Fatalf("preview must not contain %q", forbid)
		}
	}
	ratesPos := strings.Index(html, ">RATES</a>")
	aboutPos := strings.Index(html, ">ABOUT</a>")
	if ratesPos < 0 || aboutPos < 0 || ratesPos > aboutPos {
		t.Fatalf("RATES must appear before ABOUT in nav")
	}
	if strings.Count(html, `class="rate-banner`) < 6 {
		t.Fatalf("want 6 banners, html banners=%d", strings.Count(html, `class="rate-banner`))
	}
	if strings.Count(html, `/assets/theme/rates/retouch-level-1.webp`) != 4 {
		t.Fatalf("want retouch animations on fashion/beauty/lookbook/editorial only")
	}
	if strings.Contains(html, `retouchclub`) || strings.Contains(html, `retouch-level-1.gif`) {
		t.Fatal("rates forms must not hotlink retouchclub or keep RC GIFs")
	}
	if strings.Count(html, `rates-dialog-close`) < 6 {
		t.Fatalf("each modal needs a close control")
	}
	fromIdx := strings.Index(html, `rate-banner__from`)
	capIdx := strings.Index(html, `rate-banner__caption`)
	if fromIdx < 0 || capIdx < 0 || fromIdx > capIdx {
		t.Fatalf("overlay order must be price/START FROM then category name at bottom")
	}

	publishDir := t.TempDir()
	g.Cfg.OutDir = publishDir
	g.Cfg.PathPrefix = ""
	g.Cfg.PublishedOnly = true
	if err := g.GenerateSite(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "rates", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("publish must omit draft /rates/, err=%v", err)
	}
	aboutHTML, err := os.ReadFile(filepath.Join(publishDir, "about", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(aboutHTML), ">RATES</a>") || strings.Contains(string(aboutHTML), "/rates/") {
		t.Fatal("publish nav must omit unpublished RATES")
	}
	if !strings.Contains(string(aboutHTML), ">ABOUT</a>") {
		t.Fatal("publish about page missing ABOUT nav")
	}
}

func TestGenerateRateBannerUsesFormTemplateID(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{
		Slug:   "rates-beauty",
		Title:  "RATES",
		Theme:  cms.ThemeRatesContent,
		Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceBlocks(p.ID, []cms.Block{{
		Type: cms.BlockRateBanner,
		Data: cms.MustJSON(map[string]any{
			"form_template_id": cms.FormTemplateBeauty,
			"caption":          "BEAUTY",
		}),
	}}); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	g, err := New(s, Config{OutDir: out, UploadDir: filepath.Join(t.TempDir(), "up"), PreviewBase: "/preview", PathPrefix: "/preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writePage(mustGet(t, s, p.ID)); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(out, "rates-beauty", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	for _, need := range []string{
		`data-rate-key="beauty"`,
		`id="rate-banner-beauty"`,
		`id="beauty"`,
		`id="rates_form_beauty"`,
		`data-rate-form="rates_beauty"`,
	} {
		if !strings.Contains(html, need) {
			t.Fatalf("missing %q", need)
		}
	}
}

func TestGenerateRatesTurnstileExplicitWidget(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureRatesPageAndNav(); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	g, err := New(s, Config{
		OutDir:           out,
		UploadDir:        filepath.Join(t.TempDir(), "up"),
		PreviewBase:      "/preview",
		PathPrefix:       "/preview",
		TurnstileSiteKey: "0x-site",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.GenerateSite(); err != nil {
		t.Fatal(err)
	}
	htmlb, err := os.ReadFile(filepath.Join(out, "rates", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlb)
	if !strings.Contains(html, `class="js-turnstile"`) || !strings.Contains(html, `data-sitekey="0x-site"`) {
		t.Fatal("rates forms must render an explicit Turnstile host for overlay widgets")
	}
	if !strings.Contains(html, `src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"></script>`) {
		t.Fatal("rates page must load Turnstile in explicit-render mode without async/defer")
	}
	if strings.Contains(html, `api.js?render=explicit" defer`) || strings.Contains(html, `api.js?render=explicit" async`) ||
		strings.Contains(html, " async defer") {
		t.Fatal("turnstile.ready() requires api.js without async/defer")
	}
	if strings.Contains(html, `class="cf-turnstile"`) {
		t.Fatal("hidden rate modals must not use implicit cf-turnstile (empty token on submit)")
	}
}
