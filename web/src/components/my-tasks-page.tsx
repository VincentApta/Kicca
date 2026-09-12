import { useEffect, useState } from 'react'
import { api, ApiError } from '@/lib/api'
import type { Task } from '@/lib/types'
import { useAuth } from '@/lib/auth'
import { STATUS_LABELS, PRIORITY_LABELS } from '@/lib/labels'
import { Button } from './ui/button'

// Light-mode readable pills: dark text on light tint in light mode,
// original light text on dark tint in dark mode.
const STATUS_COLORS: Record<string, string> = {
  inbox:       'text-muted-foreground/70 bg-muted/30 border-border/40',
  backlog:     'text-blue-700 bg-blue-100 border-blue-300 dark:text-blue-400/80 dark:bg-blue-900/15 dark:border-blue-400/25',
  in_progress: 'text-amber-700 bg-amber-100 border-amber-300 dark:text-amber-400 dark:bg-amber-900/20 dark:border-amber-400/30',
  review:      'text-violet-700 bg-violet-100 border-violet-300 dark:text-violet-400 dark:bg-violet-900/15 dark:border-violet-400/25',
  done:        'text-emerald-700 bg-emerald-100 border-emerald-300 dark:text-emerald-400 dark:bg-emerald-900/15 dark:border-emerald-400/25',
  blocked:     'text-red-700 bg-red-100 border-red-300 dark:text-red-400 dark:bg-red-900/15 dark:border-red-400/25',
  trash:       'text-muted-foreground/40 bg-muted/10 border-border/30 line-through',
}
const PRIORITY_COLORS: Record<string, string> = {
  urgent: 'text-red-700 bg-red-100 border-red-300 dark:text-red-400 dark:bg-red-900/15 dark:border-red-400/25',
  high:   'text-orange-700 bg-orange-100 border-orange-300 dark:text-orange-400 dark:bg-orange-900/15 dark:border-orange-400/25',
  medium: 'text-muted-foreground/70 bg-muted/30 border-border/40',
  low:    'text-blue-600 bg-blue-50 border-blue-200 dark:text-blue-300/70 dark:bg-blue-900/10 dark:border-blue-300/20',
}

interface Props {
  /** called when user clicks a task row: workspace opens that project board + drawer */
  onSelectTask: (projectId: string, taskId: string) => void
}

export default function MyTasksPage({ onSelectTask }: Props) {
  const { state } = useAuth()
  const [tasks, setTasks] = useState<Task[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (state.phase !== 'authenticated') return
    setLoading(true)
    setError(null)
    api
      .myFetchMyTasks(state.user.id, page)
      .then((res) => {
        setTasks((prev) => (page === 1 ? res.data : [...prev, ...res.data]))
        setTotal(res.total)
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Failed to load tasks'))
      .finally(() => setLoading(false))
  }, [state, page])

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
    <div className="flex-1 overflow-auto p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-foreground">My Tasks</h1>
        <span className="text-xs text-muted-foreground/50">{total} task{total !== 1 && 's'}</span>
      </div>

      {/* simple neumorphic list rows — one per task, click → open in project board */}
      <ul className="space-y-4">
        {tasks.map((task) => (
          <li key={task.id}>
            <button
              onClick={() => onSelectTask(task.project_id, task.id)}
              className="w-full text-left card-neu hover:shadow-[var(--shadow-raised),var(--shadow-glow)] transition-shadow cursor-pointer flex items-start gap-3"
            >
              {/* project key mono chip */}
              {task.project_key && (
                <span className="shrink-0 mt-0.5 inline-flex items-center rounded-md bg-secondary/60
                                 px-1.5 py-0.5 font-mono text-xs font-semibold text-muted-foreground uppercase tracking-wider">
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
                <span className={`text-xs font-semibold px-2 py-0.5 rounded-md border ${STATUS_COLORS[task.status]}`}>
                  {STATUS_LABELS[task.status]}
                </span>
                <span className={`text-xs font-semibold px-2 py-0.5 rounded-md border ${PRIORITY_COLORS[task.priority]}`}>
                  {PRIORITY_LABELS[task.priority]}
                </span>
              </div>

              {/* due date */}
              {task.due_date && (
                <span className="shrink-0 text-xs text-muted-foreground/50 font-mono mt-0.5">
                  {task.due_date}
                </span>
              )}
            </button>
          </li>
        ))}
      </ul>

      {tasks.length < total && (
        <div className="pt-2 text-center">
          <Button
            variant="outline"
            size="sm"
            disabled={loading}
            onClick={() => setPage((p) => p + 1)}
          >
            Show more ({total - tasks.length} left)
          </Button>
        </div>
      )}
    </div>
  )
}
