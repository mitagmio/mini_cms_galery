import { newBlock } from './blockTypes'

export const FORM_BLOCK_PALETTE = [
  { type: 'form_step', label: 'Step heading', hint: 'STEP N / section title', defaultData: () => ({ title: 'STEP 1' }) },
  { type: 'form_text', label: 'Short text / URL', hint: 'Imagelink, Color_Reference, …', defaultData: () => ({ name: '', label: 'Short text', placeholder: '', required: false }) },
  { type: 'form_number', label: 'Number', hint: 'Total photos', defaultData: () => ({ name: 'Total', label: 'Total', required: true, min: 1 }) },
  { type: 'form_date', label: 'Date', hint: 'Final delivery', defaultData: () => ({ name: 'Final_delivery', label: 'Final delivery', required: true, placeholder: 'YYYY-MM-DD' }) },
  { type: 'form_textarea', label: 'Textarea', hint: 'Brief', defaultData: () => ({ name: 'Brief', label: 'Brief', placeholder: 'Tell us about the project', rows: 5 }) },
  { type: 'form_select', label: 'Select', hint: 'Format, Profile, Contact, Task', defaultData: () => ({ name: '', label: 'Select', required: true, options: [{ value: 'Option 1', label: 'Option 1' }] }) },
  { type: 'form_radio', label: 'Radio group', hint: 'One choice from a list', defaultData: () => ({ name: '', label: 'Choose one', required: false, options: [{ value: 'a', label: 'Option A' }] }) },
  { type: 'form_checkbox', label: 'Checkbox group', hint: 'colorwork, background, …', defaultData: () => ({ name: 'colorwork', label: 'Color Correction', required: false, options: [{ value: 'New option', label: 'New option' }] }) },
  {
    type: 'form_retouch_level',
    label: 'Retouch-level picker',
    hint: 'Images + News Gothic plaques',
    defaultData: () => ({
      name: 'Retouch_level',
      label: 'Retouch level',
      required: true,
      help: "It will give us the idea of what you prefer, and will not affect the final cost.",
      options: [1, 2, 3, 4].map((n) => ({
        value: String(n),
        label: `Level ${n}`,
        image: `/assets/theme/rates/retouch-level-${n}.webp`,
        plaque: ['LIGHT / RAW', 'NATURAL', 'CLEAN UP', 'FULL TOUCH UP'][n - 1],
      })),
    }),
  },
  { type: 'form_help', label: 'Static help text', hint: 'Hint under a step', defaultData: () => ({ text: '' }) },
  {
    type: 'form_contact_footer',
    label: 'Contact footer',
    hint: 'Name, contact method, email, phone',
    defaultData: () => ({
      legend: 'Contact',
      name_label: 'Name',
      contact_label: 'Preferred contact',
      email_label: 'Email',
      phone_label: 'Phone',
      submit_label: 'Submit project',
      contact_options: [
        { value: 'Phone', label: 'Phone' },
        { value: 'Email', label: 'Email' },
        { value: 'WhatsApp', label: 'WhatsApp' },
      ],
    }),
  },
  {
    type: 'form_honeypot',
    label: 'Honeypot (system)',
    hint: 'Hidden spam trap — not shown on the public form',
    defaultData: () => ({ note: 'Hidden website field (spam trap). Not shown to visitors.' }),
  },
]

export function newFormBlock(type) {
  const def = FORM_BLOCK_PALETTE.find((b) => b.type === type)
  const data = def ? def.defaultData() : {}
  return {
    id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    type,
    data,
  }
}

export function formBlockLabel(type) {
  return FORM_BLOCK_PALETTE.find((b) => b.type === type)?.label || type
}

export function normalizeCanvasBlocks(raw) {
  const list = Array.isArray(raw) ? raw : []
  return list.map((b, i) => ({
    id: b.id || `local-${i}-${Math.random().toString(36).slice(2, 8)}`,
    type: b.type,
    data: b.data && typeof b.data === 'object' ? { ...b.data } : {},
  }))
}

export function serializeCanvasBlocks(blocks) {
  return (blocks || []).map((b) => ({
    type: b.type,
    data: b.data || {},
  }))
}

export function optionList(data) {
  const raw = data?.options
  if (!Array.isArray(raw)) return []
  return raw.map((o) => {
    if (typeof o === 'string') return { value: o, label: o, image: '' }
    return {
      value: o?.value || o?.label || '',
      label: o?.label || o?.value || '',
      image: o?.image || '',
      plaque: o?.plaque || '',
    }
  })
}

export function showIfToString(data) {
  const raw = data?.show_if
  if (!raw) return ''
  if (typeof raw === 'string') return raw
  const field = raw.field || ''
  const vals = Array.isArray(raw.values) ? raw.values : Array.isArray(raw.in) ? raw.in : []
  if (!field) return ''
  return `${field}=${vals.join(',')}`
}

/** Keep page default_blocks compatible with PageEditor newBlock. */
export { newBlock }
