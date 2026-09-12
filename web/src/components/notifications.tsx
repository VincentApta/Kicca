// NotificationBell — 30s poll, unread badge, click → navigate, mark-read on open.
import { useCallback, useEffect, useRef, useState } from 'react'
import { BellIcon } from 'lucide-react'
import { api } from '@/lib/api'
import type { Notification as AppNotification } from '@/lib/types'
import { Button } from './ui/button'

const EVENT_LABELS: Record<string, string> = {
  status: 'changed status',
  assign: 'was assigned',
  comment: 'commented',
}
const STATUS_BADGE: Record<string, string> = {
  backlog: 'bg-gray-400',
  inbox: 'bg-amber-400',
  'in-progress': 'bg-blue-400',
  review: 'bg-purple-400',
  blocked: 'bg-red-400',
  done: 'bg-green-500',
}

function relTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  if (diff < 60_000) return 'just now'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`
  return `${Math.floor(diff / 86_400_000)}d ago`
}

export function NotificationBell({
  onOpenTask,
  className,
}: {
  onOpenTask: (taskId: string, projectId: string) => void
  className?: string
}) {
  const [items, setItems] = useState<AppNotification[]>([])
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const timer = useRef<ReturnType<typeof setInterval> | null>(null)

  const load = useCallback(() => {
    api.listNotifications().then((r) => setItems(r.data)).catch(() => { /* swallow */ })
  }, [])

  useEffect(() => { load(); timer.current = setInterval(load, 30_000); return () => { if (timer.current) clearInterval(timer.current) } }, [load])

  // click-outside close
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  function toggle() {
    if (!open) {
      // mark-read when opening
      api.markNotificationsRead().catch(() => { /* swallow */ })
      setItems([])
    }
    setOpen(!open)
  }

  const n = items.length

  return (
    <div className={`relative ${className ?? ''}`} ref={ref}>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={toggle}
        aria-label={`Notifications${n > 0 ? `, ${n} unread` : ''}`}
        className="relative"
      >
        <BellIcon strokeWidth={1.5} />
        {n > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex size-4 items-center justify-center rounded-full bg-destructive text-[10px] font-bold text-white">
            {n > 9 ? '9+' : n}
          </span>
        )}
      </Button>

      {open && (
        <div className="absolute right-0 top-full z-50 mt-2 w-80 max-h-96 overflow-y-auto rounded-xl border border-border bg-background shadow-xl p-0">
          <div className="border-b border-border px-4 py-2">
            <span className="text-xs font-medium text-muted-foreground">
              {n === 0 ? 'All caught up' : `${n} unread notification${n > 1 ? 's' : ''}`}
            </span>
          </div>
          {n === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-muted-foreground">No new activity.</p>
          ) : (
            <ul className="divide-y divide-border">
              {items.map((it) => (
                <li key={it.id}>
                  <button
                    type="button"
                    onClick={() => { setOpen(false); onOpenTask(it.task_id, it.project_id) }}
                    className="flex w-full items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-accent"
                  >
                    <span className={`mt-1 size-2 shrink-0 rounded-full ${STATUS_BADGE[it.to_status] ?? 'bg-muted-foreground'}`} />
                    <div className="min-w-0 flex-1">
                      <p className="text-xs text-foreground leading-snug">
                        <span className="font-medium">{it.actor_name}</span>{' '}
                        <span className="text-muted-foreground">{EVENT_LABELS[it.type] ?? it.type}</span>
                      </p>
                      <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                        {it.project_key}#{it.task_number} — {it.task_title}
                      </p>
                    </div>
                    <span className="mt-0.5 shrink-0 text-[10px] text-muted-foreground">{relTime(it.at)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
