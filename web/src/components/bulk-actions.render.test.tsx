// @vitest-environment happy-dom
// Render check of the multi-select bulk toolbar (issue #46): count, move +
// assign pickers, delete + clear actions. Mounts <Topbar /> in select mode.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { Topbar } from './topbar'
import { EMPTY_FILTERS } from '@/lib/board'
import type { Label, ProjectMember, User } from '@/lib/types'

const me: User = { id: 'u1', email: 'ada@kica.dev', name: 'Ada Lovelace', global_role: 'member' }
const members: ProjectMember[] = [
  { user_id: 'u1', role: 'member', name: 'Ada Lovelace', email: 'ada@kica.dev' },
  { user_id: 'u2', role: 'project_admin', name: 'Grace Hopper', email: 'grace@kica.dev' },
]
const labels: Label[] = []

let root: Root | null = null

afterEach(() => {
  if (root) act(() => root!.unmount())
  root = null
  document.body.innerHTML = ''
})

function mount(props: Partial<Parameters<typeof Topbar>[0]> = {}) {
  const host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  act(() => {
    root!.render(
      <Topbar
        project={{ name: 'Kica', key: 'KIC' }}
        view="board"
        onView={() => {}}
        filters={EMPTY_FILTERS}
        onFilters={() => {}}
        members={members}
        labels={labels}
        search=""
        onSearch={() => {}}
        theme="dark"
        onToggleTheme={() => {}}
        me={me}
        onLogout={() => {}}
        onProfile={() => {}}
        onExport={() => {}}
        onOpenNotification={() => {}}
        {...props}
      />,
    )
  })
  return host
}

describe('bulk actions toolbar', () => {
  it('shows selected count, pickers and actions; delete + clear fire handlers', () => {
    const onBulkDelete = vi.fn()
    const onExitSelect = vi.fn()
    const onToggleSelectMode = vi.fn()
    const host = mount({
      selectMode: true,
      selectedCount: 3,
      onBulkDelete,
      onExitSelect,
      onToggleSelectMode,
    })
    const text = host.textContent ?? ''
    expect(text).toContain('3 selected')
    expect(host.querySelector('[aria-label="Move to column"]')).toBeTruthy()
    expect(host.querySelector('[aria-label="Assign to member"]')).toBeTruthy()
    // filters hidden in select mode
    expect(host.querySelector('[aria-label="Assignee"]')).toBeNull()

    act(() => {
      host.querySelector<HTMLButtonElement>('[aria-label="Delete selected"]')!.click()
    })
    expect(onBulkDelete).toHaveBeenCalledTimes(1)
    act(() => {
      host.querySelector<HTMLButtonElement>('[aria-label="Deselect all"]')!.click()
    })
    expect(onExitSelect).toHaveBeenCalledTimes(1)
  })

  it('offers a Select toggle outside select mode', () => {
    const onToggleSelectMode = vi.fn()
    const host = mount({ onToggleSelectMode })
    expect(host.textContent).toContain('Select')
    const btn = [...host.querySelectorAll('button')].find((b) => b.textContent?.trim() === 'Select')!
    act(() => {
      btn.click()
    })
    expect(onToggleSelectMode).toHaveBeenCalledTimes(1)
  })
})
