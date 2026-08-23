package cms

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormField is one canvas item on a kind=form template.
type FormField struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type FormOption struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Image  string `json:"image,omitempty"`
	Plaque string `json:"plaque,omitempty"`
}

func DefaultFormFieldTypes() []string {
	return []string{
		BlockFormStep, BlockFormText, BlockFormNumber, BlockFormDate, BlockFormTextarea,
		BlockFormSelect, BlockFormRadio, BlockFormCheckbox, BlockFormRetouch,
		BlockFormHelp, BlockFormFooter, BlockFormHoneypot,
	}
}

func IsEmptyJSONArray(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null" || s == "[]"
}

func ParseFormFields(raw json.RawMessage) []FormField {
	if IsEmptyJSONArray(raw) {
		return nil
	}
	var blocks []struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	out := make([]FormField, 0, len(blocks))
	for _, b := range blocks {
		if !ValidFormBlockType(b.Type) {
			continue
		}
		data := map[string]any{}
		if len(b.Data) > 0 && string(b.Data) != "null" {
			_ = json.Unmarshal(b.Data, &data)
		}
		if data == nil {
			data = map[string]any{}
		}
		out = append(out, FormField{Type: b.Type, Data: data})
	}
	return out
}

func (f FormField) Name() string {
	return strings.TrimSpace(mapStringVal(f.Data, "name"))
}

func (f FormField) Label() string {
	if s := strings.TrimSpace(mapStringVal(f.Data, "label")); s != "" {
		return s
	}
	if s := strings.TrimSpace(mapStringVal(f.Data, "title")); s != "" {
		return s
	}
	if s := strings.TrimSpace(mapStringVal(f.Data, "legend")); s != "" {
		return s
	}
	return f.Name()
}

func (f FormField) Required() bool {
	switch v := f.Data["required"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true") || v == "1"
	default:
		return false
	}
}

func (f FormField) Placeholder() string {
	return mapStringVal(f.Data, "placeholder")
}

func (f FormField) Help() string {
	if s := mapStringVal(f.Data, "help"); s != "" {
		return s
	}
	return mapStringVal(f.Data, "text")
}

func (f FormField) FormatMode() string {
	return strings.ToLower(strings.TrimSpace(mapStringVal(f.Data, "format_mode")))
}

func (f FormField) WrapClass() string {
	return mapStringVal(f.Data, "wrap_class")
}

func (f FormField) HiddenUntilShow() bool {
	if _, ok := f.Data["hidden"]; !ok {
		return f.ShowIf().Field != ""
	}
	switch v := f.Data["hidden"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true") || v == "1"
	default:
		return false
	}
}

type ShowIf struct {
	Field  string
	Values []string
}

func (f FormField) ShowIf() ShowIf {
	raw, ok := f.Data["show_if"]
	if !ok || raw == nil {
		return ShowIf{}
	}
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return ShowIf{}
		}
		field, vals, _ := strings.Cut(s, "=")
		field = strings.TrimSpace(field)
		if field == "" {
			return ShowIf{}
		}
		parts := strings.Split(vals, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return ShowIf{Field: field, Values: out}
	case map[string]any:
		field := strings.TrimSpace(mapStringVal(v, "field"))
		var vals []string
		if arr, ok := v["values"].([]any); ok {
			for _, item := range arr {
				s := strings.TrimSpace(fmt.Sprint(item))
				if s != "" && s != "<nil>" {
					vals = append(vals, s)
				}
			}
		} else if arr, ok := v["in"].([]any); ok {
			for _, item := range arr {
				s := strings.TrimSpace(fmt.Sprint(item))
				if s != "" && s != "<nil>" {
					vals = append(vals, s)
				}
			}
		} else if s := mapStringVal(v, "value"); s != "" {
			vals = []string{s}
		}
		return ShowIf{Field: field, Values: vals}
	default:
		return ShowIf{}
	}
}

func (f FormField) Options() []FormOption {
	raw, ok := f.Data["options"]
	if !ok || raw == nil {
		if f.Type == BlockFormRetouch {
			return DefaultRetouchOptions()
		}
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]FormOption, 0, len(v))
		for _, item := range v {
			switch t := item.(type) {
			case string:
				s := strings.TrimSpace(t)
				if s != "" {
					out = append(out, FormOption{Value: s, Label: s})
				}
			case map[string]any:
				val := strings.TrimSpace(mapStringVal(t, "value"))
				lab := strings.TrimSpace(mapStringVal(t, "label"))
				img := strings.TrimSpace(mapStringVal(t, "image"))
				plaque := strings.TrimSpace(mapStringVal(t, "plaque"))
				if val == "" {
					val = lab
				}
				if lab == "" {
					lab = val
				}
				if plaque == "" && f.Type == BlockFormRetouch {
					plaque = DefaultRetouchPlaque(val)
				}
				if val != "" {
					out = append(out, FormOption{Value: val, Label: lab, Image: img, Plaque: plaque})
				}
			}
		}
		return out
	case []string:
		out := make([]FormOption, 0, len(v))
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, FormOption{Value: s, Label: s})
			}
		}
		return out
	default:
		return nil
	}
}

func (f FormField) OptionValues() []string {
	opts := f.Options()
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		if o.Value != "" {
			out = append(out, o.Value)
		}
	}
	return out
}

func (f FormField) IsInput() bool {
	switch f.Type {
	case BlockFormStep, BlockFormHelp, BlockFormHoneypot:
		return false
	default:
		return true
	}
}

// SchemaOptionValues returns option values for a POST key from a form canvas.
func SchemaOptionValues(fields []FormField, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, f := range fields {
		if f.Name() != name {
			continue
		}
		for _, v := range f.OptionValues() {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

func SchemaField(fields []FormField, name string) (FormField, bool) {
	for _, f := range fields {
		if f.Name() == name {
			return f, true
		}
	}
	return FormField{}, false
}

func SchemaHasInput(fields []FormField) bool {
	for _, f := range fields {
		if f.IsInput() && f.Type != BlockFormHoneypot {
			return true
		}
	}
	return false
}

func DefaultRetouchPlaque(value string) string {
	switch strings.TrimSpace(value) {
	case "1":
		return "LIGHT / RAW"
	case "2":
		return "NATURAL"
	case "3":
		return "CLEAN UP"
	case "4":
		return "FULL TOUCH UP"
	default:
		return ""
	}
}

func DefaultRetouchImage(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	return "/assets/theme/rates/retouch-level-" + v + ".webp"
}

func DefaultRetouchOptions() []FormOption {
	return []FormOption{
		{Value: "1", Image: DefaultRetouchImage("1"), Plaque: DefaultRetouchPlaque("1"), Label: "Light skin retouching that doesn't affect the shadow and age changes to leave your photo as natural as possible"},
		{Value: "2", Image: DefaultRetouchImage("2"), Plaque: DefaultRetouchPlaque("2"), Label: "Removing obvious blemishes, but keeping the model looking natural with all the personal characteristics and texture kept intact"},
		{Value: "3", Image: DefaultRetouchImage("3"), Plaque: DefaultRetouchPlaque("3"), Label: "Focus is on the skin retouching: removing all imperfections, smoothing wrinkles, yet saving natural texture"},
		{Value: "4", Image: DefaultRetouchImage("4"), Plaque: DefaultRetouchPlaque("4"), Label: "Retouching of blemishes, scars, improving texture, removing of unwanted wrinkles, adjusting highlights and features"},
	}
}

func DefaultFormBlocks(formKey string) json.RawMessage {
	key := strings.ToLower(strings.TrimSpace(formKey))
	var blocks []map[string]any
	switch key {
	case RateKeyBeauty:
		blocks = beautyFormBlocks()
	case RateKeyProduct:
		blocks = productFormBlocks()
	case RateKeyManual:
		blocks = manualFormBlocks()
	default:
		// fashion, lookbook, editorial share the fashion field set
		blocks = fashionFormBlocks()
	}
	return MustJSON(blocks)
}

func blk(typ string, data map[string]any) map[string]any {
	return map[string]any{"type": typ, "data": data}
}

func strOpts(vals ...string) []map[string]any {
	out := make([]map[string]any, 0, len(vals))
	for _, v := range vals {
		out = append(out, map[string]any{"value": v, "label": v})
	}
	return out
}

func pairOpts(pairs [][2]string) []map[string]any {
	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]any{"value": p[0], "label": p[1]})
	}
	return out
}

func retouchOpts() []map[string]any {
	out := make([]map[string]any, 0, 4)
	for _, o := range DefaultRetouchOptions() {
		out = append(out, map[string]any{"value": o.Value, "label": o.Label, "image": o.Image, "plaque": o.Plaque})
	}
	return out
}

func formatStdOpts() []map[string]any {
	return strOpts("JPG", "TIFF", "PSD", "Other / Specify in the brief")
}

func formatCutOpts() []map[string]any {
	return strOpts("PNG", "JPG", "TIFF", "TIFF/with layers", "PSD", "PSD/with layers", "Other / Specify in the brief")
}

func profileOptsSeed() []map[string]any {
	return strOpts("Adobe RGB (1998)", "sRGB Profile", "ProPhoto RGB", "e-sRGB", "Other / Specify in the brief")
}

func contactFooterData() map[string]any {
	return map[string]any{
		"legend":          "Contact",
		"name_label":      "Name",
		"contact_label":   "Preferred contact",
		"email_label":     "Email",
		"phone_label":     "Phone",
		"submit_label":    "Submit project",
		"contact_options": strOpts("Phone", "Email", "WhatsApp"),
	}
}

func step1Core(withRetouch bool) []map[string]any {
	out := []map[string]any{
		blk(BlockFormStep, map[string]any{"title": "STEP 1"}),
		blk(BlockFormText, map[string]any{
			"name": "Imagelink", "label": "Share link with images",
			"placeholder": "GoogleDrive, DropBox or other", "required": true,
		}),
		blk(BlockFormNumber, map[string]any{
			"name": "Total", "label": "Total", "required": true, "min": 1,
		}),
		blk(BlockFormDate, map[string]any{
			"name": "Final_delivery", "label": "Final delivery", "required": true,
			"placeholder": "YYYY-MM-DD",
			"help":        "Type YYYY-MM-DD or use the English calendar. Past dates are not allowed.",
		}),
	}
	if withRetouch {
		out = append(out, blk(BlockFormRetouch, map[string]any{
			"name": "Retouch_level", "label": "Retouch level", "required": true,
			"help":    "It will give us the idea of what you prefer, and will not affect the final cost.",
			"options": retouchOpts(),
		}))
	}
	return out
}

func step2Format() []map[string]any {
	return []map[string]any{
		blk(BlockFormStep, map[string]any{"title": "STEP 2"}),
		blk(BlockFormText, map[string]any{
			"name": "Color_Reference", "label": "Color reference",
			"placeholder": "Link to color reference (optional)",
		}),
		blk(BlockFormSelect, map[string]any{
			"name": "Format", "label": "Format", "required": true, "options": formatStdOpts(),
		}),
		blk(BlockFormSelect, map[string]any{
			"name": "Profile", "label": "Profile", "required": true, "options": profileOptsSeed(),
		}),
	}
}

func briefAndChrome() []map[string]any {
	return []map[string]any{
		blk(BlockFormTextarea, map[string]any{
			"name": "Brief", "label": "Brief", "placeholder": "Tell us about the project", "rows": 5,
		}),
		blk(BlockFormFooter, contactFooterData()),
		blk(BlockFormHoneypot, map[string]any{
			"note": "Hidden website field (spam trap). Not shown to visitors.",
		}),
	}
}

func fashionFormBlocks() []map[string]any {
	var out []map[string]any
	out = append(out, step1Core(true)...)
	out = append(out, step2Format()...)
	out = append(out,
		blk(BlockFormStep, map[string]any{"title": "STEP 3"}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "colorwork", "label": "Color Correction",
			"options": strOpts(
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
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "background", "label": "Background Editing",
			"options": strOpts("Dust, Stain, and Scratch Removal", "Clipping mask", "Background Replacement"),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "model", "label": "Model Editing",
			"options": strOpts(
				"Adjust and Change Facial Features (Eyes, Brows, Nose, Lips, Teeth Shape Correction)",
				"Makeup Correction",
				"Manicure Adjustment",
				"Hair Retouching: (Stray Hair Cleanup, Fill Hair Gaps)",
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "clothing", "label": "Clothing Editing",
			"options": strOpts(
				"General Cleaning (Pilling, Dust, Threads)",
				"Wrinkle/Crease Smoothing",
				"Shape Correction",
				"Fix Clothing, Move Clothing (closer to the body)",
				"Clothing Part Replacement from Additional Shots/Plates",
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "footwear", "label": "Footwear Editing",
			"options": strOpts(
				"General Cleaning (Dust, Scratches)",
				"Tape Removal from Shoe Sole",
				"Shoe Resizing to Models Foot Size",
			),
		}),
		blk(BlockFormStep, map[string]any{"title": "STEP 4"}),
	)
	out = append(out, briefAndChrome()...)
	return out
}

func beautyFormBlocks() []map[string]any {
	var out []map[string]any
	out = append(out, step1Core(true)...)
	out = append(out, step2Format()...)
	out = append(out,
		blk(BlockFormStep, map[string]any{"title": "STEP 3"}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "colorwork", "label": "Color Correction",
			"options": strOpts(
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
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "background", "label": "Background Editing",
			"options": strOpts("Dust, Stain, and Scratch Removal", "Clipping mask", "Background Replacement"),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "model", "label": "Model Editing",
			"options": strOpts(
				"Adjust and Change Facial Features (Eyes, Brows, Nose, Lips, Teeth Shape Correction)",
				"Makeup Correction",
				"Manicure Adjustment",
				"Hair Retouching: (Stray Hair Cleanup, Fill Hair Gaps)",
				"Detailed Lip Retouching",
				"Cosmetic Product Retouching in Frame (Lipstick, Powder, Perfume, etc.)",
				"Clothing Retouching (Wrinkle Softening, Shape Correction)",
			),
		}),
		blk(BlockFormStep, map[string]any{"title": "STEP 4"}),
	)
	out = append(out, briefAndChrome()...)
	return out
}

func productFormBlocks() []map[string]any {
	var out []map[string]any
	out = append(out, step1Core(false)...)
	out = append(out, step2Format()...)
	out = append(out,
		blk(BlockFormStep, map[string]any{"title": "STEP 3"}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "colorwork", "label": "Color Correction",
			"options": strOpts(
				"Basic RAW Development",
				"Changing the color on individual elements",
				"Color Correction by Reference",
				"No-Reference Color Image Grading (options to our taste)",
				"Contrast Enhancement",
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "background", "label": "Background Editing",
			"options": strOpts(
				"Dust, Spot, and Scratch Removal",
				"Clipping mask",
				"Background Replacement",
				"Create a mirror image effect",
				"Subject Shadow Addition",
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "objectwork", "label": "Object Editing",
			"options": strOpts(
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
			),
		}),
		blk(BlockFormStep, map[string]any{"title": "STEP 4"}),
	)
	out = append(out, briefAndChrome()...)
	return out
}

func manualFormBlocks() []map[string]any {
	var out []map[string]any
	out = append(out,
		blk(BlockFormSelect, map[string]any{
			"name": "task", "label": "Task", "required": true,
			"options": pairOpts([][2]string{
				{"cut_model", "Only cut out the model"},
				{"cut_object", "Only cut out the object"},
				{"color", "Only color correction"},
				{"hair", "Only hair retouching"},
			}),
		}),
		blk(BlockFormStep, map[string]any{"title": "STEP 1"}),
		blk(BlockFormText, map[string]any{
			"name": "Imagelink", "label": "Share link with images",
			"placeholder": "GoogleDrive, DropBox or other", "required": true,
		}),
		blk(BlockFormNumber, map[string]any{
			"name": "Total", "label": "Total", "required": true, "min": 1,
		}),
		blk(BlockFormDate, map[string]any{
			"name": "Final_delivery", "label": "Final delivery", "required": true,
			"placeholder": "YYYY-MM-DD",
			"help":        "Type YYYY-MM-DD or use the English calendar. Past dates are not allowed.",
		}),
		blk(BlockFormText, map[string]any{
			"name": "Color_Reference", "label": "Color reference",
			"placeholder": "Link to color reference",
			"show_if":     "task=color,hair,cut_model,cut_object",
			"wrap_class":  "rate-color-ref-wrap",
			"hidden":      false,
		}),
		blk(BlockFormText, map[string]any{
			"name": "backclipping", "label": "Background reference",
			"placeholder": "Background reference",
			"show_if":     "task=cut_model,cut_object",
			"hidden":      true,
		}),
		blk(BlockFormSelect, map[string]any{
			"name": "Format", "label": "Format", "required": true,
			"format_mode": "std", "options": formatStdOpts(),
		}),
		blk(BlockFormSelect, map[string]any{
			"name": "Format", "label": "Format", "required": true,
			"format_mode": "cut", "options": formatCutOpts(),
		}),
		blk(BlockFormSelect, map[string]any{
			"name": "Profile", "label": "Profile", "required": true, "options": profileOptsSeed(),
		}),
		blk(BlockFormStep, map[string]any{"title": "STEP 2"}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "colorwork", "label": "Color Correction",
			"show_if": "task=color", "hidden": true,
			"options": strOpts(
				"Basic RAW Development",
				"Color Correction by Reference",
				"No-Reference Color Image Grading (options to our taste)",
				"Change the Color of Individual Elements",
				"Contrast Enhancement",
				"Working with Light Accents: Highlighting the Main Object by Brightness Adjusting",
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "hairretouch", "label": "Hair retouching",
			"show_if": "task=hair", "hidden": true,
			"options": strOpts(
				"Remove Flyaway Hair",
				"Hair Drawing",
				"Fill Hair Gaps",
				"Change Hair Color",
				"Hair Color Enhancement/Saturation Addition",
				"Highlights and Depth Intensification",
				"Hair Smoothing/Length Softening",
			),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "clipping", "label": "Model Clipping:",
			"show_if": "task=cut_model", "hidden": true,
			"options": strOpts("Clipping mask", "Background Replacement"),
		}),
		blk(BlockFormCheckbox, map[string]any{
			"name": "clipping", "label": "Item clipping:",
			"show_if": "task=cut_object", "hidden": true,
			"options": strOpts("Clipping mask", "Background Replacement"),
		}),
		blk(BlockFormStep, map[string]any{"title": "STEP 3"}),
	)
	out = append(out, briefAndChrome()...)
	return out
}
