package cms

import (
	"encoding/json"
	"strings"
)

const (
	ThemeBAContent       = "ba_content"
	ThemePanoramaGallery = "panorama_gallery"
	ThemeTextContent     = "text_content"
	ThemeLookbookGallery = "lookbook_gallery"
	ThemeRatesContent    = "rates_content"

	BlockComparisonSlider = "comparison_slider"
	BlockGalleryImage     = "gallery_image"
	BlockRichText         = "rich_text"
	BlockContactForm      = "contact_form"
	BlockRateBanner       = "rate_banner"

	// Form-template canvas blocks (kind=form only; not page engines).
	BlockFormStep     = "form_step"
	BlockFormText     = "form_text"
	BlockFormNumber   = "form_number"
	BlockFormDate     = "form_date"
	BlockFormTextarea = "form_textarea"
	BlockFormSelect   = "form_select"
	BlockFormRadio    = "form_radio"
	BlockFormCheckbox = "form_checkbox"
	BlockFormRetouch  = "form_retouch_level"
	BlockFormHelp     = "form_help"
	BlockFormFooter   = "form_contact_footer"
	BlockFormHoneypot = "form_honeypot"
)

const (
	RateKeyFashion   = "fashion"
	RateKeyBeauty    = "beauty"
	RateKeyLookbook  = "lookbook"
	RateKeyEditorial = "editorial"
	RateKeyProduct   = "product"
	RateKeyManual    = "manual"
)

// RateFormKeys is the locked banner / overlay order.
var RateFormKeys = []string{
	RateKeyFashion, RateKeyBeauty, RateKeyLookbook,
	RateKeyEditorial, RateKeyProduct, RateKeyManual,
}

const (
	TemplateKindPage = "page"
	TemplateKindForm = "form"
)

const (
	FormTemplateFashion   = "form_fashion"
	FormTemplateBeauty    = "form_beauty"
	FormTemplateLookbook  = "form_lookbook"
	FormTemplateEditorial = "form_editorial"
	FormTemplateProduct   = "form_product"
	FormTemplateManual    = "form_manual"
)

// FormTemplateIDs are named CMS form templates (not page engines).
var FormTemplateIDs = []string{
	FormTemplateFashion, FormTemplateBeauty, FormTemplateLookbook,
	FormTemplateEditorial, FormTemplateProduct, FormTemplateManual,
}

func FormTemplateID(key string) string {
	return "form_" + strings.ToLower(strings.TrimSpace(key))
}

func FormKeyFromTemplateID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if strings.HasPrefix(id, "form_") {
		return strings.TrimPrefix(id, "form_")
	}
	return ""
}

type SiteSettings struct {
	SiteName           string `json:"site_name"`
	LogoHTML           string `json:"logo_html"`
	Description        string `json:"description"`
	OGImage            string `json:"og_image"`
	InstagramURL       string `json:"instagram_url"`
	BehanceURL         string `json:"behance_url"`
	LinkedInURL        string `json:"linkedin_url"`
	Copyright          string `json:"copyright"`
	CanonicalBase      string `json:"canonical_base"`
	DefaultTitleSuffix string `json:"default_title_suffix,omitempty"`
	DefaultDescription string `json:"default_description,omitempty"`
	Robots             string `json:"robots,omitempty"`
	FaviconMediaID     string `json:"favicon_media_id,omitempty"`
	OGImageMediaID     string `json:"og_image_media_id,omitempty"`
	MailtoAddress      string `json:"mailto_address,omitempty"`
	ContactEmail       string `json:"contact_email,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
	// Social is admin-friendly nested shape; filled on marshal via SettingsDTO.
	Social map[string]string `json:"social,omitempty"`
}

type Page struct {
	ID              string          `json:"id"`
	Slug            string          `json:"slug"`
	Title           string          `json:"title"`
	Theme           string          `json:"theme"`
	Template        string          `json:"template,omitempty"` // alias of theme for admin
	Status          string          `json:"status"`
	SortOrder       int             `json:"sort_order"`
	NavLabel        string          `json:"nav_label,omitempty"`
	MetaTitle       string          `json:"meta_title"`
	MetaDescription string          `json:"meta_description"`
	CanonicalPath   string          `json:"canonical_path"`
	OGImage         string          `json:"og_image"`
	IsHomepage      bool            `json:"is_homepage"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	Blocks          []Block         `json:"blocks,omitempty"`
	SEO             *PageSEO        `json:"seo,omitempty"`
	Settings        json.RawMessage `json:"settings"`
}

// PageSEO is the admin SEO inspector shape (nested under page.seo).
// Empty MetaTitle means the public <title> falls back to page title; it is not filled in.
type PageSEO struct {
	MetaTitle       string `json:"meta_title"`
	MetaDescription string `json:"meta_description"`
	CanonicalPath   string `json:"canonical_path"`
	OGImageMediaID  string `json:"og_image_media_id,omitempty"`
}

// FlattenSEOPatch copies nested patch["seo"] into flat page columns without touching title.
func FlattenSEOPatch(patch map[string]any) {
	if patch == nil {
		return
	}
	seo, ok := patch["seo"].(map[string]any)
	if !ok || seo == nil {
		return
	}
	if v, ok := seo["meta_title"].(string); ok {
		patch["meta_title"] = v
	}
	if v, ok := seo["meta_description"].(string); ok {
		patch["meta_description"] = v
	}
	if v, ok := seo["description"].(string); ok {
		if _, exists := patch["meta_description"]; !exists {
			patch["meta_description"] = v
		}
	}
	if v, ok := seo["canonical_path"].(string); ok {
		patch["canonical_path"] = v
	}
	if v, ok := seo["og_image"].(string); ok {
		patch["og_image"] = v
	}
	if v, ok := seo["og_image_media_id"].(string); ok {
		patch["og_image"] = v
	}
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
	// Nested seo from the client (create body) fills stored columns when those are empty.
	if p.SEO != nil {
		if p.MetaTitle == "" && p.SEO.MetaTitle != "" {
			p.MetaTitle = p.SEO.MetaTitle
		}
		if p.MetaDescription == "" && p.SEO.MetaDescription != "" {
			p.MetaDescription = p.SEO.MetaDescription
		}
		if p.CanonicalPath == "" && p.SEO.CanonicalPath != "" {
			p.CanonicalPath = p.SEO.CanonicalPath
		}
		if p.OGImage == "" && p.SEO.OGImageMediaID != "" {
			p.OGImage = p.SEO.OGImageMediaID
		}
	}
	// Always expose nested seo from persisted columns. Never copy title into meta_title.
	p.SEO = &PageSEO{
		MetaTitle:       p.MetaTitle,
		MetaDescription: p.MetaDescription,
		CanonicalPath:   p.CanonicalPath,
		OGImageMediaID:  p.OGImage,
	}
	if len(p.Settings) == 0 || string(p.Settings) == "null" {
		p.Settings = json.RawMessage(`{}`)
	}
}

// IsPublished is true only for status=published (live GitHub Pages output).
func (p Page) IsPublished() bool {
	return strings.EqualFold(strings.TrimSpace(p.Status), "published")
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

const (
	NavKindLink     = "link"
	NavKindCategory = "category"
)

type NavItem struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Href      string    `json:"href,omitempty"`
	PageID    string    `json:"page_id,omitempty"`
	ParentID  string    `json:"parent_id,omitempty"`
	SortOrder int       `json:"sort_order"`
	Kind      string    `json:"kind"` // link | category
	Visible   bool      `json:"visible"`
	Children  []NavItem `json:"children"`
}

type PublishHistory struct {
	ID        string          `json:"id"`
	CreatedAt string          `json:"created_at"`
	Note      string          `json:"note,omitempty"`
	Status    string          `json:"status"`
	Detail    json.RawMessage `json:"detail,omitempty"`
}

// Template is a reusable blueprint. kind=page uses a generate engine
// (ba_content | panorama_gallery | text_content | lookbook_gallery | rates_content).
// kind=form is a named Rate overlay (Fashion, Beauty, …) — not a page engine.
// System page rows use id == theme; system form rows use id == form_<key>.
type Template struct {
	ID            string          `json:"id"`
	Theme         string          `json:"theme"`
	Key           string          `json:"key,omitempty"` // alias of theme
	Kind          string          `json:"kind"`
	FormKey       string          `json:"form_key,omitempty"`
	Name          string          `json:"name"`
	Label         string          `json:"label,omitempty"` // alias of name
	Description   string          `json:"description"`
	AllowedBlocks []string        `json:"allowed_blocks"`
	DefaultBlocks json.RawMessage `json:"default_blocks"`
	Source        string          `json:"source"`
	FileSource    string          `json:"file_source,omitempty"`
	IsSystem      bool            `json:"is_system"`
	SortOrder     int             `json:"sort_order"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

func (t *Template) NormalizeAliases() {
	if t.Theme == "" && t.Key != "" {
		t.Theme = t.Key
	}
	if t.Key == "" && t.Theme != "" {
		t.Key = t.Theme
	}
	if t.Name == "" && t.Label != "" {
		t.Name = t.Label
	}
	if t.Label == "" && t.Name != "" {
		t.Label = t.Name
	}
	if t.Kind == "" {
		if ValidRateFormKey(FormKeyFromTemplateID(t.ID)) && t.ID == FormTemplateID(FormKeyFromTemplateID(t.ID)) {
			t.Kind = TemplateKindForm
		} else {
			t.Kind = TemplateKindPage
		}
	}
	if t.Kind == TemplateKindForm && t.FormKey == "" {
		t.FormKey = FormKeyFromTemplateID(t.ID)
	}
	if t.AllowedBlocks == nil {
		t.AllowedBlocks = []string{}
	}
	if len(t.DefaultBlocks) == 0 {
		t.DefaultBlocks = json.RawMessage(`[]`)
	}
}

func (t Template) IsForm() bool {
	return t.Kind == TemplateKindForm
}
