// @vitest-environment happy-dom
// Render check of the activity timeline (#44): newest-first order, creation
// event labeled 'created', actor shown as name only (even when the team
// payload carries email/role).
import { afterEach, describe, expect, it } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { ActivityTimeline } from './activity-timeline'
import type { TaskActivityEvent } from '@/lib/types'

const events: TaskActivityEvent[] = [
  // newest first — the API orders DESC
  {
    id: 'e3',
    from_status: 'done',
    to_status: 'backlog',
    actor: { id: 'u2', email: 'pa@kica.dev', name: 'PA', global_role: 'member' },
    at: new Date(Date.now() - 5 * 60_000).toISOString(),
  },
  {
    id: 'e2',
    from_status: 'in_progress',
    to_status: 'done',
    actor: { id: 'u2', email: 'pa@kica.dev', name: 'PA', global_role: 'member' },
    at: new Date(Date.now() - 3 * 3600_000).toISOString(),
  },
  {
    id: 'e1',
    from_status: null,
    to_status: 'backlog',
    actor: { name: 'Client' },
    at: new Date(Date.now() - 2 * 24 * 3600_000).toISOString(),
  },
]

let root: Root | null = null

afterEach(() => {
  if (root) act(() => root!.unmount())
  root = null
  document.body.innerHTML = ''
})

describe('ActivityTimeline', () => {
  it('renders events newest-first with actor name, change and relative time', () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    act(() => {
      root!.render(<ActivityTimeline events={events} />)
    })

    const rows = host.querySelectorAll('li')
    expect(rows).toHaveLength(3)
    const text = host.textContent ?? ''
    // newest transition leads
    expect(rows[0].textContent).toContain('PA')
    expect(rows[0].textContent).toContain('Done → Backlog')
    expect(rows[0].textContent).toContain('5m ago')
    // relative buckets
    expect(rows[1].textContent).toContain('3h ago')
    expect(rows[2].textContent).toContain('2d ago')
    // creation event (from_status null) reads as 'created'
    expect(rows[2].textContent).toContain('Client')
    expect(rows[2].textContent).toContain('created')
    // actor email never renders even when the team payload carries it
    expect(text).not.toContain('pa@kica.dev')
  })
})
