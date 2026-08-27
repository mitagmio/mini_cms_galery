export const TEMPLATES = [
  {
    id: 'ba_content',
    label: 'Before | After',
    description: 'Stacked comparison sliders (home / before-after).',
    starterBlocks: [],
  },
  {
    id: 'panorama_gallery',
    label: 'Gallery',
    description: 'Vertical panorama image strip (editorial / fashion).',
    starterBlocks: [],
  },
  {
    id: 'lookbook_gallery',
    label: 'Lookbook',
    description: 'Masonry photo grid; click opens the panorama overlay.',
    starterBlocks: [],
  },
  {
    id: 'text_content',
    label: 'Text / blank',
    description: 'Rich text, images, and optional contact form.',
    starterBlocks: [
      { type: 'rich_text', data: { html: '<p></p>' } },
    ],
  },
  {
    id: 'about_content',
    label: 'About',
    description: 'Portrait on the left, bio on the right — centered with wide margins.',
    starterBlocks: [],
  },
  {
    id: 'rates_content',
    label: 'Rates',
    description: 'Category banners; each Rate banner chooses a form template.',
    starterBlocks: [],
  },
]

export const SYSTEM_THEME_IDS = TEMPLATES.map((t) => t.id)

export const RATE_FORM_KEYS = ['fashion', 'beauty', 'lookbook', 'editorial', 'product', 'manual']

export const RATE_CAPTIONS = {
  fashion: 'FASHION',
  beauty: 'BEAUTY',
  lookbook: 'LOOKBOOK',
  editorial: 'EDITORIAL',
  product: 'PRODUCT',
  manual: 'MANUAL',
}

export const FORM_TEMPLATES = [
  { id: 'form_fashion', name: 'Fashion', form_key: 'fashion', kind: 'form' },
  { id: 'form_beauty', name: 'Beauty', form_key: 'beauty', kind: 'form' },
  { id: 'form_lookbook', name: 'Lookbook', form_key: 'lookbook', kind: 'form' },
  { id: 'form_editorial', name: 'Editorial', form_key: 'editorial', kind: 'form' },
  { id: 'form_product', name: 'Product', form_key: 'product', kind: 'form' },
  { id: 'form_manual', name: 'Manual', form_key: 'manual', kind: 'form' },
]

export const BANNER_ASPECTS = [
  { id: '3:4', label: '3:4 (portrait)' },
  { id: '2:3', label: '2:3 (tall portrait)' },
  { id: '4:5', label: '4:5' },
  { id: '1:1', label: '1:1 (square)' },
  { id: '4:3', label: '4:3 (landscape)' },
]

export const DEFAULT_BANNER_ASPECT = '3:4'

export function formTemplateId(formKey) {
  return `form_${formKey}`
}

export function formKeyFromTemplateId(id) {
  const raw = String(id || '')
  if (raw.startsWith('form_')) return raw.slice(5)
  return ''
}

export function bannerFormKey(data) {
  const fromId = formKeyFromTemplateId(data?.form_template_id)
  if (RATE_FORM_KEYS.includes(fromId)) return fromId
  if (RATE_FORM_KEYS.includes(data?.form_key)) return data.form_key
  return ''
}

export function formTemplateName(idOrKey, list = FORM_TEMPLATES) {
  const key = RATE_FORM_KEYS.includes(idOrKey) ? idOrKey : formKeyFromTemplateId(idOrKey) || idOrKey
  const row = (list || []).find((t) => t.id === idOrKey || t.form_key === key || t.id === formTemplateId(key))
  return row?.name || row?.label || RATE_CAPTIONS[key] || key || '—'
}

export function rateBannerData(formKey) {
  const key = RATE_FORM_KEYS.includes(formKey) ? formKey : 'fashion'
  return {
    form_template_id: formTemplateId(key),
    form_key: key,
    media_id: null,
    alt: '',
    caption: RATE_CAPTIONS[key] || key.toUpperCase(),
    start_from_label: 'start from',
    price: '',
    currency: '$',
    text_color: '', // empty = Auto (luminance at generate)
    text_backdrop: true, // soft white plate under overlay text
  }
}

/** Overlay text color for rate banners. Empty = Auto (charcoal on light / white on dark). "outline" = manual white + black stroke only. */
export const RATE_TEXT_COLOR_PRESETS = [
  { id: 'auto', label: 'Auto', value: '' },
  { id: 'white', label: 'White', value: '#ffffff' },
  // Matches Beauty placeholder / light-card text (solid near-black, no outline).
  { id: 'charcoal', label: 'Charcoal', value: '#1a1a1a' },
  { id: 'outline', label: 'White + black outline', value: 'outline' },
]

/** Default hex when entering Custom — not White/Charcoal; avoid coral/brand purple. */
export const RATE_TEXT_COLOR_CUSTOM_DEFAULT = '#1a4a7a'

export function normalizeRateTextColor(v) {
  const s = String(v || '')
    .trim()
    .toLowerCase()
  if (!s || s === 'auto') return ''
  if (s === 'outline' || s === 'white_stroke') return 'outline'
  // Drop legacy coral-ish brand defaults if anyone stored them as "preset-like" customs.
  if (s === 'coral' || s === '#c07359' || s === 'c07359') return '#1a4a7a'
  const hex = s.startsWith('#') ? s : `#${s}`
  if (/^#[0-9a-f]{3}$/i.test(hex)) {
    const full = `#${hex[1]}${hex[1]}${hex[2]}${hex[2]}${hex[3]}${hex[3]}`
    // Legacy charcoal (#2f2f2f) → Beauty charcoal
    if (full === '#2f2f2f') return '#1a1a1a'
    return full
  }
  if (/^#[0-9a-f]{6}$/i.test(hex)) {
    if (hex === '#2f2f2f') return '#1a1a1a'
    return hex
  }
  return ''
}

/** Select value for RateBannerTextColorField (keeps Custom open while typing hex). */
export function rateTextColorSelectValue(value) {
  const raw = String(value ?? '').trim()
  const normalized = normalizeRateTextColor(value)
  if (normalized === 'outline') return 'outline'
  if (!normalized) {
    if (raw.startsWith('#') || /^[0-9a-f]{1,6}$/i.test(raw)) return 'custom'
    return 'auto'
  }
  const preset = RATE_TEXT_COLOR_PRESETS.find((p) => p.value === normalized)
  if (preset && preset.id !== 'auto') return preset.id
  if (normalized.startsWith('#')) return 'custom'
  return 'auto'
}

/** Hex to store when user picks Custom… (avoid snapping back to Charcoal/White). */
export function rateTextColorEnterCustom(currentValue) {
  const n = normalizeRateTextColor(currentValue)
  const isPresetHex = RATE_TEXT_COLOR_PRESETS.some((p) => p.value && p.value === n)
  if (n && n.startsWith('#') && !isPresetHex) return n
  return RATE_TEXT_COLOR_CUSTOM_DEFAULT
}

export function rateTextColorPickerHex(value) {
  const n = normalizeRateTextColor(value)
  if (n && n.startsWith('#')) return n
  return RATE_TEXT_COLOR_CUSTOM_DEFAULT
}

export const ALLOWED_BLOCKS_BY_THEME = {
  ba_content: ['comparison_slider'],
  panorama_gallery: ['gallery_image'],
  lookbook_gallery: ['gallery_image'],
  text_content: ['rich_text', 'gallery_image', 'contact_form'],
  about_content: ['gallery_image', 'rich_text'],
  rates_content: ['rich_text', 'rate_banner'],
}

export const BLOCK_PALETTE = [
  {
    type: 'comparison_slider',
    label: 'BA pair',
    hint: 'Before / after comparison',
    defaultData: () => ({
      before_media_id: null,
      after_media_id: null,
      caption: '',
    }),
  },
  {
    type: 'gallery_image',
    label: 'Image',
    hint: 'Portrait (About) or a photo in the article / gallery',
    defaultData: () => ({
      media_id: null,
      alt: '',
      caption: '',
    }),
  },
  {
    type: 'rich_text',
    label: 'Rich text',
    hint: 'HTML / copy block',
    defaultData: () => ({ html: '<p></p>' }),
  },
  {
    type: 'contact_form',
    label: 'Contact form',
    hint: 'Sends to Settings → Contact email',
    defaultData: () => ({
      heading: 'Contact',
      mailto: '',
      success_message: 'Thanks — we will be in touch.',
    }),
  },
  {
    type: 'rate_banner',
    label: 'Rate banner',
    hint: 'Category tile — chooses a form template',
    defaultData: () => rateBannerData('fashion'),
  },
]

export function newBlock(type) {
  const def = BLOCK_PALETTE.find((b) => b.type === type)
  const data = def ? def.defaultData() : {}
  return {
    id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    type,
    data,
  }
}

export const DEFAULT_BLOCKS_BY_THEME = {
  ba_content: () => [
    { type: 'comparison_slider', data: newBlock('comparison_slider').data },
    { type: 'comparison_slider', data: newBlock('comparison_slider').data },
  ],
  panorama_gallery: () => [
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'gallery_image', data: newBlock('gallery_image').data },
  ],
  lookbook_gallery: () => [
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'gallery_image', data: newBlock('gallery_image').data },
  ],
  text_content: () => [{ type: 'rich_text', data: newBlock('rich_text').data }],
  about_content: () => [
    { type: 'gallery_image', data: newBlock('gallery_image').data },
    { type: 'rich_text', data: newBlock('rich_text').data },
  ],
  rates_content: () => [
    {
      type: 'rich_text',
      data: {
        html:
          '<div class="rates-intro-copy"><h2 class="rates-kicker">CHOOSE YOUR CATEGORY</h2>\n<p>Click on a category to submit your retouching request.</p>\n<p>The price is per photo.</p></div>',
      },
    },
    ...RATE_FORM_KEYS.map((key) => ({ type: 'rate_banner', data: rateBannerData(key) })),
  ],
}

export function templateLabel(id) {
  return TEMPLATES.find((t) => t.id === id)?.label || id || '—'
}

export function mediaUrl(item) {
  if (!item) return ''
  return item.url || item.url_path || item.path || ''
}

/** Admin UI preview: compressed thumb when present, else full original. */
export function mediaThumbUrl(item) {
  if (!item) return ''
  return item.thumb_url || mediaUrl(item)
}

export function allowedBlocksForTheme(theme) {
  if (ALLOWED_BLOCKS_BY_THEME[theme]) return ALLOWED_BLOCKS_BY_THEME[theme]
  const t = String(theme || '').toLowerCase()
  if (t === 'blank' || t.includes('blank')) {
    return ALLOWED_BLOCKS_BY_THEME.text_content
  }
  return null
}

export function paletteForAllowed(allowed) {
  if (!allowed || !allowed.length) return BLOCK_PALETTE
  const byType = Object.fromEntries(BLOCK_PALETTE.map((b) => [b.type, b]))
  return allowed.map((t) => byType[t]).filter(Boolean)
}
