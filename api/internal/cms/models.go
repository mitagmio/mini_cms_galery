package cms

import "encoding/json"

const (
	ThemeBAContent        = "ba_content"
	ThemePanoramaGallery  = "panorama_gallery"
	ThemeTextContent      = "text_content"

	BlockComparisonSlider = "comparison_slider"
	BlockGalleryImage     = "gallery_image"
	BlockRichText         = "rich_text"
	BlockContactForm      = "contact_form"
)

type SiteSettings struct {
	SiteName            string `json:"site_name"`
	LogoHTML            string `json:"logo_html"`
	Description         string `json:"description"`
	OGImage             string `json:"og_image"`
	InstagramURL        string `json:"instagram_url"`
	BehanceURL          string `json:"behance_url"`
	LinkedInURL         string `json:"linkedin_url"`
	Copyright           string `json:"copyright"`
	CanonicalBase       string `json:"canonical_base"`
	DefaultTitleSuffix  string `json:"default_title_suffix,omitempty"`
	DefaultDescription  string `json:"default_description,omitempty"`
	Robots              string `json:"robots,omitempty"`
	FaviconMediaID      string `json:"favicon_media_id,omitempty"`
	OGImageMediaID      string `json:"og_image_media_id,omitempty"`
	MailtoAddress       string `json:"mailto_address,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
	// Social is admin-friendly nested shape; filled on marshal via SettingsDTO.
	Social map[string]string `json:"social,omitempty"`
}

type Page struct {
	ID              string   `json:"id"`
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	Theme           string   `json:"theme"`
	Template        string   `json:"template,omitempty"` // alias of theme for admin
	Status          string   `json:"status"`
	SortOrder       int      `json:"sort_order"`
	NavLabel        string   `json:"nav_label,omitempty"`
	MetaDescription string   `json:"meta_description"`
	OGImage         string   `json:"og_image"`
	IsHomepage      bool     `json:"is_homepage"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Blocks          []Block  `json:"blocks,omitempty"`
	SEO             *PageSEO `json:"seo,omitempty"`
}

// PageSEO is the admin SEO inspector shape (nested under page.seo).
type PageSEO struct {
	MetaTitle       string `json:"meta_title,omitempty"`
	MetaDescription string `json:"meta_description,omitempty"`
	CanonicalPath   string `json:"canonical_path,omitempty"`
	OGImageMediaID  string `json:"og_image_media_id,omitempty"`
}

// NormalizeAliases copies theme↔template so either name works.
func (p *Page) NormalizeAliases() {
	if p.Theme == "" && p.Template != "" {
		p.Theme = p.Template
	}
	if p.Template == "" && p.Theme != "" {
		p.Template = p.Theme
	}
	if p.NavLabel == "" {
		p.NavLabel = p.Title
	}
	// Always expose nested seo for admin (values live in flat columns today).
	seo := p.SEO
	if seo == nil {
		seo = &PageSEO{}
	}
	if seo.MetaTitle == "" {
		seo.MetaTitle = p.Title
	}
	if seo.MetaDescription == "" {
		seo.MetaDescription = p.MetaDescription
	}
	if seo.OGImageMediaID == "" && p.OGImage != "" {
		seo.OGImageMediaID = p.OGImage
	}
	p.SEO = seo
}

type Block struct {
	ID        string          `json:"id"`
	PageID    string          `json:"page_id,omitempty"`
	Type      string          `json:"type"`
	SortOrder int             `json:"sort_order"`
	Position  int             `json:"position,omitempty"` // alias of sort_order for admin
	Data      json.RawMessage `json:"data"`
	CreatedAt string          `json:"created_at,omitempty"`
	UpdatedAt string          `json:"updated_at,omitempty"`
}

func (b *Block) NormalizeAliases() {
	if b.SortOrder == 0 && b.Position != 0 {
		b.SortOrder = b.Position
	}
	b.Position = b.SortOrder
}

type Media struct {
	ID           string `json:"id"`
	Filename     string `json:"filename"`
	OriginalName string `json:"original_name,omitempty"`
	URL          string `json:"url"`
	Title        string `json:"title,omitempty"`
	Alt          string `json:"alt,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Mime         string `json:"mime,omitempty"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type NavItem struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Href      string    `json:"href,omitempty"`
	PageID    string    `json:"page_id,omitempty"`
	ParentID  string    `json:"parent_id,omitempty"`
	SortOrder int       `json:"sort_order"`
	Kind      string    `json:"kind"` // link | category
	Visible   bool      `json:"visible"`
	Children  []NavItem `json:"children,omitempty"`
}

type PublishHistory struct {
	ID        string          `json:"id"`
	CreatedAt string          `json:"created_at"`
	Note      string          `json:"note,omitempty"`
	Status    string          `json:"status"`
	Detail    json.RawMessage `json:"detail,omitempty"`
}
