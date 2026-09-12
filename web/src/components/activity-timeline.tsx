// Activity timeline (issue #44): a task's task_events log as a read-only
// vertical list, newest first — `actor name · status change · relative time`.
// Actor is rendered as name only on both surfaces (the client API never
// sends email/role; the team shape is narrowed here the same way).
import { STATUS_LABELS } from '@/lib/labels'
import type { Status, TaskActivityEvent } from '@/lib/types'

function actorName(actor: TaskActivityEvent['actor']): string {
  return 'name' in actor ? actor.name : 'Unknown'
}

/** compact relative time — "3m ago" style, capped at days */
function relativeTime(iso: string): string {
  const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return 'just now'
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

function statusChange(e: TaskActivityEvent): string {
  if (e.from_status == null) return 'created'
  return `${STATUS_LABELS[e.from_status as Status]} → ${STATUS_LABELS[e.to_status]}`
}

export function ActivityTimeline({ events }: { events: TaskActivityEvent[] }) {
  return (
    <ol className="mt-2 flex flex-col" aria-label="Activity timeline">
      {events.map((e, i) => (
        <li key={e.id} className="relative flex gap-3 pb-3 last:pb-0">
          {i < events.length - 1 && (
            <span
              className="absolute top-4 bottom-0 left-[4px] w-px bg-border"
              aria-hidden
            />
          )}
          <span className="inset-neu z-10 mt-1.5 size-2.5 shrink-0 rounded-full" aria-hidden />
          <p className="flex min-w-0 flex-wrap items-baseline gap-x-2">
            <span className="text-xs font-medium text-foreground">{actorName(e.actor)}</span>
            <span className="text-xs text-muted-foreground">{statusChange(e)}</span>
            <span
              className="font-mono text-xs text-muted-foreground/60"
              title={new Date(e.at).toLocaleString()}
            >
              {relativeTime(e.at)}
            </span>
          </p>
        </li>
      ))}
    </ol>
  )
}
