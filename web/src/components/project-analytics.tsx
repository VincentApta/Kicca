// Project analytics — per-project dashboard (issue #48): stat row, burndown,
// throughput, cycle histogram, type mix, CFD, aging WIP. All math reused from
// lib/dashboard-math; charts reused from dashboard-page (SVG, no library).
// Data is client-aggregated from GET /projects/:id/tasks (trash excluded).
import { useEffect, useState } from 'react'
import {
  AlertTriangleIcon, CheckCircle2Icon, HourglassIcon, ListTodoIcon, TimerIcon,
} from 'lucide-react'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { api, ApiError } from '@/lib/api'
import type { Project, Task } from '@/lib/types'
import { TYPE_LABELS } from '@/lib/labels'
import {
  OPEN_STATUSES, agingWip, avgCycleTime, burndown, cfd, cycleHistogram, throughput,
} from '@/lib/dashboard-math'
import { BarChart, BurnChart, CfdChart, STATUS_DOT, StatCard, TYPE_FILL } from './dashboard-page'

const PER_PAGE = 100

/** Fetch every page of the project's live tasks. */
async function fetchAllTasks(projectId: string): Promise<Task[]> {
  const first = await api.listTasks(projectId, { per_page: PER_PAGE })
  const pages = Math.ceil(first.total / PER_PAGE) - 1
  if (pages <= 0) return first.data
  const rest = await Promise.all(
    Array.from({ length: pages }, (_, i) => api.listTasks(projectId, { per_page: PER_PAGE, page: i + 2 })),
  )
  return [...first.data, ...rest.flatMap((r) => r.data)]
}

export function ProjectAnalyticsPage({
  project,
  onSelectTask,
}: {
  project: Project
  onSelectTask?: (taskId: string) => void
}) {
  const [tasks, setTasks] = useState<Task[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [window, setWindow] = useState<'14' | '30'>('30')

  useEffect(() => {
    let alive = true
    setTasks(null)
    setError(null)
    fetchAllTasks(project.id).then(
      (all) => {
        if (alive) setTasks(all)
      },
      (e) => {
        if (alive) setError(e instanceof ApiError ? e.message : 'Could not load tasks')
      },
    )
    return () => {
      alive = false
    }
  }, [project.id])

  if (error) return <div className="p-8 text-center text-sm text-destructive">{error}</div>
  if (!tasks) {
    return <div className="p-8 text-center text-sm text-muted-foreground/60 animate-pulse">Loading…</div>
  }

  const now = new Date()
  const days = Number(window)
  const open = tasks.filter((t) => (OPEN_STATUSES as readonly string[]).includes(t.status))
  const backlog = tasks.filter((t) => t.status === 'backlog')
  const done = tasks.filter((t) => t.status === 'done')
  const today = now.toISOString().slice(0, 10)
  const overdue = open.filter((t) => t.due_date && t.due_date < today)
  const avgCycle = avgCycleTime(tasks)

  const burn = burndown(tasks, now, days)
  const bars = throughput(tasks, now, 14)
  const cfdSeries = cfd(tasks, now, days)
  const hist = cycleHistogram(tasks)
  const histMax = Math.max(1, ...hist.map((b) => b.count))
  const aging = agingWip(tasks, now, 7).slice(0, 6)
  const typeMix = (['task', 'bug', 'feature', 'chore'] as const).map((k) => ({
    key: k,
    count: tasks.filter((t) => (t.type ?? 'task') === k).length,
  }))
  const typeTotal = Math.max(1, typeMix.reduce((s, m) => s + m.count, 0))

  return (
    <div className="flex-1 overflow-auto p-6 space-y-6">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-heading text-2xl font-semibold text-foreground">
          {project.name} <span className="text-muted-foreground">analytics</span>{' '}
          <span className="font-mono text-base text-muted-foreground">{project.key}</span>
        </h1>
        <div className="ml-auto flex items-center gap-2">
          <Select value={window} onValueChange={(v) => v !== null && setWindow(v as '14' | '30')}>
            <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label="Window">
              <SelectValue placeholder="Window">{window}d window</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="14">14d window</SelectItem>
              <SelectItem value="30">30d window</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* row 1 — stat row */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-5">
        <StatCard icon={<ListTodoIcon className="size-5" strokeWidth={1.5} />} label="Open" value={String(open.length)} />
        <StatCard icon={<HourglassIcon className="size-5" strokeWidth={1.5} />} label="Backlog" value={String(backlog.length)} />
        <StatCard icon={<CheckCircle2Icon className="size-5" strokeWidth={1.5} />} label="Done" value={String(done.length)} />
        <StatCard icon={<AlertTriangleIcon className="size-5 text-status-blocked" strokeWidth={1.5} />} label="Overdue" value={String(overdue.length)} />
        <StatCard icon={<TimerIcon className="size-5" strokeWidth={1.5} />} label="Avg cycle time"
          value={avgCycle == null ? '—' : `${avgCycle.toFixed(1)}d`} />
      </div>

      {/* row 2 — burndown + throughput + CFD */}
      <div className="grid gap-6 lg:grid-cols-3">
        <section className="card-neu" aria-label="Burndown">
          <h2 className="mb-4 text-sm font-medium text-foreground">
            Burndown — {days}d <span className="text-muted-foreground">({burn.mode})</span>
          </h2>
          <BurnChart actual={burn.actual} ideal={burn.ideal} mode={burn.mode} />
        </section>
        <section className="card-neu" aria-label="Throughput">
          <h2 className="mb-4 text-sm font-medium text-foreground">Throughput — 14d</h2>
          <BarChart data={bars} label="Throughput" />
        </section>
        <section className="card-neu" aria-label="Cumulative flow diagram">
          <h2 className="mb-4 text-sm font-medium text-foreground">
            Cumulative flow — {days}d
            <span className="ml-2 inline-flex items-center gap-2 text-[11px] text-muted-foreground">
              <span className="flex items-center gap-1"><span className="inline-block size-2 rounded-full bg-status-in-progress" />created</span>
              <span className="flex items-center gap-1"><span className="inline-block size-2 rounded-full bg-status-done" />done</span>
            </span>
          </h2>
          {tasks.length === 0 ? (
            <p className="text-sm text-muted-foreground">No data yet.</p>
          ) : (
            <CfdChart data={cfdSeries} />
          )}
        </section>
      </div>

      {/* row 3 — cycle histogram + type mix + aging wip */}
      <div className="grid gap-6 lg:grid-cols-3">
        <section className="card-neu" aria-label="Cycle time distribution">
          <h2 className="mb-4 text-sm font-medium text-foreground">Cycle time distribution</h2>
          <ul className="space-y-3">
            {hist.map((b) => (
              <li key={b.label} className="space-y-1.5">
                <div className="flex items-center justify-between text-xs">
                  <span className="text-muted-foreground">{b.label}</span>
                  <span className="font-mono text-muted-foreground">{b.count}</span>
                </div>
                <div className="inset-neu h-2.5 rounded-md p-0.5">
                  <div className="h-full rounded-sm bg-status-in-progress opacity-80"
                    style={{ width: `${(b.count / histMax) * 100}%` }} />
                </div>
              </li>
            ))}
          </ul>
        </section>

        <section className="card-neu" aria-label="Type mix">
          <h2 className="mb-4 text-sm font-medium text-foreground">Type mix</h2>
          <div className="inset-neu flex h-4 overflow-hidden rounded-md p-0.5">
            {typeMix.map((m) =>
              m.count > 0 ? (
                <div key={m.key} className={TYPE_FILL[m.key]} style={{ width: `${(m.count / typeTotal) * 100}%` }} />
              ) : null,
            )}
          </div>
          <ul className="mt-4 flex flex-wrap gap-x-4 gap-y-1.5">
            {typeMix.map((m) => (
              <li key={m.key} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span className={`size-2 rounded-full ${TYPE_FILL[m.key]}`} />
                {TYPE_LABELS[m.key]} · <span className="font-mono">{m.count}</span>
              </li>
            ))}
          </ul>
        </section>

        <section className="card-neu" aria-label="Aging work in progress">
          <h2 className="mb-4 text-sm font-medium text-foreground">Aging WIP — stuck &gt; 7d</h2>
          {aging.length === 0 ? (
            <p className="text-sm text-muted-foreground">Nothing stuck. 🎉</p>
          ) : (
            <ul className="space-y-2">
              {aging.map(({ task: t, ageDays }) => (
                <li key={t.id}>
                  <button
                    onClick={() => onSelectTask?.(t.id)}
                    className="inset-neu flex w-full items-center gap-3 px-3 py-2 text-left transition-shadow hover:shadow-[var(--shadow-inset),var(--shadow-glow)]"
                  >
                    <span className={`size-2 shrink-0 rounded-full ${STATUS_DOT[t.status]}`} />
                    <span className="flex-1 truncate text-sm text-foreground">{t.title}</span>
                    <span className="shrink-0 font-mono text-[10px] text-status-blocked">{ageDays.toFixed(0)}d</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  )
}
