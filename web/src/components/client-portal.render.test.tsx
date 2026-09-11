// @vitest-environment happy-dom
// Client portal render checks: comments thread on the ticket detail (#43) —
// team comment shows the author name only, composer posts through the client
// endpoint. Stubs fetch for the /api/client/* surface.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { AuthProvider } from '@/lib/auth'
import { ToastProvider } from '@/lib/toast'
import { ClientPortal } from './client-portal'
import type { ClientComment, ClientTicket, User } from '@/lib/types'

const me: User = { id: 'u1', email: 'acme@kica.dev', name: 'Acme Client', global_role: 'client' }

const ticket: ClientTicket = {
  id: 't1',
  project_id: 'p1',
  project_key: 'KIC',
  number: 3,
  title: 'Printer on fire',
  description: 'It is very hot.',
  status: 'inbox',
  created_at: '2026-09-01T10:00:00Z',
  updated_at: '2026-09-02T10:00:00Z',
}

const teamComment: ClientComment = {
  id: 'c1',
  body: 'Which floor is the printer on?',
  created_at: '2026-09-02T11:00:00Z',
  user: { name: 'pm' },
}

const fetchMock = vi.fn()

function json(body: unknown, status = 200) {
  return { status, ok: status < 400, statusText: 'OK', json: async () => body }
}

function stubFetch() {
  fetchMock.mockImplementation(async (input: unknown, init?: { method?: string }) => {
    const url = String(input)
    const method = init?.method ?? 'GET'
    if (url === '/api/auth/me') return json({ user: me })
    if (url === '/api/client/projects') return json({ data: [{ id: 'p1', key: 'KIC', name: 'Kica' }] })
    if (url.includes('/api/client/tickets?')) return json({ data: [ticket], page: 1, per_page: 50, total: 1 })
    if (url.includes('/attachments')) return json({ data: [] })
    if (url.includes('/comments')) {
      if (method === 'POST') {
        return json({ id: 'c2', body: 'Second floor', created_at: '2026-09-02T12:00:00Z', user: { name: me.name } }, 201)
      }
      return json({ data: [teamComment] })
    }
    return json({ error: { code: 'not_found', message: 'nope' } }, 404)
  })
  vi.stubGlobal('fetch', fetchMock)
}

function setNativeValue(el: HTMLTextAreaElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
  setter.call(el, value)
  el.dispatchEvent(new Event('input', { bubbles: true }))
}

let root: Root | null = null

async function mount() {
  const host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  await act(async () => {
    root!.render(
      <AuthProvider>
        <ToastProvider>
          <ClientPortal />
        </ToastProvider>
      </AuthProvider>,
    )
  })
  await act(async () => {}) // projects + tickets effects settle
  return host
}

afterEach(() => {
  if (root) act(() => root!.unmount())
  root = null
  vi.unstubAllGlobals()
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('ClientPortal comments (#43)', () => {
  it('detail thread renders team comments (name only) and posts a reply', async () => {
    stubFetch()
    await mount()

    // open the ticket detail
    await act(async () => {
      ;([...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Printer on fire')) as HTMLButtonElement).click()
    })
    await act(async () => {}) // attachments + comments settle

    const dialog = document.body.textContent ?? ''
    expect(dialog).toContain('Which floor is the printer on?')
    expect(dialog).toContain('pm') // author name, no email rendered
    expect(dialog).not.toContain('member@example.com')

    // compose + send a reply
    const box = document.querySelector('textarea[aria-label="Write a comment"]') as HTMLTextAreaElement
    expect(box).toBeTruthy()
    await act(async () => {
      setNativeValue(box, 'Second floor')
    })
    const send = document.querySelector('[aria-label="Add comment"]') as HTMLButtonElement
    await act(async () => {
      send.click()
    })
    await act(async () => {})

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/client/tickets/t1/comments',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ body: 'Second floor' }) }),
    )
    expect(document.body.textContent).toContain('Second floor')
  })

  it('empty thread shows the hint', async () => {
    fetchMock.mockImplementation(async (input: unknown) => {
      const url = String(input)
      if (url === '/api/auth/me') return json({ user: me })
      if (url === '/api/client/projects') return json({ data: [{ id: 'p1', key: 'KIC', name: 'Kica' }] })
      if (url.includes('/api/client/tickets?')) return json({ data: [ticket], page: 1, per_page: 50, total: 1 })
      if (url.includes('/attachments')) return json({ data: [] })
      return json({ data: [] }) // no comments
    })
    vi.stubGlobal('fetch', fetchMock)
    await mount()
    await act(async () => {
      ;([...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Printer on fire')) as HTMLButtonElement).click()
    })
    await act(async () => {})
    expect(document.body.textContent).toContain('No comments yet')
  })

  it('search box + filter selects drive the tickets fetch (#47)', async () => {
    stubFetch()
    await mount()

    // filter selects exist alongside the search box
    expect(document.querySelector('input[aria-label="Search tickets"]')).toBeTruthy()
    expect(document.querySelector('[aria-label="Filter by project"]')).toBeTruthy()
    expect(document.querySelector('[aria-label="Filter by status"]')).toBeTruthy()

    // typing (debounced 250ms) refetches with ?q=
    const box = document.querySelector('input[aria-label="Search tickets"]') as HTMLInputElement
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
    await act(async () => {
      setter.call(box, 'printer')
      box.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => {
      await new Promise((r) => setTimeout(r, 300)) // debounce
    })
    const asked = fetchMock.mock.calls.map((c) => String(c[0])).filter((u) => u.includes('q='))
    expect(asked).toContain('/api/client/tickets?page=1&per_page=50&q=printer')
  })
})
