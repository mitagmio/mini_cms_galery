package importfront

import "testing"

func TestParseComparisonPairs(t *testing.T) {
	html := `
<div class="comparison_slider__image_wrap">
<img class="comparison_slider__slider_image comparison_slider__slider_image--2" src="/assets/cdn/after.jpg"/>
<img class="comparison_slider__slider_image comparison_slider__slider_image--1" src="/assets/cdn/before.jpg"/>
</div>`
	pairs := parseComparisonPairs(html)
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs", len(pairs))
	}
	if pairs[0].AfterURL != "/assets/cdn/after.jpg" || pairs[0].BeforeURL != "/assets/cdn/before.jpg" {
		t.Fatalf("unexpected pair: %+v", pairs[0])
	}
}

func TestParseGalleryImages(t *testing.T) {
	html := `
<div class="asset image loading">
<img alt="A" class="lazyload" data-src="/assets/cdn/a.jpg" src="/assets/cdn/a.jpg"/>
</div>
<div class="asset image loading">
<img alt="" data-src="/assets/cdn/b.webp" src="/assets/cdn/b.webp"/>
</div>`
	imgs := parseGalleryImages(html)
	if len(imgs) != 2 {
		t.Fatalf("got %d images", len(imgs))
	}
	if imgs[0].URL != "/assets/cdn/a.jpg" || imgs[0].Alt != "A" {
		t.Fatalf("img0 %+v", imgs[0])
	}
	if imgs[1].URL != "/assets/cdn/b.webp" {
		t.Fatalf("img1 %+v", imgs[1])
	}
}
