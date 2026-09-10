// @vitest-environment happy-dom
// SelectLabel: maps stored value → human label; unknown → fallback → value;
// empty value renders nothing (so SelectValue placeholders still show).
import { describe, expect, it } from 'vitest'
import { act, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { PRIORITY_LABELS, STATUS_LABELS, SelectLabel } from './labels'

function html(el: ReactNode) {
  const div = document.createElement('div')
  act(() => {
    createRoot(div).render(el)
  })
  return div.innerHTML
}

describe('SelectLabel', () => {
  it('maps stored values to labels', () => {
    expect(html(<SelectLabel value="medium" labelMap={PRIORITY_LABELS} />)).toBe('Medium')
    expect(html(<SelectLabel value="in_progress" labelMap={STATUS_LABELS} />)).toBe('In Progress')
  })

  it('falls back for unknown or empty values', () => {
    expect(html(<SelectLabel value="zzz" labelMap={STATUS_LABELS} />)).toBe('zzz')
    expect(html(<SelectLabel value="zzz" labelMap={STATUS_LABELS} fallback="Unknown" />)).toBe('Unknown')
    expect(html(<SelectLabel value="" labelMap={STATUS_LABELS} />)).toBe('')
    expect(html(<SelectLabel value={null} labelMap={STATUS_LABELS} />)).toBe('')
  })
})
