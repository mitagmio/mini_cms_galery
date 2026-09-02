package generate

import (
	"fmt"
	"html/template"
	"strings"

	"sheyanova.art/api/internal/cms"
)

// YandexMetrikaSnippet returns the official counter HTML for id.
// Empty id falls back to cms.DefaultYandexMetrikaID.
func YandexMetrikaSnippet(id string) template.HTML {
	id = strings.TrimSpace(id)
	if id == "" {
		id = cms.DefaultYandexMetrikaID
	}
	// Exact Yandex.Metrika markup (counter ID substituted).
	return template.HTML(fmt.Sprintf(`<!-- Yandex.Metrika counter -->
<script type="text/javascript">
    (function(m,e,t,r,i,k,a){
        m[i]=m[i]||function(){(m[i].a=m[i].a||[]).push(arguments)};
        m[i].l=1*new Date();
        for (var j = 0; j < document.scripts.length; j++) {if (document.scripts[j].src === r) { return; }}
        k=e.createElement(t),a=e.getElementsByTagName(t)[0],k.async=1,k.src=r,a.parentNode.insertBefore(k,a)
    })(window, document,'script','https://mc.yandex.ru/metrika/tag.js', 'ym');

    ym(%s, 'init', {clickmap:true, referrer: document.referrer, url: location.href, accurateTrackBounce:true, trackLinks:true});
</script>
<noscript><div><img src="https://mc.yandex.ru/watch/%s" style="position:absolute; left:-9999px;" alt="" /></div></noscript>
<!-- /Yandex.Metrika counter -->
`, id, id))
}

// metrikaForPage injects the counter only for public publish (empty PathPrefix)
// when the site setting is enabled. Admin preview omits analytics.
func (g *Generator) metrikaForPage(st cms.SiteSettings) template.HTML {
	if !st.YandexMetrikaEnabled {
		return ""
	}
	if strings.TrimSpace(g.Cfg.PathPrefix) != "" {
		return ""
	}
	return YandexMetrikaSnippet(st.YandexMetrikaID)
}
