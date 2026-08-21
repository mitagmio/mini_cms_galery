package importfront

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	reComparisonPair = regexp.MustCompile(
		`(?s)comparison_slider__slider_image--2"\s+src="([^"]+)".*?comparison_slider__slider_image--1"\s+src="([^"]+)"`,
	)
	reAssetImage = regexp.MustCompile(
		`(?is)<div[^>]*class="[^"]*\basset\b[^"]*\bimage\b[^"]*"[^>]*>.*?<img\b([^>]*)>`,
	)
	reAttrSrc     = regexp.MustCompile(`(?i)\b(?:data-src|src)="([^"]+)"`)
	reAttrAlt     = regexp.MustCompile(`(?i)\balt="([^"]*)"`)
	reDataSrcAny  = regexp.MustCompile(`(?i)\bdata-src="(/assets/cdn/[^"]+)"`)
	reImgSrcCDN   = regexp.MustCompile(`(?i)<img[^>]+\bsrc="(/assets/cdn/[^"]+)"`)
	reContactH2   = regexp.MustCompile(`(?is)<h2[^>]*class="[^"]*xl-headline[^"]*"[^>]*>\s*([^<]+?)\s*</h2>`)
	reContactLabel = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*_4ORMAT_module_contact_label[^"]*"[^>]*>\s*([^<]+?)\s*</div>`)
)

type comparisonPair struct {
	BeforeURL string // image--1 (original)
	AfterURL  string // image--2 (retouched)
}

type galleryImage struct {
	URL string
	Alt string
}

type contactContent struct {
	Headline    string
	NameLabel   string
	EmailLabel  string
	MessageLabel string
	SubmitLabel string
}

func parseComparisonPairs(html string) []comparisonPair {
	matches := reComparisonPair.FindAllStringSubmatch(html, -1)
	out := make([]comparisonPair, 0, len(matches))
	for _, m := range matches {
		after := strings.TrimSpace(m[1])
		before := strings.TrimSpace(m[2])
		if after == "" || before == "" {
			continue
		}
		out = append(out, comparisonPair{BeforeURL: before, AfterURL: after})
	}
	return out
}

func parseGalleryImages(html string) []galleryImage {
	out := make([]galleryImage, 0)

	for _, m := range reAssetImage.FindAllStringSubmatch(html, -1) {
		attrs := m[1]
		src := firstCDNURL(attrs)
		if src == "" {
			continue
		}
		alt := ""
		if am := reAttrAlt.FindStringSubmatch(attrs); len(am) == 2 {
			alt = strings.TrimSpace(am[1])
		}
		out = append(out, galleryImage{URL: src, Alt: alt})
	}
	if len(out) > 0 {
		return out
	}

	// Fallback: data-src / src under /assets/cdn/ (dedupe only in fallback path).
	seen := map[string]struct{}{}
	for _, re := range []*regexp.Regexp{reDataSrcAny, reImgSrcCDN} {
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			src := strings.TrimSpace(m[1])
			key := normalizeAssetKey(src)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, galleryImage{URL: src})
		}
		if len(out) > 0 {
			return out
		}
	}
	return out
}

func firstCDNURL(attrs string) string {
	// Prefer data-src, then src.
	var dataSrc, src string
	for _, m := range reAttrSrc.FindAllStringSubmatch(attrs, -1) {
		v := strings.TrimSpace(m[1])
		if !strings.Contains(strings.ToLower(m[0]), "data-src") {
			if src == "" && isCDNPath(v) {
				src = v
			}
			continue
		}
		if isCDNPath(v) {
			dataSrc = v
			break
		}
	}
	if dataSrc != "" {
		return dataSrc
	}
	return src
}

func isCDNPath(p string) bool {
	return strings.HasPrefix(p, "/assets/cdn/") || strings.HasPrefix(p, "assets/cdn/")
}

func normalizeAssetKey(p string) string {
	p = strings.TrimSpace(p)
	if u, err := url.PathUnescape(p); err == nil {
		p = u
	}
	return strings.ToLower(p)
}

func parseContact(html string) contactContent {
	c := contactContent{
		Headline:     "Get in Touch",
		NameLabel:    "Name",
		EmailLabel:   "Email",
		MessageLabel: "Message",
		SubmitLabel:  "Send Message",
	}
	if m := reContactH2.FindStringSubmatch(html); len(m) == 2 {
		if t := strings.TrimSpace(m[1]); t != "" {
			c.Headline = t
		}
	}
	labels := reContactLabel.FindAllStringSubmatch(html, -1)
	vals := make([]string, 0, len(labels))
	for _, m := range labels {
		t := strings.TrimSpace(m[1])
		if t != "" {
			vals = append(vals, t)
		}
	}
	if len(vals) >= 1 {
		c.NameLabel = vals[0]
	}
	if len(vals) >= 2 {
		c.EmailLabel = vals[1]
	}
	if len(vals) >= 3 {
		c.MessageLabel = vals[2]
	}
	if len(vals) >= 4 {
		c.SubmitLabel = vals[3]
	}
	return c
}
