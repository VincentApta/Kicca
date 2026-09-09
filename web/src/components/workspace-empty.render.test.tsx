// @vitest-environment happy-dom
// Headless render check of the empty-projects bootstrap state (issue #16):
// admin gets a create-first-project CTA + working admin nav; member gets the
// ask-admin hint. Also walks the admin flow through team + project creation.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { AuthProvider } from '@/lib/auth'
import { ToastProvider } from '@/lib/toast'
import { Workspace } from './workspace'
import type { User } from '@/lib/types'

const admin: User = { id: 'u1', email: 'root@kicca.dev', name: 'Root Admin', global_role: 'admin' }
const member: User = { id: 'u2', email: 'ada@kicca.dev', name: 'Ada Lovelace', global_role: 'member' }
const project = { id: 'p1', team_id: 't1', name: 'Kicca Core', key: 'KIC', description: '' }

const calls: string[] = []

function json(body: unknown, status = 200) {
  return { status, ok: status < 400, statusText: 'OK', json: async () => body }
}

let created = false

function stubFetch(me: User) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: unknown, init?: { method?: string }) => {
      const url = String(input)
      const method = init?.method ?? 'GET'
      calls.push(`${method} ${url}`)
      if (url.includes('/auth/me')) return json({ user: me })
      if (url.includes('/api/teams')) {
        if (method === 'POST') return json({ id: 't1', name: 'Core', slug: 'core' }, 201)
        return json({ data: [] })
      }
      if (url.includes('/api/projects')) {
        if (method === 'POST') {
          created = true
          return json(project, 201)
        }
        if (url.includes('/labels')) return json({ data: [] })
        if (url.includes('/tasks')) return json({ data: [], page: 1, per_page: 100, total: 0 })
        if (url.match(/\/api\/projects\/[^/]+$/)) {
          return json({ ...project, my_role: 'admin', members: [], gh_repo: null })
        }
        return json({ data: created ? [project] : [] }) // list — visible once created
      }
      return json({ error: { code: 'not_found', message: 'nope' } }, 404)
    }),
  )
}

function type(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
  setter.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

function button(label: string) {
  return [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === label)
}

let root: Root | null = null

async function mount(me: User) {
  stubFetch(me)
  const host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  await act(async () => {
    root!.render(
      <AuthProvider>
        <ToastProvider>
          <Workspace />
        </ToastProvider>
      </AuthProvider>,
    )
  })
  await act(async () => {}) // let projects effect settle
  return host
}

afterEach(() => {
  if (root) act(() => root!.unmount())
  root = null
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
  calls.length = 0
  created = false
})

describe('Workspace empty-projects bootstrap', () => {
  it('admin: CTA + sidebar admin nav; nav click opens Teams page', async () => {
    const host = await mount(admin)
    const text = document.body.textContent ?? ''
    expect(text).toContain('No projects yet')
    expect(text).toContain('Create the first team and project')
    expect(button('Create the first project')).toBeTruthy()
    expect(button('Teams')).toBeTruthy()
    expect(button('Users')).toBeTruthy()

    await act(async () => {
      button('Teams')!.click()
    })
    expect(document.querySelector('h1')?.textContent).toBe('Teams')
    expect(button('New team')).toBeTruthy()

    console.log('[dump-dom admin empty]\n' + host.innerHTML.replace(/\s+/g, ' ').slice(0, 1500))
  })

  it('member: ask-admin hint, no CTA, no admin nav', async () => {
    const host = await mount(member)
    const text = document.body.textContent ?? ''
    expect(text).toContain('Ask an admin to add you to a project.')
    expect(button('Create the first project')).toBeFalsy()
    expect(button('Teams')).toBeFalsy()
    expect(button('Users')).toBeFalsy()

    console.log('[dump-dom member empty]\n' + host.innerHTML.replace(/\s+/g, ' ').slice(0, 1200))
  })

  it('admin: first-project dialog creates team + project, lands on the board', async () => {
    await mount(admin)

    await act(async () => {
      button('Create the first project')!.click()
    })
    await act(async () => {}) // teams fetch resolves → no-teams input branch
    const [team, name, key] = [...document.querySelectorAll('input[id^="first-project-"]')]
    expect(team && name && key).toBeTruthy()

    await act(async () => {
      type(team as HTMLInputElement, 'Core')
      type(name as HTMLInputElement, 'Kicca Core')
      type(key as HTMLInputElement, 'KIC')
    })
    await act(async () => {
      button('Create project')!.click()
    })
    await act(async () => {})
    await act(async () => {})

    expect(calls).toContain('POST /api/teams')
    expect(calls).toContain('POST /api/projects')
    expect(button('Create the first project')).toBeFalsy() // dialog closed
    expect(document.body.textContent).toContain('Kicca Core') // topbar project name
  })
})
