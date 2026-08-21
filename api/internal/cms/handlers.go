package cms

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sheyanova.art/api/internal/httpx"
)

type Handler struct {
	Store      *Store
	FrontDir   string
	MaxUpload  int64
	ImportFn   func(force bool) (any, error)
	GenerateFn func(w http.ResponseWriter, r *http.Request)
	PreviewFn  func(w http.ResponseWriter, r *http.Request)
	PublishFn  func(w http.ResponseWriter, r *http.Request)
}

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st, err := h.Store.GetSettings()
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": settingsForAdmin(st)})
	case http.MethodPut:
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad body")
			return
		}
		st, err := parseSettingsPayload(raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad json")
			return
		}
		out, err := h.Store.PutSettings(st)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": settingsForAdmin(out)})
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func settingsForAdmin(st SiteSettings) map[string]any {
	desc := st.Description
	if st.DefaultDescription != "" {
		desc = st.DefaultDescription
	}
	ogID := st.OGImageMediaID
	if ogID == "" {
		ogID = st.OGImage
	}
	return map[string]any{
		"site_name":             st.SiteName,
		"logo_html":             st.LogoHTML,
		"description":           st.Description,
		"default_description":   desc,
		"default_title_suffix":  st.DefaultTitleSuffix,
		"robots":                st.Robots,
		"og_image":              st.OGImage,
		"og_image_media_id":     ogID,
		"favicon_media_id":      st.FaviconMediaID,
		"instagram_url":         st.InstagramURL,
		"behance_url":           st.BehanceURL,
		"linkedin_url":          st.LinkedInURL,
		"copyright":             st.Copyright,
		"canonical_base":        st.CanonicalBase,
		"mailto_address":        st.MailtoAddress,
		"updated_at":            st.UpdatedAt,
		"social": map[string]string{
			"instagram": st.InstagramURL,
			"behance":   st.BehanceURL,
			"linkedin":  st.LinkedInURL,
		},
	}
}

func parseSettingsPayload(raw []byte) (SiteSettings, error) {
	var st SiteSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return SiteSettings{}, err
	}
	// Nested social from admin UI.
	var extra struct {
		Social map[string]string `json:"social"`
	}
	_ = json.Unmarshal(raw, &extra)
	if extra.Social != nil {
		if v := extra.Social["instagram"]; v != "" {
			st.InstagramURL = v
		}
		if v := extra.Social["behance"]; v != "" {
			st.BehanceURL = v
		}
		if v := extra.Social["linkedin"]; v != "" {
			st.LinkedInURL = v
		}
	}
	if st.DefaultDescription != "" {
		st.Description = st.DefaultDescription
	}
	if st.OGImageMediaID != "" {
		st.OGImage = st.OGImageMediaID
	}
	return st, nil
}

func (h *Handler) Nav(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tree, err := h.Store.GetNavTree()
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "nav": tree})
	case http.MethodPut:
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad body")
			return
		}
		items, err := parseNavPayload(raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad json")
			return
		}
		tree, err := h.Store.ReplaceNav(items)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "nav": tree})
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func parseNavPayload(raw []byte) ([]NavItem, error) {
	type navIn struct {
		ID        string  `json:"id"`
		Label     string  `json:"label"`
		Href      string  `json:"href"`
		PageID    string  `json:"page_id"`
		ParentID  string  `json:"parent_id"`
		SortOrder int     `json:"sort_order"`
		Kind      string  `json:"kind"`
		Visible   *bool   `json:"visible"`
		Children  []navIn `json:"children"`
	}
	var convert func([]navIn) []NavItem
	convert = func(in []navIn) []NavItem {
		out := make([]NavItem, 0, len(in))
		for _, n := range in {
			vis := true
			if n.Visible != nil {
				vis = *n.Visible
			}
			out = append(out, NavItem{
				ID: n.ID, Label: n.Label, Href: n.Href, PageID: n.PageID, ParentID: n.ParentID,
				SortOrder: n.SortOrder, Kind: n.Kind, Visible: vis, Children: convert(n.Children),
			})
		}
		return out
	}
	var wrapped struct {
		Nav []navIn `json:"nav"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Nav != nil {
		return convert(wrapped.Nav), nil
	}
	var arr []navIn
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, err
	}
	return convert(arr), nil
}

func (h *Handler) Pages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pages, err := h.Store.ListPages()
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i := range pages {
			pages[i].NormalizeAliases()
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "pages": pages})
	case http.MethodPost:
		var body Page
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad json")
			return
		}
		body.NormalizeAliases()
		if body.Title == "" {
			httpx.WriteError(w, http.StatusBadRequest, "title required")
			return
		}
		if body.Slug == "" && !body.IsHomepage {
			httpx.WriteError(w, http.StatusBadRequest, "slug required")
			return
		}
		p, err := h.Store.CreatePage(body)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		p.NormalizeAliases()
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "page": p})
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) PageByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/pages/")
	path = strings.Trim(path, "/")
	if path == "" {
		h.Pages(w, r)
		return
	}

	// /api/admin/pages/{id}/blocks
	if strings.HasSuffix(path, "/blocks") || strings.Contains(path, "/blocks") {
		parts := strings.Split(path, "/")
		if len(parts) == 2 && parts[1] == "blocks" {
			h.PageBlocks(w, r, parts[0])
			return
		}
	}

	id := path
	switch r.Method {
	case http.MethodGet:
		p, err := h.Store.GetPage(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		p.NormalizeAliases()
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "page": p})
	case http.MethodPatch:
		var patch map[string]any
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad json")
			return
		}
		// Admin sends template; store uses theme.
		if _, ok := patch["theme"]; !ok {
			if t, ok := patch["template"].(string); ok {
				patch["theme"] = t
			}
		}
		if seo, ok := patch["seo"].(map[string]any); ok {
			if v, ok := seo["meta_description"].(string); ok {
				patch["meta_description"] = v
			}
			if v, ok := seo["description"].(string); ok && patch["meta_description"] == nil {
				patch["meta_description"] = v
			}
			// Meta title overrides the page <title> when provided (same column for now).
			if v, ok := seo["meta_title"].(string); ok && v != "" {
				if _, hasTitle := patch["title"]; !hasTitle {
					patch["title"] = v
				}
			}
			if v, ok := seo["og_image"].(string); ok {
				patch["og_image"] = v
			}
			if v, ok := seo["og_image_media_id"].(string); ok {
				patch["og_image"] = v
			}
		}
		p, err := h.Store.PatchPage(id, patch)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		p.NormalizeAliases()
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "page": p})
	case http.MethodDelete:
		if err := h.Store.DeletePage(id); err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) PageBlocks(w http.ResponseWriter, r *http.Request, pageID string) {
	switch r.Method {
	case http.MethodGet:
		blocks, err := h.Store.ListBlocks(pageID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i := range blocks {
			blocks[i].NormalizeAliases()
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "blocks": blocks})
	case http.MethodPut:
		var body struct {
			Blocks []Block `json:"blocks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad json")
			return
		}
		if body.Blocks == nil {
			body.Blocks = []Block{}
		}
		for i := range body.Blocks {
			body.Blocks[i].NormalizeAliases()
		}
		blocks, err := h.Store.ReplaceBlocks(pageID, body.Blocks)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		for i := range blocks {
			blocks[i].NormalizeAliases()
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "blocks": blocks})
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) Media(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		kind := r.URL.Query().Get("kind")
		q := r.URL.Query().Get("q")
		list, err := h.Store.ListMediaFiltered(kind, q)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "media": list})
	case http.MethodPost:
		h.uploadMedia(w, r)
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) MediaByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/media/")
	id = strings.Trim(id, "/")
	if id == "" {
		h.Media(w, r)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Title *string `json:"title"`
			Alt   *string `json:"alt"`
			Kind  *string `json:"kind"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad json")
			return
		}
		m, err := h.Store.PatchMedia(id, body.Title, body.Alt, body.Kind)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "media": m})
	case http.MethodDelete:
		m, err := h.Store.DeleteMedia(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = os.Remove(filepath.Join(h.Store.UploadDir(), m.Filename))
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) uploadMedia(w http.ResponseWriter, r *http.Request) {
	max := h.MaxUpload
	if max <= 0 {
		max = 25 << 20
	}
	if err := r.ParseMultipartForm(max); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad multipart")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "file required")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg":
	default:
		httpx.WriteError(w, http.StatusBadRequest, "unsupported type")
		return
	}

	id := NewID()
	name := id + ext
	dstPath := filepath.Join(h.Store.UploadDir(), name)
	dst, err := os.Create(dstPath)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dst.Close()
	n, err := io.Copy(dst, file)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	m := Media{
		ID:           id,
		Filename:     name,
		OriginalName: hdr.Filename,
		URL:          "/media/" + name,
		Title:        r.FormValue("title"),
		Alt:          r.FormValue("alt"),
		Kind:         r.FormValue("kind"),
		Mime:         hdr.Header.Get("Content-Type"),
		SizeBytes:    n,
	}
	out, err := h.Store.CreateMedia(m)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "media": out})
}

func (h *Handler) PublishHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	list, err := h.Store.ListPublishHistory(50)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "history": list})
}

func (h *Handler) Seed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := h.Store.BootSeed(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pages, _ := h.Store.ListPages()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "pages": len(pages)})
}

func (h *Handler) ImportFront(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	force := r.URL.Query().Get("force") == "1" || r.URL.Query().Get("force") == "true"
	if h.ImportFn == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "import-front not configured")
		return
	}
	out, err := h.ImportFn(force)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "import": out})
}
