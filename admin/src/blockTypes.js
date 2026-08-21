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
    id: 'text_content',
    label: 'Text / blank',
    description: 'Rich text and optional contact form.',
    starterBlocks: [
      { type: 'rich_text', data: { html: '<p></p>' } },
    ],
  },
]

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
    hint: 'One image in a panorama strip',
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

export function templateLabel(id) {
  return TEMPLATES.find((t) => t.id === id)?.label || id || '—'
}

export function mediaUrl(item) {
  if (!item) return ''
  return item.url || item.url_path || item.path || ''
}
