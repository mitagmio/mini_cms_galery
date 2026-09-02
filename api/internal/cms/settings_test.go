package cms

import "testing"

func TestPutGetContactEmail(t *testing.T) {
	s := testStore(t)
	out, err := s.PutSettings(SiteSettings{
		SiteName:     "Site",
		ContactEmail: "sheyanova@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ContactEmail != "sheyanova@example.com" {
		t.Fatalf("got %q", out.ContactEmail)
	}
	got, err := s.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContactEmail != "sheyanova@example.com" || got.MailtoAddress != got.ContactEmail {
		t.Fatalf("got %+v", got)
	}
	to, err := s.ContactRecipient()
	if err != nil || to != "sheyanova@example.com" {
		t.Fatalf("recipient %q %v", to, err)
	}
}

func TestPutGetYandexMetrika(t *testing.T) {
	s := testStore(t)
	out, err := s.PutSettings(SiteSettings{
		SiteName:             "Site",
		YandexMetrikaEnabled: true,
		YandexMetrikaID:      DefaultYandexMetrikaID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.YandexMetrikaEnabled || out.YandexMetrikaID != DefaultYandexMetrikaID {
		t.Fatalf("got %+v", out)
	}
	out, err = s.PutSettings(SiteSettings{
		SiteName:             "Site",
		YandexMetrikaEnabled: false,
		YandexMetrikaID:      DefaultYandexMetrikaID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.YandexMetrikaEnabled {
		t.Fatal("expected disabled")
	}
	got, err := s.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.YandexMetrikaEnabled || got.YandexMetrikaID != DefaultYandexMetrikaID {
		t.Fatalf("got %+v", got)
	}
	// Empty ID falls back to default on put/get.
	out, err = s.PutSettings(SiteSettings{SiteName: "Site", YandexMetrikaEnabled: true, YandexMetrikaID: ""})
	if err != nil {
		t.Fatal(err)
	}
	if out.YandexMetrikaID != DefaultYandexMetrikaID {
		t.Fatalf("default id got %q", out.YandexMetrikaID)
	}
}

func TestContactEmailFromMailtoAlias(t *testing.T) {
	s := testStore(t)
	out, err := s.PutSettings(SiteSettings{MailtoAddress: "alias@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ContactEmail != "alias@example.com" {
		t.Fatalf("got %q", out.ContactEmail)
	}
}

func TestBackfillContactEmailFromBlock(t *testing.T) {
	s := testStore(t)
	if _, err := s.PutSettings(SiteSettings{SiteName: "Site"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(Page{Slug: "contact", Title: "C", Theme: ThemeTextContent, Status: "published"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceBlocks(p.ID, []Block{{
		Type: BlockContactForm,
		Data: MustJSON(map[string]any{"mailto": "block@example.com"}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.BackfillContactEmail(); err != nil {
		t.Fatal(err)
	}
	st, err := s.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if st.ContactEmail != "block@example.com" {
		t.Fatalf("got %q", st.ContactEmail)
	}
	if err := s.BackfillContactEmail(); err != nil {
		t.Fatal(err)
	}
	st, _ = s.GetSettings()
	if st.ContactEmail != "block@example.com" {
		t.Fatal("second backfill must not clear")
	}
}
