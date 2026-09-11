// Pure analytics math for the dashboard — no React, no DOM. Tested in
// dashboard-math.test.ts. All day arithmetic in UTC; days as 2006-01-02.

export type BurnPoint = { date: string; remaining: number }
export type IdealPoint = { date: string; ideal: number }
export type DayCount = { date: string; count: number }
export type Bucket = { label: string; count: number }

const DAY_MS = 86_400_000

export const OPEN_STATUSES = ['inbox', 'backlog', 'in_progress', 'review', 'blocked'] as const
export type OpenStatus = (typeof OPEN_STATUSES)[number]

export function dayKey(d: Date | string): string {
  return typeof d === 'string' ? d.slice(0, 10) : d.toISOString().slice(0, 10)
}

export function daysAgoKey(now: Date, n: number): string {
  return dayKey(new Date(now.getTime() - n * DAY_MS))
}

/** Open = created but not done at end of that day (never started or in flight). */
export function taskWasOpenOn(t: { created_at: string; done_at?: string | null }, day: string): boolean {
  if (dayKey(t.created_at) > day) return false
  if (!t.done_at) return true
  return dayKey(t.done_at) > day
}

/** Burndown series: remaining open estimate (or count fallback) per day. */
export function burndown(
  tasks: { created_at: string; done_at?: string | null; estimate?: number | null }[],
  now: Date,
  days = 30,
): { actual: BurnPoint[]; ideal: IdealPoint[]; mode: 'points' | 'count' } {
  const mode: 'points' | 'count' = tasks.some((t) => t.estimate != null) ? 'points' : 'count'
  const weight = (t: { estimate?: number | null }) =>
    mode === 'points' ? (t.estimate ?? 1) : 1

  const actual: BurnPoint[] = []
  for (let i = days - 1; i >= 0; i--) {
    const day = daysAgoKey(now, i)
    const remaining = tasks
      .filter((t) => taskWasOpenOn(t, day))
      .reduce((sum, t) => sum + weight(t), 0)
    actual.push({ date: day, remaining })
  }

  // ideal: straight line from day-0 remaining to zero over the window
  const start = actual[0]?.remaining ?? 0
  const ideal: IdealPoint[] = actual.map((p, i) => ({
    date: p.date,
    ideal: Math.max(0, start * (1 - i / Math.max(1, actual.length - 1))),
  }))
  return { actual, ideal, mode }
}

/** Throughput: tasks done per day, last n days. */
export function throughput(tasks: { done_at?: string | null }[], now: Date, days = 14): DayCount[] {
  const out: DayCount[] = []
  for (let i = days - 1; i >= 0; i--) {
    const day = daysAgoKey(now, i)
    out.push({
      date: day,
      count: tasks.filter((t) => t.done_at && dayKey(t.done_at) === day).length,
    })
  }
  return out
}

/** Cycle time days for done tasks with both stamps. */
export function cycleTimes(tasks: { started_at?: string | null; done_at?: string | null }[]): number[] {
  return tasks
    .filter((t) => t.started_at && t.done_at)
    .map((t) => (new Date(t.done_at!).getTime() - new Date(t.started_at!).getTime()) / DAY_MS)
}

export function avgCycleTime(tasks: { started_at?: string | null; done_at?: string | null }[]): number | null {
  const cs = cycleTimes(tasks)
  if (cs.length === 0) return null
  return cs.reduce((a, b) => a + b, 0) / cs.length
}

/** Histogram buckets: [0,1), [1,3), [3,7), [7,inf) days. */
export function cycleHistogram(tasks: { started_at?: string | null; done_at?: string | null }[]): Bucket[] {
  const labels = ['0-1d', '1-3d', '3-7d', '7d+']
  const counts = [0, 0, 0, 0]
  for (const c of cycleTimes(tasks)) {
    if (c < 1) counts[0]++
    else if (c < 3) counts[1]++
    else if (c < 7) counts[2]++
    else counts[3]++
  }
  return labels.map((label, i) => ({ label, count: counts[i] }))
}

/** Aging WIP: started but not done, older than maxAgeDays, worst first. */
export function agingWip<T extends { started_at?: string | null; done_at?: string | null }>(
  tasks: T[],
  now: Date,
  maxAgeDays = 7,
): { task: T; ageDays: number }[] {
  return tasks
    .filter((t) => t.started_at && !t.done_at)
    .map((t) => ({
      task: t,
      ageDays: (now.getTime() - new Date(t.started_at!).getTime()) / DAY_MS,
    }))
    .filter((a) => a.ageDays > maxAgeDays)
    .sort((a, b) => b.ageDays - a.ageDays)
}

/** CFD series: cumulative created and cumulative done at end of each day.
 * Tasks created before the window still count (seeded as initial totals) so
 * the created line never starts at zero for an active board. */
export function cfd(
  tasks: { created_at: string; done_at?: string | null }[],
  now: Date,
  days = 30,
): { date: string; created: number; done: number }[] {
  const first = daysAgoKey(now, days - 1)
  let created = 0
  let done = 0
  for (const t of tasks) {
    if (dayKey(t.created_at) < first) {
      created++
      if (t.done_at && dayKey(t.done_at) < first) done++
    }
  }
  const out: { date: string; created: number; done: number }[] = []
  for (let i = days - 1; i >= 0; i--) {
    const day = daysAgoKey(now, i)
    created += tasks.filter((t) => dayKey(t.created_at) === day).length
    done += tasks.filter((t) => t.done_at && dayKey(t.done_at) === day).length
    out.push({ date: day, created, done })
  }
  return out
}

export type ProjectHealth = 'ongoing' | 'stalled' | 'done'
export type ProjectStat = {
  projectId: string
  total: number
  open: number
  backlog: number
  done: number
  overdue: number
  lastActivity: string | null // max(updated_at) over open tasks
  mainStatus: OpenStatus | 'done' | null // most common status among non-done
  health: ProjectHealth
  progress: number // done/total, 0..1
}

/** Per-project rollup. stalled = open work with no update in 14d. Sorted:
 * backlog (most first) -> overdue -> ongoing/done. */
export function projectStats(
  tasks: {
    project_id: string
    status: string
    updated_at: string
    due_date?: string | null
  }[],
  now: Date,
): ProjectStat[] {
  const byProject = new Map<string, ProjectStat>()
  const today = dayKey(now)
  for (const t of tasks) {
    let s = byProject.get(t.project_id)
    if (!s) {
      s = {
        projectId: t.project_id, total: 0, open: 0, backlog: 0, done: 0,
        overdue: 0, lastActivity: null, mainStatus: null, health: 'ongoing', progress: 0,
      }
      byProject.set(t.project_id, s)
    }
    s.total++
    if (t.status === 'done') {
      s.done++
    } else if ((OPEN_STATUSES as readonly string[]).includes(t.status)) {
      s.open++
      if (t.status === 'backlog') s.backlog++
      if (t.due_date && t.due_date < today) s.overdue++
      if (!s.lastActivity || t.updated_at > s.lastActivity) s.lastActivity = t.updated_at
    }
  }
  const statusCount = new Map<string, Map<string, number>>()
  for (const t of tasks) {
    if (t.status === 'done') continue
    if (!(OPEN_STATUSES as readonly string[]).includes(t.status)) continue // trash
    let m = statusCount.get(t.project_id)
    if (!m) { m = new Map(); statusCount.set(t.project_id, m) }
    m.set(t.status, (m.get(t.status) ?? 0) + 1)
  }

  const out = [...byProject.values()]
  for (const s of out) {
    const m = statusCount.get(s.projectId)
    if (m && m.size > 0) {
      s.mainStatus = [...m.entries()].sort((a, b) => b[1] - a[1])[0][0] as OpenStatus
    }
    s.progress = s.total > 0 ? s.done / s.total : 0
    const staleDays = s.lastActivity
      ? (now.getTime() - new Date(s.lastActivity).getTime()) / DAY_MS
      : Infinity
    s.health = s.open === 0 ? 'done' : staleDays > 14 ? 'stalled' : 'ongoing'
  }
  return out.sort((a, b) =>
    b.backlog - a.backlog || b.overdue - a.overdue || a.health.localeCompare(b.health),
  )
}
