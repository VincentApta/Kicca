import type { Priority, Status } from '@/lib/types'

const PRIORITY_BG: Record<Priority, string> = {
  urgent: 'bg-priority-urgent',
  high: 'bg-priority-high',
  medium: 'bg-priority-medium',
  low: 'bg-priority-low',
}

export function PriorityDot({ priority, title }: { priority: Priority; title?: string }) {
  return (
    <span
      title={title ?? priority}
      aria-label={`Priority: ${priority}`}
      className={`inline-block size-2 shrink-0 rounded-full ${PRIORITY_BG[priority]}`}
    />
  )
}

const STATUS_LINE: Record<Status, string> = {
  inbox: 'decoration-status-inbox',
  backlog: 'decoration-status-backlog',
  in_progress: 'decoration-status-in-progress',
  review: 'decoration-status-review',
  done: 'decoration-status-done',
  blocked: 'decoration-status-blocked',
  trash: 'decoration-status-backlog',
}

/** Mono task key with status-colored underline (DESIGN.md signature detail). */
export function TaskKey({
  taskKey: k,
  status,
  className = '',
}: {
  taskKey: string
  status: Status
  className?: string
}) {
  return (
    <span
      className={`font-mono text-xs text-muted-foreground underline decoration-2 underline-offset-4 ${STATUS_LINE[status]} ${className}`}
    >
      {k}
    </span>
  )
}
