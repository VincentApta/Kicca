// @vitest-environment happy-dom
// SearchDropdown render test: q>=2 fetches (mocked), results clickable,
// task click fires onOpenTask with (taskId, projectId).
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { SearchDropdown } from './search-dropdown'
import { api } from '@/lib/api'

vi.mock('@/lib/api', () => ({
  api: { search: vi.fn() },
}))

const mocked = vi.mocked(api.search)
let root: Root | null = null
let host: HTMLDivElement

beforeEach(() => {
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
})
afterEach(async () => {
  if (root) await act(async () => { root!.unmount() })
  host.remove()
  vi.clearAllMocks()
})

describe('SearchDropdown (global search)', () => {
  it('hidden below 2 chars; shows results and navigates on click', async () => {
    mocked.mockResolvedValue({
      data: {
        tasks: [{ id: 't1', project_id: 'p1', project_key: 'KIC', number: 9, title: 'Fix login', status: 'inbox' }],
        projects: [{ id: 'p2', key: 'ASRCH', name: 'Alpha Search' }],
      },
    })
    const onOpenTask = vi.fn()
    const onOpenProject = vi.fn()

    await act(async () => {
      root!.render(<SearchDropdown q="n" onOpenTask={onOpenTask} onOpenProject={onOpenProject} />)
    })
    expect(host.querySelector('div')).toBeNull() // below threshold
    expect(mocked).not.toHaveBeenCalled()

    await act(async () => {
      root!.render(<SearchDropdown q="needl" onOpenTask={onOpenTask} onOpenProject={onOpenProject} />)
    })
    await act(async () => { await new Promise((r) => setTimeout(r, 300)) }) // debounce
    expect(mocked).toHaveBeenCalledWith('needl')

    // project click first (either click closes the dropdown)
    const projBtn = [...host.querySelectorAll('button')].find((b) => b.textContent?.includes('Alpha Search'))
    expect(projBtn).toBeTruthy()
    await act(async () => { projBtn!.click() })
    expect(onOpenProject).toHaveBeenCalledWith('p2')

    // reopen: rerender with a fresh q (effect refetches on q change)
    await act(async () => {
      root!.render(<SearchDropdown q="fix" onOpenTask={onOpenTask} onOpenProject={onOpenProject} />)
    })
    await act(async () => { await new Promise((r) => setTimeout(r, 300)) })
    const taskBtn = [...host.querySelectorAll('button')].find((b) => b.textContent?.includes('Fix login'))
    expect(taskBtn).toBeTruthy()
    await act(async () => { taskBtn!.click() })
    expect(onOpenTask).toHaveBeenCalledWith('t1', 'p1')
  })
})
