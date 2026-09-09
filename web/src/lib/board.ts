// Pure board + filter logic. No React — unit-tested in board.test.ts.

import type { Status, Task } from './types'

export const BOARD_STATUSES: Status[] = [
  'inbox',
  'backlog',
  'in_progress',
  'review',
  'done',
  'blocked',
]

export const STATUS_LABEL: Record<Status, string> = {
  inbox: 'Inbox',
  backlog: 'Backlog',
  in_progress: 'In Progress',
  review: 'Review',
  done: 'Done',
  blocked: 'Blocked',
  trash: 'Trash',
}

export const PRIORITY_ORDER = ['urgent', 'high', 'medium', 'low'] as const

/** Server column order: position, then number (stable tiebreak). */
export function byPosition(a: Task, b: Task): number {
  return a.position - b.position || a.number - b.number
}

/** Tasks for one column, ordered. `exclude` skips a task id (the dragged one). */
export function columnOf(tasks: Task[], status: Status, exclude?: string): Task[] {
  return tasks.filter((t) => t.status === status && t.id !== exclude).sort(byPosition)
}

const GAP = 1024

export type MovePlan = {
  next: Task[]
  move: { status: Status; before_task_id: string | null; after_task_id: string | null }
}

/**
 * Compute a drag-drop move client-side, mirroring the server's position math
 * (api handlers.MoveTask): midpoint between neighbors, ±GAP at the ends,
 * GAP in an empty column. Pure: input arrays are never mutated.
 *
 * `toIndex` is the insertion slot within the target column after the dragged
 * task is removed (dnd-kit over index or column end for a drop on empty well).
 */
export function planMove(
  tasks: Task[],
  dragId: string,
  toStatus: Status,
  toIndex: number | null,
): MovePlan {
  const dragged = tasks.find((t) => t.id === dragId)
  if (!dragged) return { next: tasks, move: { status: toStatus, before_task_id: null, after_task_id: null } }

  const col = columnOf(tasks, toStatus, dragId)
  const idx =
    toIndex === null ? col.length : Math.max(0, Math.min(toIndex, col.length))

  const after = col[idx - 1] ?? null
  const before = col[idx] ?? null

  let position: number
  if (col.length === 0) position = GAP
  else if (idx === 0) position = col[0].position - GAP
  else if (idx === col.length) position = col[col.length - 1].position + GAP
  else position = (col[idx - 1].position + col[idx].position) / 2

  const moved = { ...dragged, status: toStatus, position }
  return {
    next: tasks.map((t) => (t.id === dragId ? moved : t)),
    move: {
      status: toStatus,
      before_task_id: before?.id ?? null,
      after_task_id: after?.id ?? null,
    },
  }
}

export type TaskFilters = {
  q: string
  assignee_id: string // user id, '' = all
  priority: string
  label_id: string
  status: string
}

export const EMPTY_FILTERS: TaskFilters = {
  q: '',
  assignee_id: '',
  priority: '',
  label_id: '',
  status: '',
}

export function filtersActive(f: TaskFilters): number {
  return (Object.keys(EMPTY_FILTERS) as (keyof TaskFilters)[]).filter(
    (k) => f[k] !== '',
  ).length
}

/** Client-side filter predicate — same semantics the server applies per-field. */
export function matchesFilters(t: Task, f: TaskFilters): boolean {
  if (f.status !== '' && t.status !== f.status) return false
  if (f.priority !== '' && t.priority !== f.priority) return false
  if (f.assignee_id !== '' && t.assignee?.id !== f.assignee_id) return false
  if (f.label_id !== '' && !t.labels.some((l) => l.id === f.label_id)) return false
  if (f.q !== '') {
    const q = f.q.trim().toLowerCase()
    if (q !== '' && !t.title.toLowerCase().includes(q) && !t.description.toLowerCase().includes(q)) {
      return false
    }
  }
  return true
}

/** All non-trash tasks that pass the filters, column order preserved. */
export function boardTasks(tasks: Task[], f: TaskFilters): Task[] {
  return tasks.filter((t) => t.status !== 'trash' && matchesFilters(t, f)).sort(byPosition)
}

export function taskKey(projectKey: string, t: Task): string {
  return `${projectKey}-${t.number}`
}
