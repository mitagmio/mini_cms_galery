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
    description: 'Rich text and optional contact form.',
    starterBlocks: [
      { type: 'rich_text', data: { html: '<p></p>' } },
    ],
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
  }
}

export const ALLOWED_BLOCKS_BY_THEME = {
  ba_content: ['comparison_slider'],
  panorama_gallery: ['gallery_image'],
  lookbook_gallery: ['gallery_image'],
  text_content: ['rich_text', 'contact_form'],
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
    label: 'Gallery image',
    hint: 'One image in a panorama strip or lookbook',
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

export function allowedBlocksForTheme(theme) {
  return ALLOWED_BLOCKS_BY_THEME[theme] || null
}
