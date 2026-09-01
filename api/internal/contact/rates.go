package contact

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sheyanova.art/api/internal/cms"
)

var fashionColorwork = []string{
	"Basic RAW Development (Camera RAW or Capture One)",
	"Removing Unwanted Color Casts (on the whites of the eyes, on the teeth)",
	"Skin Tone Balancing (for example, reduce redness in hands/fingers)",
	"Even Out Body Exposure (for example, when the neck and chest area lighter in relation to the face)",
	"Brightening Eye Whites and Teeth",
	"Color Correction by Reference",
	"No-Reference Color Image Grading (options to our taste)",
	"Selective Color Adjustment",
	"Changing the color on individual elements",
	"Working with Light Accents: Highlighting the Main Object by Brightness Adjusting",
}

var beautyColorwork = []string{
	"Basic RAW Development (Camera RAW or Capture One)",
	"Removing Unwanted Color Casts (on the whites of the eyes, on the teeth)",
	"Skin Tone Balancing (for example, reduce redness in hands/fingers)",
	"Even Out Body Exposure (for example, when the neck and chest area lighter in relation to the face)",
	"Brightening Eye Whites and Teeth",
	"Color Correction by Reference",
	"No-Reference Color Image Grading (options to our taste)",
	"Changing the color on individual elements (for example, makeup/nail/lipstick colors)",
	"Contrast Enhancement",
	"Working with Light Accents: Highlighting the Main Object by Brightness Adjusting",
}

var productColorwork = []string{
	"Basic RAW Development",
	"Changing the color on individual elements",
	"Color Correction by Reference",
	"No-Reference Color Image Grading (options to our taste)",
	"Contrast Enhancement",
}

var manualColorwork = []string{
	"Basic RAW Development",
	"Color Correction by Reference",
	"No-Reference Color Image Grading (options to our taste)",
	"Change the Color of Individual Elements",
	"Contrast Enhancement",
	"Working with Light Accents: Highlighting the Main Object by Brightness Adjusting",
}

var fashionBackground = []string{
	"Dust, Stain, and Scratch Removal",
	"Clipping mask",
	"Background Replacement",
}

var productBackground = []string{
	"Dust, Spot, and Scratch Removal",
	"Clipping mask",
	"Background Replacement",
	"Create a mirror image effect",
	"Subject Shadow Addition",
}

var fashionModel = []string{
	"Adjust and Change Facial Features (Eyes, Brows, Nose, Lips, Teeth Shape Correction)",
	"Makeup Correction",
	"Manicure Adjustment",
	"Hair Retouching: (Stray Hair Cleanup, Fill Hair Gaps)",
}

var beautyModel = []string{
	"Adjust and Change Facial Features (Eyes, Brows, Nose, Lips, Teeth Shape Correction)",
	"Makeup Correction",
	"Manicure Adjustment",
	"Hair Retouching: (Stray Hair Cleanup, Fill Hair Gaps)",
	"Detailed Lip Retouching",
	"Cosmetic Product Retouching in Frame (Lipstick, Powder, Perfume, etc.)",
	"Clothing Retouching (Wrinkle Softening, Shape Correction)",
}

var fashionClothing = []string{
	"General Cleaning (Pilling, Dust, Threads)",
	"Wrinkle/Crease Smoothing",
	"Shape Correction",
	"Fix Clothing, Move Clothing (closer to the body)",
	"Clothing Part Replacement from Additional Shots/Plates",
}

var fashionFootwear = []string{
	"General Cleaning (Dust, Scratches)",
	"Tape Removal from Shoe Sole",
	"Shoe Resizing to Models Foot Size",
}

var productObjectwork = []string{
	"Dust, Spot, Scratch, Chip, and Minor Flaw Removal",
	"Object Alignment, Perspective Correction",
	"Highlights Correction, Shape Enhancement, Transition Smoothing",
	"Highlight Addition",
	"Glare Reflection Removal",
	"Refining Facets of Diamonds",
	"Sharpening",
	"Brightness&Contrast Enhancement",
	"Improving lights and Shadows",
	"Metal Polishing",
	"Text and Logo Editing (add highlight/replacement from additional files or delete unnecessary text)",
	"Image Compositing, Object Addition and Replacement",
}

var manualHair = []string{
	"Remove Flyaway Hair",
	"Hair Drawing",
	"Fill Hair Gaps",
	"Change Hair Color",
	"Hair Color Enhancement/Saturation Addition",
	"Highlights and Depth Intensification",
	"Hair Smoothing/Length Softening",
}

var clippingOpts = []string{"Clipping mask", "Background Replacement"}

var formatStd = []string{"JPG", "TIFF", "PSD", "Other / Specify in the brief"}
var formatCut = []string{"PNG", "JPG", "TIFF", "TIFF/with layers", "PSD", "PSD/with layers", "Other / Specify in the brief"}
var profileOpts = []string{"Adobe RGB (1998)", "sRGB Profile", "ProPhoto RGB", "e-sRGB", "Other / Specify in the brief"}
var retouchLevels = []string{"1", "2", "3", "4"}
var retouchLevelLabels = map[string]string{
	"1": "Light skin retouching that doesn't affect the shadow and age changes to leave your photo as natural as possible",
	"2": "Removing obvious blemishes, but keeping the model looking natural with all the personal characteristics",
	"3": "Focus is on the skin retouching: removing all imperfections, smoothing wrinkles, yet saving natural texture",
	"4": "Retouching of blemishes, scars, improving texture, removing of unwanted wrinkles, adjusting highlights and features",
}
var contactMethods = []string{"Phone", "Email", "WhatsApp"}
var manualTasks = []string{"cut_model", "cut_object", "color", "hair"}
var manualTaskLabels = map[string]string{
	"cut_model":  "Only cut out the model",
	"cut_object": "Only cut out the object",
	"color":      "Only color correction",
	"hair":       "Only hair retouching",
}

var ratesKnown = map[string]bool{
	"rates_fashion":   true,
	"rates_beauty":    true,
	"rates_lookbook":  true,
	"rates_editorial": true,
	"rates_product":   true,
	"rates_manual":    true,
}

func validateRates(form string, req submitReq, schema []cms.FormField) (name, emailAddr, errMsg string) {
	if !ratesKnown[form] {
		return "", "", "Unknown form."
	}
	name = strings.TrimSpace(req.Name)
	emailAddr = strings.TrimSpace(req.Email)
	if name == "" {
		return "", "", "Please enter your name."
	}
	if utf8.RuneCountInString(name) > maxName {
		return "", "", "Name is too long."
	}
	if emailAddr == "" || !validEmail(emailAddr) {
		return "", "", "Please enter a valid email address."
	}
	if utf8.RuneCountInString(emailAddr) > maxEmail {
		return "", "", "Email is too long."
	}
	if hasCRLF(name) || hasCRLF(emailAddr) {
		return "", "", "Invalid input."
	}

	f := req.Fields
	if msg := requireString(f, "Imagelink", "Please share a link with images."); msg != "" {
		return "", "", msg
	}
	if _, ok := parseTotal(fieldAny(f, "Total")); !ok {
		return "", "", "Please enter the number of photos (at least 1)."
	}
	if msg := requireString(f, "Final_delivery", "Please choose a delivery date."); msg != "" {
		return "", "", msg
	}
	iso, ok := normalizeDeliveryDate(fieldString(f, "Final_delivery"))
	if !ok {
		return "", "", "Please choose a delivery date (YYYY-MM-DD)."
	}
	f["Final_delivery"] = iso
	formatList := unionAllow(formatStd, cms.SchemaOptionValues(schema, "Format"))
	if form == "rates_manual" {
		task := fieldString(f, "task")
		taskAllow := unionAllow(manualTasks, cms.SchemaOptionValues(schema, "task"))
		if !inList(task, taskAllow) {
			return "", "", "Please choose a task."
		}
		if task == "cut_model" || task == "cut_object" {
			formatList = unionAllow(formatCut, cms.SchemaOptionValues(schema, "Format"))
		}
		colorRef := fieldString(f, "Color_Reference")
		if (task == "color" || task == "hair") && colorRef == "" {
			return "", "", "Please add a color reference."
		}
		if utf8.RuneCountInString(colorRef) > maxField {
			return "", "", "Color reference is too long."
		}
		if utf8.RuneCountInString(fieldString(f, "backclipping")) > maxField {
			return "", "", "Background reference is too long."
		}
	} else if form != "rates_product" {
		retouchAllow := unionAllow(retouchLevels, cms.SchemaOptionValues(schema, "Retouch_level"))
		if !inList(fieldString(f, "Retouch_level"), retouchAllow) {
			return "", "", "Please choose a retouch level."
		}
	}
	if !inList(fieldString(f, "Format"), formatList) {
		return "", "", "Please choose a file format."
	}
	profileAllow := unionAllow(profileOpts, cms.SchemaOptionValues(schema, "Profile"))
	if !inList(fieldString(f, "Profile"), profileAllow) {
		return "", "", "Please choose a color profile."
	}
	method := fieldString(f, "Contact")
	contactAllow := unionAllow(contactMethods, cms.SchemaOptionValues(schema, "Contact"))
	if !inList(method, contactAllow) {
		return "", "", "Please choose a contact method."
	}
	phone := fieldString(f, "Phone")
	if (method == "Phone" || method == "WhatsApp") && phone == "" {
		return "", "", "Please enter a phone number."
	}
	if utf8.RuneCountInString(phone) > maxField {
		return "", "", "Phone is too long."
	}
	if utf8.RuneCountInString(fieldString(f, "Brief")) > maxBrief {
		return "", "", "Brief is too long."
	}
	if utf8.RuneCountInString(fieldString(f, "Color_Reference")) > maxField {
		return "", "", "Color reference is too long."
	}
	if utf8.RuneCountInString(fieldString(f, "Imagelink")) > maxField {
		return "", "", "Image link is too long."
	}
	if utf8.RuneCountInString(fieldString(f, "Final_delivery")) > 32 {
		return "", "", "Delivery date is invalid."
	}
	listKeys := []string{"colorwork", "background", "model", "clothing", "footwear", "objectwork", "hairretouch", "clipping"}
	seenList := map[string]bool{}
	for _, key := range listKeys {
		seenList[key] = true
		vals := asStringSlice(fieldAny(f, key))
		if len(vals) > maxArray {
			return "", "", "Too many options selected."
		}
		allow := unionAllow(allowlistFor(form, key), cms.SchemaOptionValues(schema, key))
		for _, v := range vals {
			if utf8.RuneCountInString(v) > maxField {
				return "", "", "An option is too long."
			}
			if len(allow) > 0 && !inList(v, allow) {
				return "", "", "Invalid option selected."
			}
		}
	}
	for _, fld := range schema {
		if !fld.IsInput() || fld.Type == cms.BlockFormFooter {
			continue
		}
		key := fld.Name()
		if key == "" || seenList[key] || knownRatesKey[key] {
			continue
		}
		if fld.Type == cms.BlockFormCheckbox || fld.Type == cms.BlockFormRadio {
			vals := asStringSlice(fieldAny(f, key))
			if len(vals) > maxArray {
				return "", "", "Too many options selected."
			}
			allow := fld.OptionValues()
			for _, v := range vals {
				if utf8.RuneCountInString(v) > maxField {
					return "", "", "An option is too long."
				}
				if len(allow) > 0 && !inList(v, allow) {
					return "", "", "Invalid option selected."
				}
			}
			continue
		}
		if utf8.RuneCountInString(fieldString(f, key)) > maxField {
			return "", "", key + " is too long."
		}
	}
	return name, emailAddr, ""
}

var knownRatesKey = map[string]bool{
	"form": true, "name": true, "Name": true, "email": true, "Email": true,
	"message": true, "company": true, "website": true, "subject": true,
	"_t": true, "cf-turnstile-response": true,
	"Imagelink": true, "Total": true, "Final_delivery": true, "Retouch_level": true,
	"Color_Reference": true, "backclipping": true, "Format": true, "Profile": true,
	"Contact": true, "Phone": true, "Brief": true, "task": true,
	"colorwork": true, "background": true, "model": true, "clothing": true,
	"footwear": true, "objectwork": true, "hairretouch": true, "clipping": true,
}

func unionAllow(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(base)+len(extra))
	for _, s := range base {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range extra {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func allowlistFor(form, key string) []string {
	switch form {
	case "rates_fashion", "rates_lookbook", "rates_editorial":
		switch key {
		case "colorwork":
			return fashionColorwork
		case "background":
			return fashionBackground
		case "model":
			return fashionModel
		case "clothing":
			return fashionClothing
		case "footwear":
			return fashionFootwear
		}
	case "rates_beauty":
		switch key {
		case "colorwork":
			return beautyColorwork
		case "background":
			return fashionBackground
		case "model":
			return beautyModel
		}
	case "rates_product":
		switch key {
		case "colorwork":
			return productColorwork
		case "background":
			return productBackground
		case "objectwork":
			return productObjectwork
		}
	case "rates_manual":
		switch key {
		case "colorwork":
			return manualColorwork
		case "hairretouch":
			return manualHair
		case "clipping":
			return clippingOpts
		}
	}
	return nil
}

func formatRatesBody(form string, req submitReq, name, email, ip string) string {
	plain, _ := formatRatesEmail(form, req, name, email, ip, nil)
	return plain
}

func formatRatesEmail(form string, req submitReq, name, email, ip string, schema []cms.FormField) (plain, htmlBody string) {
	label := strings.ToUpper(strings.TrimPrefix(form, "rates_"))
	intro := fmt.Sprintf("New request from sheyanova.art RATES (%s)", label)

	type item struct {
		label string
		value string
		url   string
		list  []string
	}
	var rows []item
	add := func(it item) {
		if it.value == "" && it.url == "" && len(it.list) == 0 {
			return
		}
		rows = append(rows, it)
	}

	add(item{label: "Name", value: name})
	add(item{label: "Email", value: email})
	if v := fieldString(req.Fields, "Contact"); v != "" {
		add(item{label: "Preferred contact", value: v})
	}
	if v := fieldString(req.Fields, "Phone"); v != "" {
		add(item{label: "Phone", value: v})
	}

	if v := fieldString(req.Fields, "task"); v != "" {
		if pretty, ok := manualTaskLabels[v]; ok {
			v = pretty
		}
		add(item{label: "Task", value: v})
	}
	if v := fieldString(req.Fields, "Imagelink"); v != "" {
		add(item{label: "Images", url: absoluteURL(v)})
	}
	if v := fieldString(req.Fields, "Total"); v != "" {
		add(item{label: "Total", value: v})
	}
	if v := fieldString(req.Fields, "Final_delivery"); v != "" {
		if iso, ok := normalizeDeliveryDate(v); ok {
			v = iso
		}
		add(item{label: "Final delivery", value: v})
	}
	if v := fieldString(req.Fields, "Retouch_level"); v != "" {
		text := v
		if lab := retouchLevelLabels[v]; lab != "" {
			text = v + " — " + lab
		}
		add(item{label: "Retouch level", value: text})
	}
	if v := fieldString(req.Fields, "Color_Reference"); v != "" {
		add(item{label: "Color reference", url: absoluteURL(v)})
	}
	if v := fieldString(req.Fields, "backclipping"); v != "" {
		add(item{label: "Background reference", url: absoluteURL(v)})
	}
	if v := fieldString(req.Fields, "Format"); v != "" {
		add(item{label: "Format", value: v})
	}
	if v := fieldString(req.Fields, "Profile"); v != "" {
		add(item{label: "Profile", value: v})
	}

	listKeys := []struct{ key, label string }{
		{"colorwork", "Color correction"},
		{"background", "Background editing"},
		{"model", "Model editing"},
		{"clothing", "Clothing editing"},
		{"footwear", "Footwear editing"},
		{"objectwork", "Object editing"},
		{"hairretouch", "Hair retouching"},
		{"clipping", "Clipping"},
	}
	for _, lk := range listKeys {
		kept := keptOptions(form, req.Fields, lk.key, cms.SchemaOptionValues(schema, lk.key))
		if len(kept) == 0 {
			continue
		}
		add(item{label: lk.label, list: kept})
	}
	if v := fieldString(req.Fields, "Brief"); v != "" {
		add(item{label: "Brief", value: v})
	}
	used := map[string]bool{}
	for k := range knownRatesKey {
		used[k] = true
	}
	for _, fld := range schema {
		key := fld.Name()
		if key == "" || used[key] || !fld.IsInput() || fld.Type == cms.BlockFormFooter {
			continue
		}
		used[key] = true
		label := fld.Label()
		if label == "" {
			label = key
		}
		if fld.Type == cms.BlockFormCheckbox || fld.Type == cms.BlockFormRadio {
			list := asStringSlice(fieldAny(req.Fields, key))
			if len(list) == 0 {
				continue
			}
			add(item{label: label, list: list})
			continue
		}
		v := fieldString(req.Fields, key)
		if v == "" {
			continue
		}
		if utf8.RuneCountInString(v) > maxField {
			v = string([]rune(v)[:maxField])
		}
		add(item{label: label, value: v})
	}
	for key, raw := range req.Fields {
		if used[key] {
			continue
		}
		used[key] = true
		if list := asStringSlice(raw); len(list) > 1 || (len(list) == 1 && (isSlice(raw))) {
			add(item{label: key, list: list})
			continue
		}
		v := asString(raw)
		if v == "" {
			continue
		}
		if utf8.RuneCountInString(v) > maxField {
			v = string([]rune(v)[:maxField])
		}
		add(item{label: key, value: v})
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", intro)
	for _, row := range rows {
		if len(row.list) > 0 {
			fmt.Fprintf(&b, "%s:\n", row.label)
			for _, line := range row.list {
				fmt.Fprintf(&b, "- %s\n", line)
			}
			b.WriteByte('\n')
			continue
		}
		if row.url != "" {
			fmt.Fprintf(&b, "%s:\n%s\n\n", row.label, row.url)
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", row.label, row.value)
	}
	if ip != "" {
		fmt.Fprintf(&b, "\nSubmitted from IP: %s\n", ip)
	}

	var h strings.Builder
	h.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"/></head>`)
	h.WriteString(`<body style="font-family:Georgia,serif;color:#2f2f2f;background:#f6f1f1;">`)
	fmt.Fprintf(&h, "<p>%s</p><ul>", html.EscapeString(intro))
	for _, row := range rows {
		if len(row.list) > 0 {
			fmt.Fprintf(&h, "<li>%s<ul>", html.EscapeString(row.label))
			for _, line := range row.list {
				fmt.Fprintf(&h, "<li>%s</li>", html.EscapeString(line))
			}
			h.WriteString("</ul></li>")
			continue
		}
		if row.url != "" {
			esc := html.EscapeString(row.url)
			fmt.Fprintf(&h, `<li>%s<br/><a href="%s">%s</a></li>`, html.EscapeString(row.label), esc, esc)
			continue
		}
		fmt.Fprintf(&h, "<li>%s: %s</li>", html.EscapeString(row.label), html.EscapeString(row.value))
	}
	h.WriteString("</ul>")
	if ip != "" {
		fmt.Fprintf(&h, `<p style="color:#7a7070;font-size:12px">Submitted from IP: %s</p>`, html.EscapeString(ip))
	}
	h.WriteString("</body></html>")
	return b.String(), h.String()
}

func keptOptions(form string, fields map[string]any, key string, extra []string) []string {
	v := fieldAny(fields, key)
	arr := asStringSlice(v)
	if len(arr) == 0 {
		return nil
	}
	allow := unionAllow(allowlistFor(form, key), extra)
	var kept []string
	for _, item := range arr {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if len(allow) > 0 && !inList(item, allow) {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func isSlice(v any) bool {
	switch v.(type) {
	case []any, []string:
		return true
	default:
		return false
	}
}

func absoluteURL(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	if strings.HasPrefix(lower, "www.") {
		return "https://" + s
	}
	return s
}

func normalizeDeliveryDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	layouts := []string{"2006-01-02", "02-01-2006", "02/01/2006"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

func requireString(f map[string]any, key, emptyMsg string) string {
	s := fieldString(f, key)
	if s == "" {
		return emptyMsg
	}
	if utf8.RuneCountInString(s) > maxField {
		return key + " is too long."
	}
	return ""
}

func parseTotal(v any) (string, bool) {
	s := asString(v)
	if s == "" {
		return "", false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n < 1 {
		return "", false
	}
	return s, true
}

func inList(v string, list []string) bool {
	for _, item := range list {
		if v == item {
			return true
		}
	}
	return false
}

func fieldString(fields map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := asString(fieldAny(fields, k)); s != "" {
			return s
		}
	}
	return ""
}

func fieldAny(fields map[string]any, key string) any {
	if fields == nil {
		return nil
	}
	return fields[key]
}

func fieldInt64(fields map[string]any, key string) int64 {
	v := fieldAny(fields, key)
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		s := asString(v)
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	}
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []any, []string:
		parts := asStringSlice(t)
		return strings.Join(parts, ", ")
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s := asString(item)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}
