package generate

import (
	"fmt"
	"html/template"
	"strings"

	"sheyanova.art/api/internal/cms"
)

// GTMHeadSnippet returns the official GTM head script for container id.
func GTMHeadSnippet(id string) template.HTML {
	id = strings.TrimSpace(id)
	if id == "" {
		id = cms.DefaultGTMContainerID
	}
	return template.HTML(fmt.Sprintf(`<!-- Google Tag Manager -->
<script>(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','%s');</script>
<!-- End Google Tag Manager -->
`, id))
}

// GTMBodySnippet returns the official GTM noscript iframe for container id.
func GTMBodySnippet(id string) template.HTML {
	id = strings.TrimSpace(id)
	if id == "" {
		id = cms.DefaultGTMContainerID
	}
	return template.HTML(fmt.Sprintf(`<!-- Google Tag Manager (noscript) -->
<noscript><iframe src="https://www.googletagmanager.com/ns.html?id=%s"
height="0" width="0" style="display:none;visibility:hidden"></iframe></noscript>
<!-- End Google Tag Manager (noscript) -->
`, id))
}

// gtmForPage injects GTM only for public publish (empty PathPrefix) when enabled.
func (g *Generator) gtmForPage(st cms.SiteSettings) (head, body template.HTML) {
	if !st.GTMEnabled {
		return "", ""
	}
	if strings.TrimSpace(g.Cfg.PathPrefix) != "" {
		return "", ""
	}
	id := st.GTMContainerID
	return GTMHeadSnippet(id), GTMBodySnippet(id)
}
