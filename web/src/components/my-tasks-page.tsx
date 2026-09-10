import { useEffect, useState } from 'react'
import { api, ApiError } from '@/lib/api'
import type { Task } from '@/lib/types'
import { useAuth } from '@/lib/auth'
import { STATUS_LABELS, PRIORITY_LABELS } from '@/lib/labels'

const STATUS_COLORS: Record<string, string> = {
  inbox:       'text-muted-foreground/70 bg-muted/30 border-border/40',
  backlog:     'text-blue-400/80 bg-blue-900/15 border-blue-400/25',
  in_progress: 'text-amber-400 bg-amber-900/20 border-amber-400/30',
  review:      'text-violet-400 bg-violet-900/15 border-violet-400/25',
  done:        'text-emerald-400 bg-emerald-900/15 border-emerald-400/25',
  blocked:     'text-red-400 bg-red-900/15 border-red-400/25',
  trash:       'text-muted-foreground/40 bg-muted/10 border-border/30 line-through',
}
const PRIORITY_COLORS: Record<string, string> = {
  urgent: 'text-red-400 bg-red-900/15 border-red-400/25',
  high:   'text-orange-400 bg-orange-900/15 border-orange-400/25',
  medium: 'text-muted-foreground/70 bg-muted/30 border-border/40',
  low:    'text-blue-300/70 bg-blue-900/10 border-blue-300/20',
}

interface Props {
  /** called when user clicks a task row: workspace opens that project board + drawer */
  onSelectTask: (projectId: string, taskId: string) => void
}

export default function MyTasksPage({ onSelectTask }: Props) {
  const { state } = useAuth()
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (state.phase !== 'authenticated') return
    setLoading(true)
    api
      .myFetchMyTasks(state.user.id)
      .then((res) => setTasks(res.data))
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Failed to load tasks'))
      .finally(() => setLoading(false))
  }, [state])

  if (loading) {
    return (
      <div className="p-8 text-center text-muted-foreground/60 text-sm animate-pulse">
        Loading tasks…
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8 text-center text-destructive text-sm">{error}</div>
    )
  }

  if (tasks.length === 0) {
    return (
      <div className="p-8 text-center text-muted-foreground/50 text-sm">
        No tasks assigned to you across any project.
      </div>
    )
  }

  return (
    <div className="px-3 py-4 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-foreground">My Tasks</h1>
        <span className="text-xs text-muted-foreground/50">{tasks.length} task{tasks.length !== 1 && 's'}</span>
      </div>

      {/* simple neumorphic list rows — one per task, click → open in project board */}
      <ul className="space-y-2">
        {tasks.map((task) => (
          <li key={task.id}>
            <button
              onClick={() => onSelectTask(task.project_id, task.id)}
              className="w-full text-left card-neu px-4 py-3
                         hover:shadow-[var(--shadow-raised),var(--shadow-glow)] transition-shadow cursor-pointer
                         flex items-start gap-3"
            >
              {/* project key mono chip */}
              {task.project_key && (
                <span className="shrink-0 mt-0.5 inline-flex items-center rounded-md bg-secondary/60
                                 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">
                  {task.project_key}
                </span>
              )}

              <div className="flex-1 min-w-0">
                <p className="text-sm text-foreground truncate font-medium">
                  {task.title}
                </p>
                <p className="text-xs text-muted-foreground/50 mt-0.5">
                  {task.project_name ?? ''}
                </p>
              </div>

              {/* status + priority labels */}
              <div className="shrink-0 flex items-center gap-1.5 mt-0.5">
                <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded-md border ${STATUS_COLORS[task.status]}`}>
                  {STATUS_LABELS[task.status]}
                </span>
                <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded-md border ${PRIORITY_COLORS[task.priority]}`}>
                  {PRIORITY_LABELS[task.priority]}
                </span>
              </div>

              {/* due date */}
              {task.due_date && (
                <span className="shrink-0 text-[10px] text-muted-foreground/50 font-mono mt-0.5">
                  {task.due_date}
                </span>
              )}
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
