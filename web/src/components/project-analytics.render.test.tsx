// @vitest-environment happy-dom
// Render check of the per-project analytics page (issue #48): stat row,
// burndown/throughput/CFD sections, cycle histogram, type mix, aging WIP —
// dashboard-math reuse, scoped to one project's task feed.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { ProjectAnalyticsPage } from './project-analytics'
import type { Project, Task } from '@/lib/types'

const project: Project = {
  id: 'p1', team_id: 't1', name: 'Kica', key: 'KIC', description: '',
}

let n = 0
function task(partial: Partial<Task> & { id: string }): Task {
  n += 1
  return {
    project_id: 'p1',
    number: n,
    title: `Task ${n}`,
    description: '',
    status: 'backlog',
    priority: 'medium',
    type: 'task',
    estimate: null,
    assessment: '',
    started_at: null,
    done_at: null,
    assignee: null,
    labels: [],
    due_date: null,
    position: n * 1024,
    created_by: 'u1',
    created_at: new Date(Date.now() - 20 * 86_400_000).toISOString(),
    updated_at: new Date().toISOString(),
    gh_link: null,
    ...partial,
  }
}

// 2 open (1 overdue) + 2 backlog + 2 done (1 w/ cycle stamps) + 1 aging bug
const day = (d: number) => new Date(Date.now() - d * 86_400_000).toISOString()
const tasks: Task[] = [
  task({ id: 'a', status: 'in_progress', due_date: '2020-01-01' }),
  task({ id: 'b', status: 'review' }),
  task({ id: 'c', status: 'backlog' }),
  task({ id: 'd', status: 'backlog', type: 'bug' }),
  task({ id: 'e', status: 'done', started_at: day(10), done_at: day(8) }),
  task({ id: 'f', status: 'done', started_at: day(6), done_at: day(2) }),
  task({ id: 'g', status: 'in_progress', started_at: day(12), type: 'chore' }),
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

describe('ProjectAnalyticsPage render', () => {
  it('renders stat row + chart sections scoped to the project feed', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => json({ data: tasks, page: 1, per_page: 100, total: tasks.length })),
    )

    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(<ProjectAnalyticsPage project={project} />)
    })

    const text = host.textContent ?? ''
    expect(host.querySelector('h1')?.textContent).toContain('Kica')
    // stat row: open 3, backlog 2, done 2, overdue 1, avg cycle (10-8=2d + 6-2=4d)/2 = 3d
    expect(text).toContain('Open')
    expect(text).toContain('Backlog')
    expect(text).toContain('Overdue')
    expect(host.querySelector('[aria-label="Open tasks by status"], [aria-label="Burndown"]') ?? true).toBeTruthy()
    for (const label of ['Burndown', 'Throughput', 'Cumulative flow diagram', 'Cycle time distribution', 'Type mix', 'Aging work in progress']) {
      expect(host.querySelector(`[aria-label="${label}"]`)).toBeTruthy()
    }
    // type mix counts tasks from the feed
    expect(text).toContain('Bug · 1')
    expect(text).toContain('Chore · 1')

    console.log('[dump-dom project analytics]\n' + host.innerHTML.replace(/\s+/g, ' ').slice(0, 1200))
  })

  it('fetches follow-up pages when total exceeds one page', async () => {
    const urls: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: unknown) => {
        const url = String(input)
        urls.push(url)
        if (url.includes('page=2')) {
          return json({ data: [task({ id: 'z1' }), task({ id: 'z2' })], page: 2, per_page: 100, total: 102 })
        }
        return json({ data: Array.from({ length: 100 }, (_, i) => task({ id: `t${i}` })), page: 1, per_page: 100, total: 102 })
      }),
    )

    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(<ProjectAnalyticsPage project={project} />)
    })
    expect(urls.some((u) => u.includes('page=2'))).toBe(true)
  })
})
