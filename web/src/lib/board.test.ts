import { describe, expect, it } from 'vitest'

import {
  boardTasks,
  byPosition,
  columnOf,
  matchesFilters,
  planMove,
  type TaskFilters,
} from './board'
import type { Priority, Status, Task } from './types'

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
    started_at: null,
    done_at: null,
    assignee: null,
    labels: [],
    due_date: null,
    position: n * 1024,
    created_by: 'u1',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    gh_link: null,
    ...partial,
  }
}

// Column: a(1024) b(2048) c(3072); second column: x(1024)
function fixture() {
  return [
    task({ id: 'a', status: 'review', position: 1024 }),
    task({ id: 'b', status: 'review', position: 2048 }),
    task({ id: 'c', status: 'review', position: 3072 }),
    task({ id: 'x', status: 'inbox', position: 1024 }),
  ]
}

describe('planMove', () => {
  it('drops at end of empty column with both anchors null', () => {
    const { next, move } = planMove(fixture(), 'a', 'done', null)
    expect(move).toEqual({ status: 'done', before_task_id: null, after_task_id: null })
    expect(columnOf(next, 'done').map((t) => t.id)).toEqual(['a'])
    expect(next.find((t) => t.id === 'a')?.status).toBe('done')
  })

  it('inserts between neighbors → before/after anchor the two ids', () => {
    const { next, move } = planMove(fixture(), 'x', 'review', 1)
    expect(move).toEqual({ status: 'review', before_task_id: 'b', after_task_id: 'a' })
    expect(columnOf(next, 'review').map((t) => t.id)).toEqual(['a', 'x', 'b', 'c'])
  })

  it('inserts at head → only before anchor', () => {
    const { move, next } = planMove(fixture(), 'x', 'review', 0)
    expect(move).toEqual({ status: 'review', before_task_id: 'a', after_task_id: null })
    expect(columnOf(next, 'review').map((t) => t.id)).toEqual(['x', 'a', 'b', 'c'])
  })

  it('inserts at tail → only after anchor', () => {
    const { move, next } = planMove(fixture(), 'x', 'review', 3)
    expect(move).toEqual({ status: 'review', before_task_id: null, after_task_id: 'c' })
    expect(columnOf(next, 'review').map((t) => t.id)).toEqual(['a', 'b', 'c', 'x'])
  })

  it('reorders within one column and ignores the dragged task when indexing', () => {
    const { move, next } = planMove(fixture(), 'c', 'review', 0)
    expect(move).toEqual({ status: 'review', before_task_id: 'a', after_task_id: null })
    expect(columnOf(next, 'review').map((t) => t.id)).toEqual(['c', 'a', 'b'])
  })

  it('clamps an out-of-range index', () => {
    const { move } = planMove(fixture(), 'x', 'review', 99)
    expect(move).toEqual({ status: 'review', before_task_id: null, after_task_id: 'c' })
  })

  it('computes a position that lands in the requested slot', () => {
    const { next } = planMove(fixture(), 'x', 'review', 1)
    const col = columnOf(next, 'review')
    expect(col.map((t) => t.id)).toEqual(['a', 'x', 'b', 'c'])
    expect(col[1].position).toBeGreaterThan(col[0].position)
    expect(col[1].position).toBeLessThan(col[2].position)
  })

  it('never mutates the input array or its tasks', () => {
    const before = fixture()
    const snapshot = before.map((t) => ({ ...t }))
    planMove(before, 'x', 'review', 1)
    expect(before).toEqual(snapshot)
  })

  it('returns input unchanged for an unknown id', () => {
    const tasks = fixture()
    expect(planMove(tasks, 'nope', 'done', 0).next).toBe(tasks)
  })
})

const t = (over: Partial<Task>) => task({ id: `t${n}`, ...over })

describe('matchesFilters', () => {
  const base = {
    q: '',
    assignee_id: '',
    priority: '',
    label_id: '',
    status: '',
  } satisfies TaskFilters
  const withF = (f: Partial<TaskFilters>) => ({ ...base, ...f })

  it('passes everything with empty filters', () => {
    expect(matchesFilters(t({ title: 'anything' }), base)).toBe(true)
  })

  it('matches q against title and description, case-insensitive', () => {
    const x = t({ title: 'Fix Login', description: 'session cookie' })
    expect(matchesFilters(x, withF({ q: 'login' }))).toBe(true)
    expect(matchesFilters(x, withF({ q: 'SESSION' }))).toBe(true)
    expect(matchesFilters(x, withF({ q: 'nope' }))).toBe(false)
  })

  it('blank q does not filter', () => {
    expect(matchesFilters(t({ title: 'x' }), withF({ q: '   ' }))).toBe(true)
  })

  it('filters by status, priority, assignee, label', () => {
    const x = t({
      status: 'review',
      priority: 'urgent',
      assignee: { id: 'u9', email: 'e', name: 'n', global_role: 'member' },
      labels: [{ id: 'l1', name: 'bug', color: '#fff' }],
    })
    expect(matchesFilters(x, withF({ status: 'review' }))).toBe(true)
    expect(matchesFilters(x, withF({ status: 'done' }))).toBe(false)
    expect(matchesFilters(x, withF({ priority: 'urgent' }))).toBe(true)
    expect(matchesFilters(x, withF({ priority: 'low' }))).toBe(false)
    expect(matchesFilters(x, withF({ assignee_id: 'u9' }))).toBe(true)
    expect(matchesFilters(x, withF({ assignee_id: 'u1' }))).toBe(false)
    expect(matchesFilters(x, withF({ label_id: 'l1' }))).toBe(true)
    expect(matchesFilters(x, withF({ label_id: 'l2' }))).toBe(false)
  })

  it('combines fields with AND', () => {
    const x = t({ status: 'done', priority: 'low' })
    expect(matchesFilters(x, withF({ status: 'done', priority: 'low' }))).toBe(true)
    expect(matchesFilters(x, withF({ status: 'done', priority: 'high' }))).toBe(false)
  })
})

describe('boardTasks', () => {
  it('excludes trash and respects filters, sorted by position', () => {
    const tasks = [
      t({ id: 'z', status: 'backlog', position: 2048 }),
      t({ id: 'y', status: 'backlog', position: 1024 }),
      t({ id: 'tr', status: 'trash' }),
      t({ id: 'w', status: 'done', priority: 'urgent' }),
    ]
    const out = boardTasks(tasks, { q: '', assignee_id: '', priority: '', label_id: '', status: '' })
    expect(out.map((x) => x.id)).toEqual(['y', 'z', 'w'])
    const urgent = boardTasks(tasks, {
      q: '',
      assignee_id: '',
      priority: 'urgent' as Priority,
      label_id: '',
      status: '' as Status,
    })
    expect(urgent.map((x) => x.id)).toEqual(['w'])
  })

  it('byPosition tiebreaks on number', () => {
    const a = t({ position: 5, number: 9 })
    const b = t({ position: 5, number: 2 })
    expect([a, b].sort(byPosition).map((x) => x.number)).toEqual([2, 9])
  })
})
