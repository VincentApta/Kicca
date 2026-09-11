// @vitest-environment happy-dom
// Headless render check of the admin users page shape: stubs fetch, mounts
// <UsersPage />, asserts the table skeleton and dumps the DOM.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { AuthProvider } from '@/lib/auth'
import { UsersPage } from './admin-pages'
import type { User } from '@/lib/types'

const users: User[] = [
  { id: 'u1', email: 'root@kica.dev', name: 'Root Admin', global_role: 'admin' },
  { id: 'u2', email: 'ada@kica.dev', name: 'Ada Lovelace', global_role: 'member' },
  { id: 'u3', email: 'grace@kica.dev', name: 'Grace Hopper', global_role: 'member' },
]

function json(body: unknown, status = 200) {
  return { status, ok: status < 400, statusText: 'OK', json: async () => body }
}

let root: Root | null = null

afterEach(() => {
  if (root) act(() => root!.unmount())
  root = null
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('UsersPage render', () => {
  it('renders the admin table (email, name, role, status) with pagination', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: unknown) => {
        const url = String(input)
        if (url.includes('/auth/me')) {
          return json({ user: users[0] })
        }
        if (url.includes('/api/users')) {
          return json({ data: users, page: 1, per_page: 50, total: 3 })
        }
        return json({ error: { code: 'not_found', message: 'nope' } }, 404)
      }),
    )

    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(
        <AuthProvider>
          <UsersPage />
        </AuthProvider>,
      )
    })

    const text = document.body.textContent ?? ''
    expect(document.querySelector('h1')?.textContent).toBe('Users')
    expect(text).toContain('3 users in the workspace')
    for (const h of ['Name', 'Email', 'Role', 'Status']) {
      expect(text).toContain(h)
    }
    expect(text).toContain('ada@kica.dev')
    expect(text).toContain('Ada Lovelace')
    expect(text).toContain('Admin')
    expect(text).toContain('Active')
    expect(text).toContain('Page 1 of 1')
    expect(document.querySelectorAll('table tbody tr')).toHaveLength(3)
    expect(document.querySelector('[aria-label="Edit Ada Lovelace"]')).toBeTruthy()
    expect(document.querySelector('[aria-label="New user"], button')).toBeTruthy()

    // dump-dom for the ticket report
    console.log('[dump-dom users page]\n' + host.innerHTML.replace(/\s+/g, ' ').slice(0, 1500))
  })
})
