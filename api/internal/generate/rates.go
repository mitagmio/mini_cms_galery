package generate

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"sheyanova.art/api/internal/cms"
)

type RateModal struct {
	Key              string
	Caption          string
	FormID           string
	FormValue        string
	Action           string
	TurnstileSiteKey string
	SuccessMessage   string
	Today            string
	FormHTML         template.HTML
}

func mapString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	switch v := data[key].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

// mapBoolDefaultTrue treats missing/nil as true (backwards compatible for text_backdrop).
func mapBoolDefaultTrue(data map[string]any, key string) bool {
	if data == nil {
		return true
	}
	v, ok := data[key]
	if !ok || v == nil {
		return true
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		if s == "" {
			return true
		}
		return s != "false" && s != "0" && s != "off" && s != "no"
	case float64:
		return t != 0
	case int:
		return t != 0
	case json.Number:
		n, err := t.Float64()
		if err != nil {
			return true
		}
		return n != 0
	default:
		return true
	}
}

func (g *Generator) renderRateBanner(data map[string]any, src, analyzePath, loading string, w, h int) string {
	key := cms.RateFormKeyFromData(data)
	caption := strings.TrimSpace(mapString(data, "caption"))
	if caption == "" && key != "" {
		caption = cms.RateCaption(key)
	}
	alt := strings.TrimSpace(mapString(data, "alt"))
	if alt == "" {
		alt = caption
	}
	from := strings.TrimSpace(mapString(data, "start_from_label"))
	price := strings.TrimSpace(mapString(data, "price"))
	currency := strings.TrimSpace(mapString(data, "currency"))
	esc := template.HTMLEscapeString
	cls := "rate-banner"
	media := ""
	hasImage := strings.TrimSpace(src) != ""
	if hasImage {
		imgSrc := pathURL(src)
		if loading == "" {
			loading = "lazy"
		}
		dim := imgDimensionAttrs(w, h)
		media = fmt.Sprintf(
			`<img class="rate-banner__img" src="%s" alt="%s" loading="%s" decoding="async"%s/>`,
			imgSrc, esc(alt), esc(loading), dim,
		)
	} else {
		cls += " is-placeholder"
	}
	toneCls, overlayStyle := g.resolveRateOverlayTone(mapString(data, "text_color"), analyzePath, hasImage)
	cls += toneCls
	if mapBoolDefaultTrue(data, "text_backdrop") {
		cls += " rate-banner--text-backdrop"
	}
	priceHTML := ""
	if price != "" {
		cur := currency
		if cur == "" {
			cur = "$"
		}
		priceHTML = fmt.Sprintf(
			`<span class="rate-banner__price-row"><span class="rate-banner__currency">%s</span><span class="rate-banner__price">%s</span></span>`,
			esc(cur), esc(price),
		)
	}
	fromHTML := ""
	if from != "" {
		fromHTML = fmt.Sprintf(`<span class="rate-banner__from">%s</span>`, esc(from))
	}
	bannerID := ""
	controls := ""
	if cms.ValidRateFormKey(key) {
		bannerID = fmt.Sprintf(` id="rate-banner-%s"`, esc(key))
		controls = fmt.Sprintf(` aria-controls="%s"`, esc(key))
	}
	return fmt.Sprintf(
		`<button type="button" class="%s" data-rate-key="%s"%s aria-haspopup="dialog"%s>
%s<div class="rate-banner__overlay"%s><div class="rate-banner__plate"><div class="rate-banner__meta">%s%s</div><span class="rate-banner__caption">%s</span></div></div>
</button>`,
		cls, esc(key), bannerID, controls, media, overlayStyle, priceHTML, fromHTML, esc(caption),
	)
}

func (g *Generator) rateModals(p cms.Page) []RateModal {
	captions := map[string]string{}
	for _, b := range p.Blocks {
		if b.Type != cms.BlockRateBanner {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(b.Data, &data)
		key := cms.RateFormKeyFromData(data)
		if !cms.ValidRateFormKey(key) {
			continue
		}
		cap := strings.TrimSpace(mapString(data, "caption"))
		if cap == "" {
			cap = cms.RateCaption(key)
		}
		captions[key] = cap
	}
	today := time.Now().Format("2006-01-02")
	success := "Thank you. Your message has been sent."
	out := make([]RateModal, 0, len(cms.RateFormKeys))
	for _, key := range cms.RateFormKeys {
		cap := captions[key]
		if cap == "" {
			cap = cms.RateCaption(key)
		}
		m := RateModal{
			Key:              key,
			Caption:          cap,
			FormID:           "rates_form_" + key,
			FormValue:        cms.RateFormValue(key),
			Action:           g.contactAction(),
			TurnstileSiteKey: g.Cfg.TurnstileSiteKey,
			SuccessMessage:   success,
			Today:            today,
		}
		m.FormHTML = g.rateFormHTML(key, m)
		out = append(out, m)
	}
	return out
}

func (g *Generator) rateFormHTML(key string, m RateModal) template.HTML {
	if g.formsByKey == nil {
		g.loadTemplateOverrides()
	}
	t, ok := g.formsByKey[key]
	if !ok || !cms.SchemaHasInput(cms.ParseFormFields(t.DefaultBlocks)) {
		return ""
	}
	fields := cms.ParseFormFields(t.DefaultBlocks)
	return template.HTML(renderRateFormFromSchema(fields, m))
}
