import { useSortable } from '@dnd-kit/sortable'
import { GithubIcon } from '@/components/github-icon'
import { CSS } from '@dnd-kit/utilities'
import { PriorityDot, TaskKey } from './priority-dot'
import { taskKey } from '@/lib/board'
import type { Task } from '@/lib/types'

export function TaskCard({
  task,
  projectKey,
  onOpen,
}: {
  task: Task
  projectKey: string
  onOpen: (id: string) => void
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: task.id,
    data: { type: 'task', status: task.status },
  })

  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      {...attributes}
      {...listeners}
      role="button"
      tabIndex={0}
      aria-label={`${taskKey(projectKey, task)} ${task.title}`}
      onClick={() => onOpen(task.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onOpen(task.id)
        }
      }}
      className={`card-neu cursor-grab p-3 text-left select-none active:cursor-grabbing focus-visible:outline-2 focus-visible:outline-ring ${
        isDragging ? 'opacity-40' : ''
      }`}
    >
      <CardBody task={task} projectKey={projectKey} />
    </div>
  )
}

/** DragOverlay copy — lifted look per DESIGN.md (scale 1.02 + stronger shadow). */
export function TaskCardOverlay({ task, projectKey }: { task: Task; projectKey: string }) {
  return (
    <div className="card-neu w-72 rotate-[1.2deg] scale-[1.02] p-3 opacity-95 shadow-[0_16px_32px_rgba(0,0,0,0.5)]">
      <CardBody task={task} projectKey={projectKey} />
    </div>
  )
}

function CardBody({ task, projectKey }: { task: Task; projectKey: string }) {
  return (
    <div className="flex flex-col gap-2">
      <p className="text-sm leading-snug font-medium text-foreground">{task.title}</p>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <TaskKey taskKey={taskKey(projectKey, task)} status={task.status} />
        <PriorityDot priority={task.priority} />
        {task.gh_link && (
          <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-xs text-secondary-foreground">
            <GithubIcon className="size-3" strokeWidth={1.5} />
            #{task.gh_link.issue_number}
          </span>
        )}
        {task.labels.map((l) => (
          <span
            key={l.id}
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground"
            style={{ backgroundColor: `${l.color}22`, color: l.color }}
          >
            <span className="size-1.5 rounded-full" style={{ backgroundColor: l.color }} />
            {l.name}
          </span>
        ))}
        {task.assignee && (
          <span
            title={task.assignee.name}
            className="inset-neu ml-auto flex size-6 items-center justify-center rounded-full font-mono text-xs text-primary"
          >
            {initials(task.assignee.name || task.assignee.email)}
          </span>
        )}
      </div>
    </div>
  )
}

export function initials(s: string): string {
  const parts = s.split(/[\s.@]+/).filter(Boolean)
  return ((parts[0]?.[0] ?? '?') + (parts[1]?.[0] ?? '')).toUpperCase()
}
