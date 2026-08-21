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
<div class="cf-turnstile" data-sitekey="{{.TurnstileSiteKey}}" data-size="invisible"></div>
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
{{- end }}
</form>
<noscript><p class="contact_form__status is-err">JavaScript is required to send this form.</p></noscript>
<script>
(function(){
  var form = document.getElementById({{.FormIDJSON}});
  if (!form) return;
  var action = {{.ActionJSON}};
  var successMsg = {{.SuccessJSON}};
  var t = form.querySelector('input[name="_t"]');
  var loaded = Date.now();
  if (t) t.value = String(loaded);
  var status = form.querySelector('.contact_form__status');
  function setStatus(kind, text) {
    if (!status) return;
    status.className = 'error_messages contact_form__status' + (kind ? ' is-' + kind : '');
    status.textContent = text || '';
  }
  function val(n) {
    var el = form.querySelector('[name="'+n+'"]');
    return (el && el.value || '').trim();
  }
  function turnstileToken() {
    var ts = form.querySelector('[name="cf-turnstile-response"]');
    return (ts && ts.value) ? ts.value : '';
  }
  function send(token) {
    var payload = {name: val('name'), email: val('email'), message: val('message'), _t: loaded};
    var website = val('website');
    if (website) payload.website = website;
    if (token) payload['cf-turnstile-response'] = token;
    fetch(action, {
      method: 'POST',
      headers: {'Content-Type': 'application/json', 'Accept': 'application/json'},
      body: JSON.stringify(payload)
    }).then(function(res) {
      return res.json().then(function(body) { return {res: res, body: body}; }).catch(function() {
        return {res: res, body: {}};
      });
    }).then(function(out) {
      if (out.res.ok && out.body && out.body.ok) {
        setStatus('ok', (out.body.message || successMsg || 'Thank you. Your message has been sent.'));
        form.reset();
        loaded = Date.now();
        if (t) t.value = String(loaded);
      } else {
        setStatus('err', (out.body && out.body.error) || 'Could not send your message. Please try again later.');
      }
    }).catch(function() {
      setStatus('err', 'Could not send your message. Please try again later.');
    }).then(function() {
      form.classList.remove('is-busy');
    });
  }
  form.addEventListener('submit', function(ev) {
    ev.preventDefault();
    if (form.classList.contains('is-busy')) return;
    if (!val('name')) { setStatus('err', 'Please enter your name.'); return; }
    if (!val('email')) { setStatus('err', 'Please enter a valid email address.'); return; }
    if (!val('message')) { setStatus('err', 'Please enter a message.'); return; }
    form.classList.add('is-busy');
    setStatus('', 'Sending…');
    var token = turnstileToken();
    var widget = form.querySelector('.cf-turnstile');
    if (!token && widget && window.turnstile) {
      var done = false;
      var finish = function(tok) {
        if (done) return;
        done = true;
        send(tok || turnstileToken());
      };
      try {
        window.turnstile.execute(widget, { callback: finish });
      } catch (e) {
        finish(token);
        return;
      }
      setTimeout(function() { finish(turnstileToken()); }, 8000);
      return;
    }
    send(token);
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
