package cms

import "testing"

func TestFormTemplatesAreNotPageEngines(t *testing.T) {
	for _, id := range FormTemplateIDs {
		if ValidTheme(id) {
			t.Fatalf("%s must not be a generate engine", id)
		}
		if !IsReservedTemplateID(id) {
			t.Fatalf("%s should be a reserved form template id", id)
		}
	}
	for _, th := range []string{
		ThemeBAContent, ThemePanoramaGallery, ThemeTextContent, ThemeLookbookGallery, ThemeRatesContent,
	} {
		if !ValidTheme(th) {
			t.Fatalf("%s must remain a valid theme", th)
		}
	}
}

func TestEnsureSystemFormTemplates(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		FormTemplateFashion:   "Fashion",
		FormTemplateBeauty:    "Beauty",
		FormTemplateLookbook:  "Lookbook",
		FormTemplateEditorial: "Editorial",
		FormTemplateProduct:   "Product",
		FormTemplateManual:    "Manual",
	}
	list, err := s.ListTemplates()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]Template{}
	for _, tmpl := range list {
		if tmpl.Kind == TemplateKindForm {
			found[tmpl.ID] = tmpl
		}
	}
	if len(found) != 6 {
		t.Fatalf("form templates=%d want 6", len(found))
	}
	for id, name := range want {
		tmpl, ok := found[id]
		if !ok {
			t.Fatalf("missing %s", id)
		}
		if tmpl.Name != name || tmpl.FormKey != FormKeyFromTemplateID(id) || !tmpl.IsSystem {
			t.Fatalf("%s: %+v", id, tmpl)
		}
		if tmpl.Theme != ThemeRatesContent {
			t.Fatalf("%s theme=%s", id, tmpl.Theme)
		}
		if ValidTheme(tmpl.ID) {
			t.Fatalf("%s id must not be a page engine", id)
		}
		fields := ParseFormFields(tmpl.DefaultBlocks)
		if !SchemaHasInput(fields) {
			t.Fatalf("%s: expected seeded form fields", id)
		}
		if _, ok := SchemaField(fields, "Imagelink"); !ok {
			t.Fatalf("%s: missing Imagelink", id)
		}
	}
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
}

func TestPatchSystemFormFields(t *testing.T) {
	s := testStore(t)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	cur, err := s.GetTemplate(FormTemplateBeauty)
	if err != nil {
		t.Fatal(err)
	}
	fields := ParseFormFields(cur.DefaultBlocks)
	var color *FormField
	for i := range fields {
		if fields[i].Name() == "colorwork" {
			color = &fields[i]
			break
		}
	}
	if color == nil {
		t.Fatal("beauty colorwork missing")
	}
	opts := color.Options()
	opts = append(opts, FormOption{Value: "Extra Beauty Option", Label: "Extra Beauty Option"})
	rawOpts := make([]any, 0, len(opts))
	for _, o := range opts {
		rawOpts = append(rawOpts, map[string]any{"value": o.Value, "label": o.Label})
	}
	color.Data["options"] = rawOpts
	blocks := make([]map[string]any, 0, len(fields))
	for _, f := range fields {
		blocks = append(blocks, map[string]any{"type": f.Type, "data": f.Data})
	}
	out, err := s.PatchTemplate(FormTemplateBeauty, map[string]any{
		"name":           "Beauty retouch",
		"default_blocks": blocks,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Beauty retouch" {
		t.Fatalf("name=%s", out.Name)
	}
	got := SchemaOptionValues(ParseFormFields(out.DefaultBlocks), "colorwork")
	found := false
	for _, v := range got {
		if v == "Extra Beauty Option" {
			found = true
		}
	}
	if !found {
		t.Fatalf("custom option missing: %v", got)
	}
}

func TestCreatePageRejectsFormTemplateID(t *testing.T) {
	s := testStore(t)
	_, err := s.CreatePage(Page{
		Slug:  "not-a-page",
		Title: "Nope",
		Theme: FormTemplateFashion,
	})
	if err == nil {
		t.Fatal("expected invalid theme")
	}
}

func TestRateFormKeyFromDataPrefersTemplateID(t *testing.T) {
	key := RateFormKeyFromData(map[string]any{
		"form_template_id": FormTemplateBeauty,
	})
	if key != RateKeyBeauty {
		t.Fatalf("got %q", key)
	}
	key = RateFormKeyFromData(map[string]any{"form_key": "editorial"})
	if key != RateKeyEditorial {
		t.Fatalf("fallback got %q", key)
	}
}

func TestBannerAspectDefault(t *testing.T) {
	if NormalizeBannerAspect("") != DefaultBannerAspect || DefaultBannerAspect != "3:4" {
		t.Fatalf("default=%s", DefaultBannerAspect)
	}
	if BannerAspectCSS("3:4") != "3 / 4" {
		t.Fatalf("css=%s", BannerAspectCSS("3:4"))
	}
	style := BannerGridStyle(MustJSON(map[string]any{"banner_aspect": "2:3"}))
	if style != "--rate-banner-aspect: 2 / 3;" {
		t.Fatalf("style=%q", style)
	}
}
