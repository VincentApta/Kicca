// Dashboard — TEAM overview: all tasks across visible projects, filterable
// by member + project. Uses the tasks feed endpoint + listProjects.
import { useEffect, useMemo, useState } from 'react'
import { FolderKanbanIcon, ListTodoIcon, CheckCircle2Icon, AlertTriangleIcon } from 'lucide-react'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { api, ApiError } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import type { Project, Task, User } from '@/lib/types'
import { STATUS_LABELS } from '@/lib/labels'

const OPEN_STATUSES = ['inbox', 'backlog', 'in_progress', 'review', 'blocked'] as const

const STATUS_DOT: Record<string, string> = {
  inbox: 'bg-status-inbox',
  backlog: 'bg-status-backlog',
  in_progress: 'bg-status-in-progress',
  review: 'bg-status-review',
  done: 'bg-status-done',
  blocked: 'bg-status-blocked',
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: number }) {
  return (
    <div className="card-neu flex items-center gap-4">
      <div className="inset-neu flex size-10 shrink-0 items-center justify-center text-primary">
        {icon}
      </div>
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

export function DashboardPage({ onSelectTask }: { onSelectTask: (projectId: string, taskId: string) => void }) {
  const { state } = useAuth()
  const [projects, setProjects] = useState<Project[] | null>(null)
  const [tasks, setTasks] = useState<Task[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  // filters — ALL by default
  const [memberId, setMemberId] = useState<string>('all')
  const [projectId, setProjectId] = useState<string>('all')

  const me = state.phase === 'authenticated' ? state.user : null

  useEffect(() => {
    if (state.phase !== 'authenticated') return
    api.listProjects().then((r) => setProjects(r.data)).catch(() => setError('Could not load projects'))
  }, [state])

  useEffect(() => {
    if (state.phase !== 'authenticated') return
    setTasks(null)
    api
      .myFetchMyTasks(memberId === 'all' ? undefined : memberId)
      .then((r) => setTasks(r.data))
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Could not load tasks'))
  }, [state, memberId])

  const allTasks = tasks ?? []
  const shown = useMemo(
    () => (projectId === 'all' ? allTasks : allTasks.filter((t) => t.project_id === projectId)),
    [allTasks, projectId],
  )

  // member list = assignees seen in tasks + me (even with no tasks)
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

  const open = shown.filter((t) => (OPEN_STATUSES as readonly string[]).includes(t.status))
  const today = new Date().toISOString().slice(0, 10)
  const overdue = open.filter((t) => t.due_date && t.due_date < today)
  const done = shown.filter((t) => t.status === 'done')

  const byStatus = OPEN_STATUSES.map((s) => ({
    status: s,
    tasks: open.filter((t) => t.status === s),
  })).filter((b) => b.tasks.length > 0)
  const maxCount = Math.max(1, ...byStatus.map((b) => b.tasks.length))

  const attention = [...overdue, ...open.filter((t) => t.status === 'blocked')].slice(0, 8)

  return (
    <div className="flex-1 overflow-auto p-6 space-y-6">
      {/* header + filters */}
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Dashboard</h1>
        <div className="ml-auto flex items-center gap-2">
          <Select value={memberId} onValueChange={(v) => v !== null && setMemberId(v)}>
            <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label="Member">
              <SelectValue placeholder="Member">
                {memberId === 'all' ? 'All members' : members.find((m) => m.id === memberId)?.name ?? 'All members'}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All members</SelectItem>
              {members.map((m) => (
                <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>
              ))}
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
              {projects.map((p) => (
                <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* stat row */}
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard icon={<FolderKanbanIcon className="size-5" strokeWidth={1.5} />} label="Projects" value={projects.length} />
        <StatCard icon={<ListTodoIcon className="size-5" strokeWidth={1.5} />} label="Open tasks" value={open.length} />
        <StatCard icon={<CheckCircle2Icon className="size-5" strokeWidth={1.5} />} label="Done" value={done.length} />
        <StatCard icon={<AlertTriangleIcon className="size-5 text-status-blocked" strokeWidth={1.5} />} label="Overdue" value={overdue.length} />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* open tasks by status */}
        <section className="card-neu" aria-label="Open tasks by status">
          <h2 className="text-sm font-medium text-foreground mb-4">Open tasks by status</h2>
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
                    <div
                      className={`h-full rounded-sm ${STATUS_DOT[status]} opacity-80`}
                      style={{ width: `${(st.length / maxCount) * 100}%` }}
                    />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        {/* workload by member — open tasks per assignee */}
        <section className="card-neu" aria-label="Workload by member">
          <h2 className="text-sm font-medium text-foreground mb-4">Workload by member</h2>
          {members.length === 0 ? (
            <p className="text-sm text-muted-foreground">No members yet.</p>
          ) : (
            <ul className="space-y-2">
              {members.map((m) => {
                const mo = open.filter((t) => t.assignee?.id === m.id)
                const mod = mo.filter((t) => t.due_date && t.due_date < today).length
                return (
                  <li key={m.id}>
                    <button
                      onClick={() => setMemberId(m.id)}
                      className="inset-neu flex w-full items-center gap-3 px-3 py-2 text-left transition-shadow hover:shadow-[var(--shadow-inset),var(--shadow-glow)]"
                    >
                      <span className="inset-neu flex size-6 shrink-0 items-center justify-center rounded-full font-mono text-[10px] text-muted-foreground">
                        {initials(m.name)}
                      </span>
                      <span className="flex-1 truncate text-sm text-foreground">{m.name}</span>
                      {mod > 0 && (
                        <span className="font-mono text-[10px] text-status-blocked">{mod} overdue</span>
                      )}
                      <span className="font-mono text-xs text-muted-foreground">{mo.length} open</span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </section>
      </div>

      {/* needs attention — overdue + blocked, click through */}
      <section className="card-neu" aria-label="Needs attention">
        <h2 className="text-sm font-medium text-foreground mb-4">Needs attention</h2>
        {attention.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nothing overdue or blocked.</p>
        ) : (
          <ul className="space-y-2">
            {attention.map((t) => (
              <li key={t.id}>
                <button
                  onClick={() => onSelectTask(t.project_id, t.id)}
                  className="inset-neu flex w-full items-center gap-3 px-3 py-2 text-left transition-shadow hover:shadow-[var(--shadow-inset),var(--shadow-glow)]"
                >
                  <span className={`size-2 shrink-0 rounded-full ${STATUS_DOT[t.status]}`} />
                  <span className="flex-1 truncate text-sm text-foreground">{t.title}</span>
                  {t.assignee && (
                    <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">{t.assignee.name}</span>
                  )}
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
  )
}
// ponytail: member list from task assignees only — users with zero tasks in
// scope are invisible; swap to GET /users (admin) or a members endpoint later.
