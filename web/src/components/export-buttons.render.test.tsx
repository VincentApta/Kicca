// @vitest-environment happy-dom
// #50 web wiring: the board/list toolbar Export button calls back with the
// board view mounted (the workspace builds the filtered export URL).
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { Topbar } from './topbar'
import { EMPTY_FILTERS } from '@/lib/board'
import type { User } from '@/lib/types'

const me: User = { id: 'u1', email: 'ada@kica.dev', name: 'Ada', global_role: 'member' }

let root: Root | null = null

afterEach(() => {
  if (root) act(() => root!.unmount())
  root = null
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

describe('Topbar export button (#50)', () => {
  it('renders in the board view and fires onExport', async () => {
    const onExport = vi.fn()
    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(
        <Topbar
          project={{ name: 'Kica Core', key: 'KIC' }}
          view="board"
          onView={() => {}}
          filters={EMPTY_FILTERS}
          onFilters={() => {}}
          members={[]}
          labels={[]}
          search=""
          onSearch={() => {}}
          theme="dark"
          onToggleTheme={() => {}}
          me={me}
          onLogout={() => {}}
          onProfile={() => {}}
          onExport={onExport}
        />,
      )
    })
    const btn = document.querySelector('[aria-label="Export the current task list as CSV"]') as HTMLButtonElement
    expect(btn).toBeTruthy()
    await act(async () => {
      btn.click()
    })
    expect(onExport).toHaveBeenCalledTimes(1)
  })

  it('stays hidden outside the board/list/trash views', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(
        <Topbar
          project={{ name: 'Kica Core', key: 'KIC' }}
          view="settings"
          onView={() => {}}
          filters={EMPTY_FILTERS}
          onFilters={() => {}}
          members={[]}
          labels={[]}
          search=""
          onSearch={() => {}}
          theme="dark"
          onToggleTheme={() => {}}
          me={me}
          onLogout={() => {}}
          onProfile={() => {}}
          onExport={() => {}}
        />,
      )
    })
    expect(document.querySelector('[aria-label="Export the current task list as CSV"]')).toBeNull()
  })
})
