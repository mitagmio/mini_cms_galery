package generate

import (
	"bytes"
	"encoding/json"
	"html/template"
	"strings"
)

type ContactFormInput struct {
	APIURL           string
	FormID           string
	NameLabel        string
	EmailLabel       string
	MessageLabel     string
	SubmitLabel      string
	SuccessMessage   string
	TurnstileSiteKey string
}

var contactFormTmpl = template.Must(template.New("contact_form").Parse(`
<div class="_4ORMAT_content_page_row _4ormat_sort_item _4ORMAT_module_contact_07">
<div class="twocol spacer"></div>
<div class="eightcol">
<style>
.contact_form__status{margin:0 0 14px;min-height:1.2em;font-size:0.95em}
.contact_form__status.is-ok{color:#2a7a3a}
.contact_form__status.is-err{color:#c00}
.contact_form.is-busy input[type=submit]{opacity:.6;pointer-events:none}
.contact_form{position:relative}
.contact_form_hp{position:absolute;left:-10000px;top:auto;width:1px;height:1px;overflow:hidden}
</style>
<form id="{{.FormID}}" class="email_form contact_form" method="post" action="{{.Action}}" novalidate>
<div class="error_messages contact_form__status" role="status" aria-live="polite"></div>
<div class="row naked">
<div class="sixcol">
<label>{{.NameLabel}}<input class="_4ORMAT_module_contact_input" name="name" type="text" required maxlength="200" autocomplete="name"/></label>
</div>
<div class="sixcol last">
<label>{{.EmailLabel}}<input class="_4ORMAT_module_contact_input" name="email" type="email" required maxlength="254" autocomplete="email"/></label>
</div>
</div>
<div class="row naked last">
<div class="twelvecol last">
<label>{{.MessageLabel}}<textarea class="_4ORMAT_module_contact_input" name="message" rows="6" required maxlength="8000"></textarea></label>
</div>
</div>
<div class="contact_form_hp" aria-hidden="true">
<label>Website<input type="text" name="website" tabindex="-1" autocomplete="off"/></label>
</div>
<input type="hidden" name="_t" value=""/>
<fieldset class="submit _4ORMAT_module_submit_left">
<input class="btn primary _4ORMAT_module_contact_btn _4ORMAT_module_contact_input" type="submit" value="{{.SubmitLabel}}"/>
</fieldset>
{{- if .TurnstileSiteKey }}
<div class="js-turnstile" data-sitekey="{{.TurnstileSiteKey}}"></div>
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"></script>
{{- end }}
</form>
<noscript><p class="contact_form__status is-err">JavaScript is required to send this form.</p></noscript>
<script src="/assets/theme/form-submit.js?v=4"></script>
<script>
(function(){
  var form = document.getElementById({{.FormIDJSON}});
  if (!form || !window.SheyanovaForms) return;
  var successMsg = {{.SuccessJSON}};
  window.SheyanovaForms.attach(form, {
    action: {{.ActionJSON}},
    successMessage: successMsg,
    extra: {form: 'contact'},
    validate: function(f) {
      if (!window.SheyanovaForms.val(f, 'name')) return 'Please enter your name.';
      if (!window.SheyanovaForms.val(f, 'email')) return 'Please enter a valid email address.';
      if (!window.SheyanovaForms.val(f, 'message')) return 'Please enter a message.';
      return '';
    }
  });
})();
</script>
</div>
<div class="twocol spacer last"></div>
</div>
`))

func RenderContactForm(in ContactFormInput) string {
	api := strings.TrimRight(strings.TrimSpace(in.APIURL), "/")
	if api == "" {
		api = "https://api.sheyanova.art"
	}
	action := api + "/api/contact"
	id := strings.TrimSpace(in.FormID)
	if id == "" {
		id = "contact_form"
	}
	nameL := in.NameLabel
	if nameL == "" {
		nameL = "Name"
	}
	emailL := in.EmailLabel
	if emailL == "" {
		emailL = "Email"
	}
	msgL := in.MessageLabel
	if msgL == "" {
		msgL = "Message"
	}
	subL := in.SubmitLabel
	if subL == "" {
		subL = "Send Message"
	}
	success := strings.TrimSpace(in.SuccessMessage)
	if success == "" {
		success = "Thank you. Your message has been sent."
	}
	view := map[string]any{
		"Action":           action,
		"FormID":           id,
		"NameLabel":        nameL,
		"EmailLabel":       emailL,
		"MessageLabel":     msgL,
		"SubmitLabel":      subL,
		"TurnstileSiteKey": in.TurnstileSiteKey,
		"FormIDJSON":       jsonJS(id),
		"ActionJSON":       jsonJS(action),
		"SuccessJSON":      jsonJS(success),
	}
	var buf bytes.Buffer
	if err := contactFormTmpl.Execute(&buf, view); err != nil {
		return "<!-- contact form render error -->"
	}
	return buf.String()
}

func jsonJS(v string) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return template.JS(`""`)
	}
	return template.JS(b)
}
