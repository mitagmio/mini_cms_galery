package generate

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"sheyanova.art/api/internal/cms"
)

func renderRateFormFromSchema(fields []cms.FormField, m RateModal) string {
	var b strings.Builder
	esc := template.HTMLEscapeString
	fmt.Fprintf(&b, `<form id="%s" class="rate-form email_form contact_form" method="post" action="%s" novalidate data-rate-form="%s">`,
		esc(m.FormID), esc(m.Action), esc(m.FormValue))
	b.WriteByte('\n')
	b.WriteString(renderRateHidden(m))
	b.WriteString(renderRateTitle(m))
	b.WriteString(renderRateStatus())
	hasFooter := false
	for _, f := range fields {
		if f.Type == cms.BlockFormFooter {
			hasFooter = true
		}
		b.WriteString(renderFormField(f, m))
	}
	if !hasFooter {
		b.WriteString(renderContactFooter(cms.FormField{Type: cms.BlockFormFooter, Data: map[string]any{}}, m))
	}
	b.WriteString("</form>\n")
	return b.String()
}

func renderRateHidden(m RateModal) string {
	esc := template.HTMLEscapeString
	return fmt.Sprintf(`<input type="hidden" name="form" value="%s"/>
<div class="contact_form_hp rate-hp" aria-hidden="true">
<label>Website<input type="text" name="website" tabindex="-1" autocomplete="off"/></label>
</div>
<input type="hidden" name="_t" value=""/>
`, esc(m.FormValue))
}

func renderRateTitle(m RateModal) string {
	esc := template.HTMLEscapeString
	return fmt.Sprintf(`<p class="rate-form-caption" id="rate-title-%s">%s</p>
<h2 class="rate-form-heading">PROJECT DETAILS</h2>
`, esc(m.Key), esc(m.Caption))
}

func renderRateStatus() string {
	return `<div class="error_messages contact_form__status rate-form__status" role="status" aria-live="polite"></div>
`
}

func renderFormField(f cms.FormField, m RateModal) string {
	inner := ""
	switch f.Type {
	case cms.BlockFormHoneypot:
		return ""
	case cms.BlockFormStep:
		title := strings.TrimSpace(mapString(f.Data, "title"))
		if title == "" {
			title = strings.TrimSpace(mapString(f.Data, "label"))
		}
		if title == "" {
			return ""
		}
		inner = fmt.Sprintf(`<h3 class="rate-step">%s</h3>`+"\n", template.HTMLEscapeString(title))
	case cms.BlockFormHelp:
		text := f.Help()
		if html := strings.TrimSpace(mapString(f.Data, "html")); html != "" {
			inner = fmt.Sprintf(`<p class="rate-hint">%s</p>`+"\n", html)
			break
		}
		if text == "" {
			return ""
		}
		inner = fmt.Sprintf(`<p class="rate-hint">%s</p>`+"\n", template.HTMLEscapeString(text))
	case cms.BlockFormText:
		inner = renderTextInput(f, "text")
	case cms.BlockFormNumber:
		inner = renderNumberInput(f)
	case cms.BlockFormDate:
		inner = renderDateInput(f, m)
	case cms.BlockFormTextarea:
		inner = renderTextarea(f)
	case cms.BlockFormSelect:
		inner = renderSelect(f)
	case cms.BlockFormRadio:
		inner = renderRadioGroup(f)
	case cms.BlockFormCheckbox:
		inner = renderCheckboxGroup(f)
	case cms.BlockFormRetouch:
		inner = renderRetouch(f)
	case cms.BlockFormFooter:
		inner = renderContactFooter(f, m)
	default:
		return ""
	}
	return wrapField(f, inner)
}

func wrapField(f cms.FormField, inner string) string {
	if inner == "" {
		return ""
	}
	html := inner
	if mode := f.FormatMode(); mode != "" {
		hidden := ""
		if mode == "cut" {
			hidden = " hidden"
		}
		html = fmt.Sprintf(`<div data-format-mode="%s"%s>`+"\n%s</div>\n",
			template.HTMLEscapeString(mode), hidden, html)
	}
	si := f.ShowIf()
	if si.Field != "" && len(si.Values) > 0 {
		hidden := ""
		if f.HiddenUntilShow() {
			hidden = " hidden"
		}
		cls := ""
		if w := f.WrapClass(); w != "" {
			cls = ` class="` + template.HTMLEscapeString(w) + `"`
		}
		html = fmt.Sprintf(`<div data-task-show="%s"%s%s>`+"\n%s</div>\n",
			template.HTMLEscapeString(strings.Join(si.Values, ",")), cls, hidden, html)
	}
	return html
}

func fieldClass() string {
	return "_4ORMAT_module_contact_input"
}

func renderTextInput(f cms.FormField, typ string) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		return ""
	}
	req := ""
	if f.Required() {
		req = " required"
	}
	extra := ""
	if name == "Color_Reference" {
		extra += ` data-rate-color-ref`
	}
	if name == "task" {
		extra += ` data-rate-task`
	}
	ph := f.Placeholder()
	phAttr := ""
	if ph != "" {
		phAttr = fmt.Sprintf(` placeholder="%s"`, esc(ph))
	}
	max := 2000
	if name == "name" || name == "Name" {
		max = 200
	}
	if name == "email" || name == "Email" {
		max = 254
		typ = "email"
	}
	return fmt.Sprintf(`<label class="rate-field">%s
<input class="%s" name="%s" type="%s"%s maxlength="%d"%s autocomplete="off"%s/>
</label>
`, esc(f.Label()), fieldClass(), esc(name), esc(typ), req, max, phAttr, extra)
}

func renderNumberInput(f cms.FormField) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		return ""
	}
	req := ""
	if f.Required() {
		req = " required"
	}
	min := 1
	if v, ok := f.Data["min"]; ok {
		switch t := v.(type) {
		case float64:
			min = int(t)
		case int:
			min = t
		case string:
			if n, err := strconv.Atoi(t); err == nil {
				min = n
			}
		}
	}
	return fmt.Sprintf(`<label class="rate-field">%s
<input class="%s" name="%s" type="number"%s min="%d" step="1" inputmode="numeric"/>
</label>
`, esc(f.Label()), fieldClass(), esc(name), req, min)
}

func renderDateInput(f cms.FormField, m RateModal) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		name = "Final_delivery"
	}
	req := ""
	if f.Required() {
		req = " required"
	}
	label := f.Label()
	if label == "" {
		label = "Final delivery"
	}
	help := f.Help()
	if help == "" {
		help = "Type YYYY-MM-DD or use the English calendar. Past dates are not allowed."
	}
	hintID := "rate-date-hint-" + m.Key
	return fmt.Sprintf(`<label class="rate-field">%s
<span class="rate-date" lang="en">
<span class="rate-date__row">
<input class="%s rate-date__input" name="%s" type="text"%s maxlength="10" placeholder="YYYY-MM-DD" inputmode="numeric" autocomplete="off" spellcheck="false" data-rate-date data-rate-min="%s" aria-describedby="%s"/>
<button type="button" class="rate-date__btn" data-rate-date-toggle aria-label="Open calendar">Calendar</button>
</span>
</span>
<span class="rate-hint" id="%s">%s</span>
</label>
`, esc(label), fieldClass(), esc(name), req, esc(m.Today), esc(hintID), esc(hintID), esc(help))
}

func renderTextarea(f cms.FormField) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		return ""
	}
	req := ""
	if f.Required() {
		req = " required"
	}
	rows := 5
	if v, ok := f.Data["rows"]; ok {
		switch t := v.(type) {
		case float64:
			rows = int(t)
		case int:
			rows = t
		}
	}
	ph := f.Placeholder()
	phAttr := ""
	if ph != "" {
		phAttr = fmt.Sprintf(` placeholder="%s"`, esc(ph))
	}
	return fmt.Sprintf(`<label class="rate-field">%s
<textarea class="%s" name="%s" rows="%d" maxlength="8000"%s%s></textarea>
</label>
`, esc(f.Label()), fieldClass(), esc(name), rows, req, phAttr)
}

func renderSelect(f cms.FormField) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		return ""
	}
	req := ""
	if f.Required() {
		req = " required"
	}
	extra := ""
	if name == "task" {
		extra = ` data-rate-task`
	}
	var opts strings.Builder
	opts.WriteString(`<option value="" disabled selected>Select</option>`)
	for _, o := range f.Options() {
		fmt.Fprintf(&opts, `<option value="%s">%s</option>`, esc(o.Value), esc(o.Label))
	}
	return fmt.Sprintf(`<label class="rate-field">%s
<select class="%s" name="%s"%s%s>
%s
</select>
</label>
`, esc(f.Label()), fieldClass(), esc(name), req, extra, opts.String())
}

func renderRadioGroup(f cms.FormField) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<fieldset class="rate-fieldset"><legend>%s</legend>`, esc(f.Label()))
	if help := f.Help(); help != "" {
		fmt.Fprintf(&b, `<p class="rate-hint">%s</p>`, esc(help))
	}
	for i, o := range f.Options() {
		req := ""
		if f.Required() && i == 0 {
			req = " required"
		}
		fmt.Fprintf(&b, `<label class="rate-check"><input type="radio" name="%s" value="%s"%s/> %s</label>`,
			esc(name), esc(o.Value), req, esc(o.Label))
	}
	b.WriteString("</fieldset>\n")
	return b.String()
}

func renderCheckboxGroup(f cms.FormField) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<fieldset class="rate-fieldset"><legend>%s</legend>`, esc(f.Label()))
	if help := f.Help(); help != "" {
		fmt.Fprintf(&b, `<p class="rate-hint">%s</p>`, esc(help))
	}
	for _, o := range f.Options() {
		fmt.Fprintf(&b, `<label class="rate-check"><input type="checkbox" name="%s" value="%s"/> %s</label>`+"\n",
			esc(name), esc(o.Value), esc(o.Label))
	}
	b.WriteString("</fieldset>\n")
	return b.String()
}

func renderRetouch(f cms.FormField) string {
	esc := template.HTMLEscapeString
	name := f.Name()
	if name == "" {
		name = "Retouch_level"
	}
	label := f.Label()
	if label == "" {
		label = "Retouch level"
	}
	help := f.Help()
	if help == "" {
		help = "It will give us the idea of what you prefer, and will not affect the final cost."
	}
	opts := f.Options()
	if len(opts) == 0 {
		opts = cms.DefaultRetouchOptions()
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<fieldset class="rate-fieldset rate-retouch-level">
<legend>%s</legend>
<p class="rate-hint">%s</p>
<div class="rate-retouch-grid">
`, esc(label), esc(help))
	for i, o := range opts {
		req := ""
		if (f.Required() || true) && i == 0 {
			req = " required"
		}
		img := o.Image
		if img == "" {
			img = fmt.Sprintf("/assets/theme/rates/retouch-level-%s.gif", o.Value)
		}
		text := o.Label
		if !strings.HasPrefix(strings.TrimSpace(text), o.Value) {
			text = o.Value + ". " + text
		}
		fmt.Fprintf(&b, `<label class="rate-retouch"><input type="radio" name="%s" value="%s"%s/>
<span class="rate-retouch__frame"><img src="%s" alt="Retouch level %s"/></span>
<span class="rate-retouch__text">%s</span>
</label>
`, esc(name), esc(o.Value), req, esc(img), esc(o.Value), esc(text))
	}
	b.WriteString("</div>\n</fieldset>\n")
	return b.String()
}

func renderContactFooter(f cms.FormField, m RateModal) string {
	esc := template.HTMLEscapeString
	legend := strings.TrimSpace(mapString(f.Data, "legend"))
	if legend == "" {
		legend = "Contact"
	}
	nameL := strings.TrimSpace(mapString(f.Data, "name_label"))
	if nameL == "" {
		nameL = "Name"
	}
	contactL := strings.TrimSpace(mapString(f.Data, "contact_label"))
	if contactL == "" {
		contactL = "Preferred contact"
	}
	emailL := strings.TrimSpace(mapString(f.Data, "email_label"))
	if emailL == "" {
		emailL = "Email"
	}
	phoneL := strings.TrimSpace(mapString(f.Data, "phone_label"))
	if phoneL == "" {
		phoneL = "Phone"
	}
	submitL := strings.TrimSpace(mapString(f.Data, "submit_label"))
	if submitL == "" {
		submitL = "Submit project"
	}
	opts := []cms.FormOption{
		{Value: "Phone", Label: "Phone"},
		{Value: "Email", Label: "Email"},
		{Value: "WhatsApp", Label: "WhatsApp"},
	}
	if custom := parseFooterContactOptions(f.Data); len(custom) > 0 {
		opts = custom
	}
	var sel strings.Builder
	sel.WriteString(`<option value="" disabled selected>Select</option>`)
	for _, o := range opts {
		fmt.Fprintf(&sel, `<option value="%s">%s</option>`, esc(o.Value), esc(o.Label))
	}
	turnstile := ""
	if m.TurnstileSiteKey != "" {
		// Explicit host (not .cf-turnstile): implicit scan would run inside display:none
		// overlays and never mint a token. Size is omitted — render() only accepts
		// normal|flexible|compact; Invisible vs Managed is the dashboard widget type.
		turnstile = fmt.Sprintf(`
<div class="js-turnstile" data-sitekey="%s"></div>`, esc(m.TurnstileSiteKey))
	}
	return fmt.Sprintf(`<fieldset class="rate-fieldset rate-contact">
<legend>%s</legend>
<label class="rate-field">%s
<input class="%s" name="name" type="text" required maxlength="200" autocomplete="name"/>
</label>
<label class="rate-field">%s
<select class="%s" name="Contact" required>
%s
</select>
</label>
<label class="rate-field">%s
<input class="%s" name="email" type="email" required maxlength="254" autocomplete="email"/>
</label>
<label class="rate-field">%s
<input class="%s" name="Phone" type="tel" maxlength="200" autocomplete="tel"/>
</label>
</fieldset>
<fieldset class="submit _4ORMAT_module_submit_left">
<input class="btn primary _4ORMAT_module_contact_btn _4ORMAT_module_contact_input" type="submit" value="%s"/>
<button type="button" class="rate-form-done" data-rate-close="1" hidden>Close</button>
</fieldset>%s
`, esc(legend), esc(nameL), fieldClass(), esc(contactL), fieldClass(), sel.String(),
		esc(emailL), fieldClass(), esc(phoneL), fieldClass(), esc(submitL), turnstile)
}

func parseFooterContactOptions(data map[string]any) []cms.FormOption {
	if data == nil {
		return nil
	}
	f := cms.FormField{Type: cms.BlockFormSelect, Data: map[string]any{"options": data["contact_options"]}}
	return f.Options()
}
