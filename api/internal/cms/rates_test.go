package cms

import (
	"strings"
	"testing"
)

func TestValidThemeRates(t *testing.T) {
	if !ValidTheme(ThemeRatesContent) {
		t.Fatal("rates_content must be a valid theme")
	}
	if !IsReservedTemplateID(ThemeRatesContent) {
		t.Fatal("rates_content id is reserved")
	}
	if !ValidBlockType(BlockRateBanner) {
		t.Fatal("rate_banner must be a valid block")
	}
	allowed := DefaultAllowedBlocks(ThemeRatesContent)
	if len(allowed) != 2 || allowed[0] != BlockRichText || allowed[1] != BlockRateBanner {
		t.Fatalf("allowed=%v", allowed)
	}
}

func TestEnsureSystemTemplatesRates(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	tmpl, err := s.GetTemplate(ThemeRatesContent)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Theme != ThemeRatesContent || !tmpl.IsSystem {
		t.Fatalf("%+v", tmpl)
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRatesPageAndNav(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	about := mustPage(t, s, "about", "ABOUT", false)
	contact := mustPage(t, s, "contact", "CONTACT", false)
	if _, err := s.ReplaceNav([]NavItem{
		{Label: "ABOUT", Kind: NavKindLink, PageID: about.ID, Visible: true},
		{Label: "CONTACT", Kind: NavKindLink, PageID: contact.ID, Visible: true},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.EnsureRatesPageAndNav(); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureRatesPageAndNav(); err != nil {
		t.Fatal(err)
	}

	page, err := s.GetPageBySlug("rates")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "RATES" || page.Theme != ThemeRatesContent || page.Status != "draft" {
		t.Fatalf("%+v", page)
	}
	if page.IsPublished() {
		t.Fatal("rates must stay draft")
	}
	intro := ""
	for _, b := range page.Blocks {
		if b.Type == BlockRichText {
			m := PageSettingsMap(b.Data)
			intro, _ = m["html"].(string)
			break
		}
	}
	if !strings.Contains(intro, "rates-intro-copy") || !strings.Contains(intro, "rates-kicker") || !strings.Contains(intro, "CHOOSE YOUR CATEGORY") {
		t.Fatalf("intro=%s", intro)
	}

	var banners []string
	var tmplIDs []string
	for _, b := range page.Blocks {
		if b.Type != BlockRateBanner {
			continue
		}
		m := PageSettingsMap(b.Data)
		key := RateFormKeyFromData(m)
		id, _ := m["form_template_id"].(string)
		banners = append(banners, key)
		tmplIDs = append(tmplIDs, id)
	}
	if len(banners) != 6 {
		t.Fatalf("banners=%v", banners)
	}
	for i, want := range RateFormKeys {
		if banners[i] != want {
			t.Fatalf("banner %d=%s want %s", i, banners[i], want)
		}
		if tmplIDs[i] != FormTemplateID(want) {
			t.Fatalf("banner %d template=%s want %s", i, tmplIDs[i], FormTemplateID(want))
		}
	}
	if BannerAspectFromSettings(page.Settings) != DefaultBannerAspect {
		t.Fatalf("banner_aspect=%s", BannerAspectFromSettings(page.Settings))
	}

	tree, err := s.GetNavTree()
	if err != nil {
		t.Fatal(err)
	}
	labels := make([]string, 0, len(tree))
	var ratesCount int
	for _, it := range tree {
		labels = append(labels, it.Label)
		if isRatesNavItem(it) {
			ratesCount++
			if !it.Visible || it.PageID != page.ID {
				t.Fatalf("rates nav=%+v", it)
			}
		}
	}
	if ratesCount != 1 {
		t.Fatalf("rates nav count=%d labels=%v", ratesCount, labels)
	}
	aboutIdx := indexNavLabelOrHref(tree, "ABOUT", "about")
	ratesIdx := indexNavLabelOrHref(tree, "RATES", "rates")
	if ratesIdx < 0 || aboutIdx != ratesIdx+1 {
		t.Fatalf("nav order=%v", labels)
	}
}

func TestRatesIntroHTMLHasNoInlineFont(t *testing.T) {
	html := RatesIntroHTML()
	if strings.Contains(strings.ToLower(html), "font-family") || strings.Contains(strings.ToLower(html), "<font") {
		t.Fatalf("intro must not set a face: %s", html)
	}
	if strings.Contains(html, "xl-headline") {
		t.Fatal("intro must not use Contact xl-headline")
	}
	if !strings.Contains(html, `class="rates-kicker"`) {
		t.Fatalf("intro=%s", html)
	}
}

func TestEnsureRatesIntroCopyStripsInlineFonts(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	about := mustPage(t, s, "about", "ABOUT", false)
	if _, err := s.ReplaceNav([]NavItem{
		{Label: "ABOUT", Kind: NavKindLink, PageID: about.ID, Visible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureRatesPageAndNav(); err != nil {
		t.Fatal(err)
	}
	page, err := s.GetPageBySlug("rates")
	if err != nil {
		t.Fatal(err)
	}
	blocks := append([]Block(nil), page.Blocks...)
	found := false
	for i, b := range blocks {
		if b.Type != BlockRichText {
			continue
		}
		blocks[i].Data = MustJSON(map[string]any{
			"html": `<div class="rates-intro-copy"><h2 class="xl-headline"><span style="font-family:Georgia, serif">CHOOSE YOUR CATEGORY</span></h2><p style="font-family:Arial,sans-serif">Click on a category to submit your retouching request.</p><p>The price is per photo.</p></div>`,
		})
		found = true
		break
	}
	if !found {
		t.Fatal("missing rich_text")
	}
	if _, err := s.ReplaceBlocks(page.ID, blocks); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureRatesPageAndNav(); err != nil {
		t.Fatal(err)
	}
	page, err = s.GetPageBySlug("rates")
	if err != nil {
		t.Fatal(err)
	}
	intro := ""
	for _, b := range page.Blocks {
		if b.Type == BlockRichText {
			m := PageSettingsMap(b.Data)
			intro, _ = m["html"].(string)
			break
		}
	}
	if intro != RatesIntroHTML() {
		t.Fatalf("intro=%s", intro)
	}
	if strings.Contains(strings.ToLower(intro), "georgia") || strings.Contains(strings.ToLower(intro), "arial") || strings.Contains(strings.ToLower(intro), "font-family") {
		t.Fatalf("inline fonts remain: %s", intro)
	}
}
