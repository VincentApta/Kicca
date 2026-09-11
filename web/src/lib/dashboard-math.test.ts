// Pure-function tests for dashboard math. Buckets: [0,1), [1,3), [3,7), [7,inf).
import { describe, expect, it } from 'vitest'
import {
  agingWip,
  avgCycleTime,
  burndown,
  cfd,
  cycleHistogram,
  dayKey,
  projectStats,
  throughput,
} from './dashboard-math'

const NOW = new Date('2026-09-11T12:00:00Z')
const d = (offsetDays: number, h = 0) => new Date(NOW.getTime() + offsetDays * 86_400_000 + h * 3_600_000).toISOString()

describe('dayKey', () => {
  it('truncates ISO to date', () => {
    expect(dayKey('2026-09-10T23:59:00Z')).toBe('2026-09-10')
    expect(dayKey(new Date('2026-09-10T01:00:00Z'))).toBe('2026-09-10')
  })
})

describe('burndown', () => {
  it('counts remaining open per day with estimates (points mode)', () => {
    const tasks = [
      { created_at: d(-40), done_at: d(-5), estimate: 3 },
      { created_at: d(-40), done_at: null, estimate: 2 },
    ]
    const { actual, ideal, mode } = burndown(tasks, NOW, 30)
    expect(mode).toBe('points')
    expect(actual).toHaveLength(30)
    expect(actual[0]).toEqual({ date: dayKey(d(-29)), remaining: 5 }) // both open 29d ago
    const idx5 = 30 - 1 - 5 // 5 days ago = done day; done day counts as closed
    expect(actual[idx5].remaining).toBe(2)
    expect(actual[29].remaining).toBe(2) // today
    // ideal: straight line start=5 -> 0
    expect(ideal[0].ideal).toBeCloseTo(5)
    expect(ideal[29].ideal).toBeCloseTo(0)
  })

  it('falls back to count mode when no estimates', () => {
    const tasks = [{ created_at: d(-3), done_at: null }]
    const { actual, mode } = burndown(tasks, NOW, 7)
    expect(mode).toBe('count')
    expect(actual[6].remaining).toBe(1)
    expect(actual[2].remaining).toBe(0) // created 3d ago: 4d ago not open
  })

  it('task done same day it was created is not open that day', () => {
    const tasks = [{ created_at: d(-2), done_at: d(-2) }]
    const { actual } = burndown(tasks, NOW, 5)
    expect(actual[4].remaining).toBe(0)
  })
})

describe('throughput', () => {
  it('buckets done tasks per day', () => {
    const tasks = [
      { done_at: d(-1) },
      { done_at: d(-1) },
      { done_at: d(-3) },
      { done_at: null },
    ]
    const days = throughput(tasks, NOW, 14)
    expect(days).toHaveLength(14)
    expect(days[12].count).toBe(2) // yesterday
    expect(days[10].count).toBe(1) // 3 days ago
    expect(days[13].count).toBe(0) // today
    expect(days[11].count).toBe(0)
  })
})

describe('cycle time', () => {
  const tasks = [
    { started_at: d(-10, 1), done_at: d(-9, 1) },  // 1.0d exactly -> 1-3 bucket? [0,1) no -> 1-3
    { started_at: d(-2), done_at: d(-2, 12) },     // 0.5d -> 0-1
    { started_at: d(-8), done_at: d(-3) },         // 5d -> 3-7
    { started_at: d(-20), done_at: d(-1) },        // 19d -> 7+
    { started_at: d(-5), done_at: null },          // not done: ignored
  ]

  it('avg over done tasks only', () => {
    expect(avgCycleTime(tasks)).toBeCloseTo((1 + 0.5 + 5 + 19) / 4)
  })

  it('avg null when none', () => {
    expect(avgCycleTime([{ started_at: d(-5), done_at: null }])).toBeNull()
  })

  it('histogram bucket edges', () => {
    const b = cycleHistogram(tasks)
    expect(b).toEqual([
      { label: '0-1d', count: 1 },
      { label: '1-3d', count: 1 },
      { label: '3-7d', count: 1 },
      { label: '7d+', count: 1 },
    ])
  })
})

describe('agingWip', () => {
  it('started-not-done older than threshold, worst first', () => {
    const tasks = [
      { id: 'a', started_at: d(-8), done_at: null },
      { id: 'b', started_at: d(-20), done_at: null },
      { id: 'c', started_at: d(-2), done_at: null },
      { id: 'e', started_at: d(-10), done_at: d(-9) },
    ]
    const wip = agingWip(tasks, NOW, 7)
    expect(wip.map((w) => w.task.id)).toEqual(['b', 'a'])
    expect(wip[0].ageDays).toBeGreaterThan(19)
  })
})

describe('cfd', () => {
  it('cumulative created and done per day', () => {
    const tasks = [
      { created_at: d(-10), done_at: d(-5) },
      { created_at: d(-3), done_at: null },
      { created_at: d(-1), done_at: d(-1) },
    ]
    const s = cfd(tasks, NOW, 7)
    expect(s).toHaveLength(7)
    // first day (6d ago): only task 1 exists, not done
    expect(s[0]).toEqual({ date: dayKey(d(-6)), created: 1, done: 0 })
    // day -5: task 1 done
    expect(s[1]).toEqual({ date: dayKey(d(-5)), created: 1, done: 1 })
    // day -3: second created
    expect(s[3]).toEqual({ date: dayKey(d(-3)), created: 2, done: 1 })
    // today: all three created, two done
    expect(s[6]).toEqual({ date: dayKey(d(0)), created: 3, done: 2 })
  })
})

describe('projectStats', () => {
  it('rolls up per project: counts, main status, health, progress', () => {
    const tasks = [
      // p1: heavy backlog, fresh activity
      { project_id: 'p1', status: 'backlog', updated_at: d(-1), due_date: null },
      { project_id: 'p1', status: 'backlog', updated_at: d(-2), due_date: null },
      { project_id: 'p1', status: 'in_progress', updated_at: d(-1), due_date: '2026-01-01' },
      { project_id: 'p1', status: 'done', updated_at: d(-3), due_date: null },
      // p2: all done
      { project_id: 'p2', status: 'done', updated_at: d(-1), due_date: null },
      // p3: open but untouched 20d
      { project_id: 'p3', status: 'in_progress', updated_at: d(-20), due_date: null },
      // p4: trash only — open=0 counts as done-health but no open tasks
      { project_id: 'p4', status: 'trash', updated_at: d(-1), due_date: null },
    ]
    const stats = projectStats(tasks, NOW)
    const p1 = stats.find((s) => s.projectId === 'p1')!
    expect(p1.total).toBe(4)
    expect(p1.open).toBe(3)
    expect(p1.backlog).toBe(2)
    expect(p1.done).toBe(1)
    expect(p1.overdue).toBe(1)
    expect(p1.mainStatus).toBe('backlog')
    expect(p1.health).toBe('ongoing')
    expect(p1.progress).toBeCloseTo(0.25)
    expect(p1.lastActivity).toBe(d(-1))

    const p2 = stats.find((s) => s.projectId === 'p2')!
    expect(p2.health).toBe('done')
    expect(p2.mainStatus).toBeNull()

    const p3 = stats.find((s) => s.projectId === 'p3')!
    expect(p3.health).toBe('stalled')

    // sort: p1 (2 backlog) first, then p3 (1 open, stalled), p4/p2 by health
    expect(stats[0].projectId).toBe('p1')
    expect(stats.map((s) => s.projectId)).toContain('p3')
  })
})
