package cms

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// BootSeed fills empty DB with site settings, pages matching current front slugs, and nav.
// Optionally imports photos from legacy meta.json into media.
func (s *Store) BootSeed() error {
	nPages, err := s.CountPages()
	if err != nil {
		return err
	}
	if nPages == 0 {
		if err := s.seedDefaults(); err != nil {
			return err
		}
		log.Printf("cms: seeded default pages/nav/settings")
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		return err
	}
	if err := s.EnsureAboutPage(); err != nil {
		log.Printf("cms: ensure about page: %v", err)
		return err
	}
	if err := s.EnsureRatesPageAndNav(); err != nil {
		log.Printf("cms: ensure rates page/nav: %v", err)
		return err
	}
	if err := s.BackfillContactEmail(); err != nil {
		log.Printf("cms: contact_email backfill: %v", err)
	}
	nMedia, err := s.CountMedia()
	if err != nil {
		return err
	}
	if nMedia == 0 {
		imported, err := s.importLegacyMeta()
		if err != nil {
			log.Printf("cms: legacy meta import skipped: %v", err)
		} else if imported > 0 {
			log.Printf("cms: imported %d media from meta.json", imported)
		}
	}
	return nil
}

func (s *Store) seedDefaults() error {
	_, err := s.PutSettings(SiteSettings{
		SiteName:      "Daria Sheyanova's Portfolio",
		LogoHTML:      " DARYIA <br/>SHEYANOVA",
		Description:   "retouch services",
		InstagramURL:  "https://www.instagram.com/d._retoucher/",
		BehanceURL:    "https://www.behance.net/dariasheianova",
		LinkedInURL:   "https://www.linkedin.com/in/daria-sheyanova-5a6b212a5/",
		Copyright:     "Daria Sheyanova © All Rights Reserved",
		CanonicalBase: "https://sheyanova.art",
	})
	if err != nil {
		return err
	}

	type seedPage struct {
		Slug      string
		Title     string
		Theme     string
		Homepage  bool
		SortOrder int
		Blocks    []Block
	}
	pages := []seedPage{
		{Slug: "before-after", Title: "BEFORE | AFTER", Theme: ThemeBAContent, Homepage: true, SortOrder: 0},
		{Slug: "editorial-i", Title: "EDITORIAL I", Theme: ThemePanoramaGallery, SortOrder: 1},
		{Slug: "editorial-ii", Title: "EDITORIAL II", Theme: ThemePanoramaGallery, SortOrder: 2},
		{Slug: "editorial-3", Title: "EDITORIAL 3", Theme: ThemePanoramaGallery, SortOrder: 3},
		{Slug: "editorial-iv", Title: "EDITORIAL IV", Theme: ThemePanoramaGallery, SortOrder: 4},
		{Slug: "fashion", Title: "FASHION", Theme: ThemePanoramaGallery, SortOrder: 5},
		{Slug: "editorial", Title: "EDITORIAL", Theme: ThemePanoramaGallery, SortOrder: 6},
		{Slug: "product", Title: "PRODUCT", Theme: ThemePanoramaGallery, SortOrder: 7},
		{Slug: "about", Title: "ABOUT", Theme: ThemeAboutContent, SortOrder: 8, Blocks: defaultAboutBlockSlice()},
		{Slug: "contact", Title: "CONTACT", Theme: ThemeTextContent, SortOrder: 9, Blocks: []Block{
			{Type: BlockRichText, Data: MustJSON(map[string]any{"html": `<h2 class="xl-headline">Get in Touch</h2>`})},
			{Type: BlockContactForm, Data: MustJSON(map[string]any{
				"name_label": "Name", "email_label": "Email", "message_label": "Message", "submit_label": "Send Message",
			})},
		}},
	}

	pageIDs := map[string]string{}
	for _, sp := range pages {
		p, err := s.CreatePage(Page{
			Slug:            sp.Slug,
			Title:           sp.Title,
			Theme:           sp.Theme,
			Status:          "published",
			SortOrder:       sp.SortOrder,
			IsHomepage:      sp.Homepage,
			MetaDescription: "retouch services",
		})
		if err != nil {
			return err
		}
		pageIDs[sp.Slug] = p.ID
		if len(sp.Blocks) > 0 {
			if _, err := s.ReplaceBlocks(p.ID, sp.Blocks); err != nil {
				return err
			}
		}
	}

	nav := []NavItem{
		{
			Label: "BEAUTY", Kind: "category", Visible: true, SortOrder: 0,
			Children: []NavItem{
				{Label: "EDITORIAL I", Href: "/editorial-i", PageID: pageIDs["editorial-i"], Kind: "link", Visible: true, SortOrder: 0},
				{Label: "EDITORIAL II", Href: "/editorial-ii", PageID: pageIDs["editorial-ii"], Kind: "link", Visible: true, SortOrder: 1},
				{Label: "EDITORIAL 3", Href: "/editorial-3", PageID: pageIDs["editorial-3"], Kind: "link", Visible: true, SortOrder: 2},
				{Label: "EDITORIAL IV", Href: "/editorial-iv", PageID: pageIDs["editorial-iv"], Kind: "link", Visible: true, SortOrder: 3},
			},
		},
		{Label: "BEFORE | AFTER", Href: "/before-after", PageID: pageIDs["before-after"], Kind: "link", Visible: true, SortOrder: 1},
		{Label: "FASHION", Href: "/fashion", PageID: pageIDs["fashion"], Kind: "link", Visible: true, SortOrder: 2},
		{Label: "EDITORIAL", Href: "/editorial", PageID: pageIDs["editorial"], Kind: "link", Visible: true, SortOrder: 3},
		{Label: "PRODUCT", Href: "/product", PageID: pageIDs["product"], Kind: "link", Visible: true, SortOrder: 4},
		{Label: "ABOUT", Href: "/about", PageID: pageIDs["about"], Kind: "link", Visible: true, SortOrder: 5},
		{Label: "CONTACT", Href: "/contact", PageID: pageIDs["contact"], Kind: "link", Visible: true, SortOrder: 6},
	}
	_, err = s.ReplaceNav(nav)
	return err
}

func (s *Store) importLegacyMeta() (int, error) {
	path := filepath.Join(s.dataDir, "meta.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var meta struct {
		Photos []struct {
			ID        string `json:"id"`
			Filename  string `json:"filename"`
			URL       string `json:"url"`
			Title     string `json:"title"`
			Kind      string `json:"kind"`
			CreatedAt string `json:"created_at"`
		} `json:"photos"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return 0, err
	}
	n := 0
	for _, p := range meta.Photos {
		id := p.ID
		if id == "" {
			id = NewID()
		}
		url := p.URL
		if url == "" && p.Filename != "" {
			url = "/media/" + p.Filename
		}
		created := p.CreatedAt
		if created == "" {
			created = Now()
		}
		m := Media{
			ID: id, Filename: p.Filename, OriginalName: p.Filename, URL: url,
			Title: p.Title, Kind: p.Kind, CreatedAt: created, UpdatedAt: created,
		}
		if _, err := s.CreateMedia(m); err != nil {
			continue
		}
		n++
	}
	return n, nil
}
