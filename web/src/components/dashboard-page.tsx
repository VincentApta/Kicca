// Dashboard — team overview with analytics: burndown, throughput, cycle
// time, aging WIP, workload, type mix. Pure math lives in lib/dashboard-math.
import { useEffect, useMemo, useState } from 'react'
import {
  AlertTriangleIcon, CheckCircle2Icon, ListTodoIcon, TimerIcon,
} from 'lucide-react'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { api, ApiError } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import type { Project, Task, User } from '@/lib/types'
import { STATUS_LABELS, TYPE_LABELS } from '@/lib/labels'
import {
  OPEN_STATUSES, agingWip, avgCycleTime, burndown, cfd, cycleHistogram,
  projectStats, throughput,
} from '@/lib/dashboard-math'

export const STATUS_DOT: Record<string, string> = {
  inbox: 'bg-status-inbox', backlog: 'bg-status-backlog',
  in_progress: 'bg-status-in-progress', review: 'bg-status-review',
  done: 'bg-status-done', blocked: 'bg-status-blocked',
}
export const TYPE_FILL: Record<string, string> = {
  task: 'bg-status-in-progress', bug: 'bg-priority-urgent',
  feature: 'bg-status-review', chore: 'bg-status-backlog',
}

export function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="card-neu flex items-center gap-4">
      <div className="inset-neu flex size-10 shrink-0 items-center justify-center text-primary">{icon}</div>
      <div>
        <p className="font-heading text-2xl font-semibold text-foreground leading-none">{value}</p>
        <p className="mt-1 text-xs text-muted-foreground">{label}</p>
      </div>
    </div>
  )
}

function initials(name: string) {
  return name.split(/\s+/).map((w) => w[0]).filter(Boolean).slice(0, 2).join('').toUpperCase()
}

/** Compact SVG line chart: actual solid + ideal dashed. */
export function BurnChart({ actual, ideal, mode }: {
  actual: { date: string; remaining: number }[]
  ideal: { date: string; ideal: number }[]
  mode: 'points' | 'count'
}) {
  const W = 560, H = 160, PAD = 8
  const max = Math.max(1, ...actual.map((p) => p.remaining))
  const x = (i: number) => PAD + (i / Math.max(1, actual.length - 1)) * (W - 2 * PAD)
  const y = (v: number) => H - PAD - (v / max) * (H - 2 * PAD)
  const line = (pts: number[]) => pts.map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ')
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="inset-neu w-full" role="img" aria-label={`Burndown (${mode})`}>
      <path d={line(ideal.map((p) => p.ideal))} fill="none" stroke="currentColor" strokeWidth="1.5"
        strokeDasharray="4 4" className="text-muted-foreground/50" />
      <path d={line(actual.map((p) => p.remaining))} fill="none" stroke="currentColor" strokeWidth="2"
        className="text-primary" />
    </svg>
  )
}

/** Compact SVG bar chart. */
export function BarChart({ data, label }: { data: { date: string; count: number }[]; label: string }) {
  const W = 560, H = 160, PAD = 8
  const max = Math.max(1, ...data.map((p) => p.count))
  const bw = (W - 2 * PAD) / data.length
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="inset-neu w-full" role="img" aria-label={label}>
      {data.map((p, i) => {
        const h = (p.count / max) * (H - 2 * PAD)
        return (
          <rect key={p.date} x={PAD + i * bw + 1} y={H - PAD - h} width={Math.max(1, bw - 2)}
            height={h} rx="2" className="fill-status-done opacity-80" />
        )
      })}
    </svg>
  )
}

/** CFD: cumulative created (upper) vs cumulative done (lower). */
export function CfdChart({ data }: { data: { date: string; created: number; done: number }[] }) {
  const W = 560, H = 160, PAD = 8
  const max = Math.max(1, ...data.map((p) => p.created))
  const x = (i: number) => PAD + (i / Math.max(1, data.length - 1)) * (W - 2 * PAD)
  const y = (v: number) => H - PAD - (v / max) * (H - 2 * PAD)
  const line = (vals: number[]) =>
    vals.map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ')
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="inset-neu w-full" role="img" aria-label="Cumulative flow">
      <path d={line(data.map((p) => p.created))} fill="none" strokeWidth="2"
        className="stroke-status-in-progress" />
      <path d={line(data.map((p) => p.done))} fill="none" strokeWidth="2"
        className="stroke-status-done" />
    </svg>
  )
}

export function DashboardPage({ onSelectTask }: { onSelectTask: (projectId: string, taskId: string) => void }) {
  const { state } = useAuth()
  const [projects, setProjects] = useState<Project[] | null>(null)
  const [teams, setTeams] = useState<{ id: string; name: string }[]>([])
  const [tasks, setTasks] = useState<Task[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [memberId, setMemberId] = useState<string>('all')
  const [projectId, setProjectId] = useState<string>('all')
  const [teamId, setTeamId] = useState<string>('all')
  const [teamUserIds, setTeamUserIds] = useState<string[]>([])

  const me = state.phase === 'authenticated' ? state.user : null

  useEffect(() => {
    if (state.phase !== 'authenticated') return
    api.listProjects().then((r) => setProjects(r.data)).catch(() => setError('Could not load projects'))
    api.listTeams()
      .then((r) => setTeams(r.data.map((t) => ({ id: t.id, name: t.name }))))
      .catch(() => { /* teams optional */ })
  }, [state])

  // fetch team member user ids when a team is selected
  useEffect(() => {
    if (state.phase !== 'authenticated' || teamId === 'all') {
      setTeamUserIds([])
      return
    }
    api.listTeamMembers(teamId).then((r) => setTeamUserIds(r.members.map((m) => m.user_id)))
      .catch(() => setTeamUserIds([]))
  }, [state, teamId])

  useEffect(() => {
    if (state.phase !== 'authenticated') return
    setTasks(null)
    api
      .myFetchMyTasks(memberId === 'all' ? undefined : memberId, 1, 100, teamUserIds.length ? teamUserIds : undefined)
      .then((r) => setTasks(r.data))
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Could not load tasks'))
  }, [state, memberId, teamUserIds])

  const allTasks = tasks ?? []
  const shown = useMemo(
    () => (projectId === 'all' ? allTasks : allTasks.filter((t) => t.project_id === projectId)),
    [allTasks, projectId],
  )

  const members = useMemo(() => {
    const byId = new Map<string, User>()
    if (me) byId.set(me.id, me)
    for (const t of allTasks) if (t.assignee) byId.set(t.assignee.id, t.assignee)
    return [...byId.values()].sort((a, b) => a.name.localeCompare(b.name))
  }, [allTasks, me])

  if (error) return <div className="p-8 text-center text-sm text-destructive">{error}</div>
  if (!projects || !tasks) {
    return <div className="p-8 text-center text-sm text-muted-foreground/60 animate-pulse">Loading…</div>
  }

  const now = new Date()
  const open = shown.filter((t) => (OPEN_STATUSES as readonly string[]).includes(t.status))
  const today = now.toISOString().slice(0, 10)
  const overdue = open.filter((t) => t.due_date && t.due_date < today)
  const weekAgo = new Date(now.getTime() - 7 * 86_400_000).toISOString()
  const doneThisWeek = shown.filter((t) => t.done_at && t.done_at >= weekAgo).length
  const avgCycle = avgCycleTime(shown)

  const burn = burndown(shown, now, 30)
  const bars = throughput(shown, now, 14)
  const cfdSeries = cfd(shown, now, 30)
  const hist = cycleHistogram(shown)
  const histMax = Math.max(1, ...hist.map((b) => b.count))
  const aging = agingWip(shown, now, 7).slice(0, 6)
  const attention = [
    ...overdue,
    ...open.filter((t) => t.status === 'blocked'),
  ].filter((t, i, a) => a.indexOf(t) === i).slice(0, 8)

  const typeMix = (['task', 'bug', 'feature', 'chore'] as const).map((k) => ({
    key: k,
    count: shown.filter((t) => (t.type ?? 'task') === k).length,
  }))
  const typeTotal = Math.max(1, typeMix.reduce((s, m) => s + m.count, 0))

  const byStatus = OPEN_STATUSES.map((s) => ({ status: s, tasks: open.filter((t) => t.status === s) }))
    .filter((b) => b.tasks.length > 0)
  const maxCount = Math.max(1, ...byStatus.map((b) => b.tasks.length))

  // project-wise rollup over ALL tasks (member filter applies, project filter
  // would make it a single row, so show it whenever tasks exist)
  const projStats = projectStats(allTasks, now)
  const projById = new Map(projects.map((p) => [p.id, p]))
  const HEALTH_BADGE: Record<string, string> = {
    ongoing: 'text-status-in-progress', stalled: 'text-status-blocked', done: 'text-status-done',
  }
  const HEALTH_LABEL: Record<string, string> = {
    ongoing: 'Ongoing', stalled: 'Stalled', done: 'Done',
  }

  return (
    <div className="flex-1 overflow-auto p-6 space-y-6">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Dashboard</h1>
        <div className="ml-auto flex items-center gap-2">
          {teams.length > 0 && (
            <Select value={teamId} onValueChange={(v) => v !== null && setTeamId(v)}>
              <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label="Team">
                <SelectValue placeholder="Team">
                  {teamId === 'all' ? 'All teams' : teams.find((t) => t.id === teamId)?.name ?? 'All teams'}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All teams</SelectItem>
                {teams.map((t) => <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>)}
              </SelectContent>
            </Select>
          )}
          <Select value={memberId} onValueChange={(v) => v !== null && setMemberId(v)}>
            <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label="Member">
              <SelectValue placeholder="Member">
                {memberId === 'all' ? 'All members' : members.find((m) => m.id === memberId)?.name ?? 'All members'}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All members</SelectItem>
              {members.map((m) => <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>)}
            </SelectContent>
          </Select>
          <Select value={projectId} onValueChange={(v) => v !== null && setProjectId(v)}>
            <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label="Project">
              <SelectValue placeholder="Project">
                {projectId === 'all' ? 'All projects' : projects.find((p) => p.id === projectId)?.name ?? 'All projects'}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All projects</SelectItem>
              {projects.map((p) => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* row 1 — stats */}
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard icon={<ListTodoIcon className="size-5" strokeWidth={1.5} />} label="Open tasks" value={String(open.length)} />
        <StatCard icon={<CheckCircle2Icon className="size-5" strokeWidth={1.5} />} label="Done this week" value={String(doneThisWeek)} />
        <StatCard icon={<TimerIcon className="size-5" strokeWidth={1.5} />} label="Avg cycle time"
          value={avgCycle == null ? '—' : `${avgCycle.toFixed(1)}d`} />
        <StatCard icon={<AlertTriangleIcon className="size-5 text-status-blocked" strokeWidth={1.5} />} label="Overdue" value={String(overdue.length)} />
      </div>

      {/* row 2 — project-wise overview */}
      <section className="card-neu" aria-label="Projects overview">
        <h2 className="mb-4 text-sm font-medium text-foreground">Projects</h2>
        {projStats.length === 0 ? (
          <p className="text-sm text-muted-foreground">No projects with tasks yet.</p>
        ) : (
          <ul className="space-y-4">
            {projStats.map((s) => {
              const p = projById.get(s.projectId)
              return (
                <li key={s.projectId}>
                  <button
                    onClick={() => setProjectId(s.projectId)}
                    className="inset-neu flex w-full flex-col gap-2 px-4 py-3 text-left transition-shadow hover:shadow-[var(--shadow-inset),var(--shadow-glow)]"
                  >
                    <div className="flex items-center gap-3">
                      <span className="shrink-0 rounded bg-secondary px-1.5 py-0.5 font-mono text-[10px] uppercase text-muted-foreground">
                        {p?.key ?? '?'}
                      </span>
                      <span className="flex-1 truncate text-sm font-medium text-foreground">
                        {p?.name ?? 'Unknown project'}
                      </span>
                      <span className={`shrink-0 text-[10px] font-medium tracking-wide uppercase ${HEALTH_BADGE[s.health]}`}>
                        {HEALTH_LABEL[s.health]}
                      </span>
                    </div>
                    <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
                      {s.mainStatus ? (
                        <span className="flex items-center gap-1.5">
                          <span className={`size-2 rounded-full ${STATUS_DOT[s.mainStatus]}`} />
                          {STATUS_LABELS[s.mainStatus]}
                        </span>
                      ) : (
                        <span>—</span>
                      )}
                      <span className="font-mono">{s.open} open</span>
                      <span className="font-mono">{s.backlog} backlog</span>
                      {s.overdue > 0 && <span className="font-mono text-status-blocked">{s.overdue} overdue</span>}
                      <span className="ml-auto font-mono">{s.done}/{s.total}</span>
                    </div>
                    <div className="inset-neu h-2 rounded-md p-0.5">
                      <div className="h-full rounded-sm bg-status-done opacity-80"
                        style={{ width: `${s.progress * 100}%` }} />
                    </div>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </section>

      {/* row 3 — burndown + throughput + CFD */}
      <div className="grid gap-6 lg:grid-cols-3">
        <section className="card-neu" aria-label="Burndown">
          <h2 className="mb-4 text-sm font-medium text-foreground">
            Burndown — 30d <span className="text-muted-foreground">({burn.mode})</span>
          </h2>
          <BurnChart actual={burn.actual} ideal={burn.ideal} mode={burn.mode} />
        </section>
        <section className="card-neu" aria-label="Throughput">
          <h2 className="mb-4 text-sm font-medium text-foreground">Throughput — 14d</h2>
          <BarChart data={bars} label="Throughput" />
        </section>
        <section className="card-neu" aria-label="Cumulative flow diagram">
          <h2 className="mb-4 text-sm font-medium text-foreground">
            Cumulative flow — 30d
            <span className="ml-2 inline-flex items-center gap-2 text-[11px] text-muted-foreground">
              <span className="flex items-center gap-1"><span className="inline-block size-2 rounded-full bg-status-in-progress" />created</span>
              <span className="flex items-center gap-1"><span className="inline-block size-2 rounded-full bg-status-done" />done</span>
            </span>
          </h2>
          <CfdChart data={cfdSeries} />
        </section>
      </div>

      {/* row 4 — cycle histogram + aging wip + type mix */}
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

        <section className="card-neu" aria-label="Aging work in progress">
          <h2 className="mb-4 text-sm font-medium text-foreground">Aging WIP — stuck &gt; 7d</h2>
          {aging.length === 0 ? (
            <p className="text-sm text-muted-foreground">Nothing stuck. 🎉</p>
          ) : (
            <ul className="space-y-2">
              {aging.map(({ task: t, ageDays }) => (
                <li key={t.id}>
                  <button onClick={() => onSelectTask(t.project_id, t.id)}
                    className="inset-neu flex w-full items-center gap-3 px-3 py-2 text-left transition-shadow hover:shadow-[var(--shadow-inset),var(--shadow-glow)]">
                    <span className={`size-2 shrink-0 rounded-full ${STATUS_DOT[t.status]}`} />
                    <span className="flex-1 truncate text-sm text-foreground">{t.title}</span>
                    <span className="shrink-0 font-mono text-[10px] text-status-blocked">{ageDays.toFixed(0)}d</span>
                    <span className="shrink-0 font-mono text-[10px] text-muted-foreground uppercase">{t.project_key}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="card-neu" aria-label="Type mix">
          <h2 className="mb-4 text-sm font-medium text-foreground">Type mix</h2>
          {/* stacked bar */}
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
      </div>

      {/* row 5 — status + workload + attention */}
      <div className="grid gap-6 lg:grid-cols-3">
        <section className="card-neu" aria-label="Open tasks by status">
          <h2 className="mb-4 text-sm font-medium text-foreground">Open tasks by status</h2>
          {byStatus.length === 0 ? (
            <p className="text-sm text-muted-foreground">Nothing open.</p>
          ) : (
            <ul className="space-y-3">
              {byStatus.map(({ status, tasks: st }) => (
                <li key={status} className="space-y-1.5">
                  <div className="flex items-center justify-between text-xs">
                    <span className="flex items-center gap-2 text-muted-foreground">
                      <span className={`size-2 rounded-full ${STATUS_DOT[status]}`} />
                      {STATUS_LABELS[status]}
                    </span>
                    <span className="font-mono text-muted-foreground">{st.length}</span>
                  </div>
                  <div className="inset-neu h-2.5 rounded-md p-0.5">
                    <div className={`h-full rounded-sm ${STATUS_DOT[status]} opacity-80`}
                      style={{ width: `${(st.length / maxCount) * 100}%` }} />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="card-neu" aria-label="Workload by member">
          <h2 className="mb-4 text-sm font-medium text-foreground">Workload by member</h2>
          {members.length === 0 ? (
            <p className="text-sm text-muted-foreground">No members yet.</p>
          ) : (
            <ul className="space-y-2">
              {members.map((m) => {
                const mo = open.filter((t) => t.assignee?.id === m.id)
                const mod = mo.filter((t) => t.due_date && t.due_date < today).length
                return (
                  <li key={m.id}>
                    <button onClick={() => setMemberId(m.id)}
                      className="inset-neu flex w-full items-center gap-3 px-3 py-2 text-left transition-shadow hover:shadow-[var(--shadow-inset),var(--shadow-glow)]">
                      <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-secondary font-mono text-[10px] text-muted-foreground">
                        {initials(m.name)}
                      </span>
                      <span className="flex-1 truncate text-sm text-foreground">{m.name}</span>
                      {mod > 0 && <span className="font-mono text-[10px] text-status-blocked">{mod} overdue</span>}
                      <span className="font-mono text-xs text-muted-foreground">{mo.length} open</span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </section>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <section className="card-neu" aria-label="Needs attention">
          <h2 className="mb-4 text-sm font-medium text-foreground">Needs attention</h2>
          {attention.length === 0 ? (
            <p className="text-sm text-muted-foreground">Nothing overdue or blocked.</p>
          ) : (
            <ul className="space-y-2">
              {attention.map((t) => (
                <li key={t.id}>
                  <button onClick={() => onSelectTask(t.project_id, t.id)}
                    className="inset-neu flex w-full items-center gap-3 px-3 py-2 text-left transition-shadow hover:shadow-[var(--shadow-inset),var(--shadow-glow)]">
                    <span className={`size-2 shrink-0 rounded-full ${STATUS_DOT[t.status]}`} />
                    <span className="flex-1 truncate text-sm text-foreground">{t.title}</span>
                    {t.assignee && <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">{t.assignee.name}</span>}
                    <span className="shrink-0 font-mono text-[10px] text-muted-foreground uppercase">{t.project_key}</span>
                    {t.due_date && (
                      <span className={`shrink-0 font-mono text-[10px] ${t.due_date < today ? 'text-status-blocked' : 'text-muted-foreground'}`}>
                        {t.due_date}
                      </span>
                    )}
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
