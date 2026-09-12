import { CheckIcon } from 'lucide-react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { PriorityDot, TaskKey } from './board/priority-dot'
import { STATUS_LABEL, taskKey } from '@/lib/board'
import type { Label, Task } from '@/lib/types'

const STATUS_TEXT: Record<string, string> = {
  inbox: 'text-status-inbox',
  backlog: 'text-status-backlog',
  in_progress: 'text-status-in-progress',
  review: 'text-status-review',
  done: 'text-status-done',
  blocked: 'text-status-blocked',
  trash: 'text-muted-foreground',
}

export function ListView({
  tasks,
  projectKey,
  onOpen,
  rowAction,
  selectable = false,
  selectedIds,
  onSelect,
}: {
  tasks: Task[]
  projectKey: string
  onOpen: (id: string) => void
  rowAction?: (task: Task) => React.ReactNode
  selectable?: boolean // multi-select mode (issue #46)
  selectedIds?: Set<string>
  onSelect?: (id: string, shiftKey: boolean) => void
}) {
  return (
    <div className="flex-1 overflow-auto p-4 pb-8">
      <Table className="min-w-160">
        <TableHeader>
          <TableRow className="sticky top-0 z-10 bg-background hover:bg-background">
            {selectable && <TableHead className="w-10" aria-label="Selected" />}
            <TableHead className="w-24">Key</TableHead>
            <TableHead>Title</TableHead>
            <TableHead className="w-28">Status</TableHead>
            <TableHead className="w-24">Priority</TableHead>
            <TableHead className="w-36">Assignee</TableHead>
            <TableHead className="w-44">Labels</TableHead>
            <TableHead className="w-28">Due</TableHead>
            <TableHead className="w-28">Updated</TableHead>
            {rowAction && <TableHead className="w-20" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {tasks.map((t) => (
            <TableRow
              key={t.id}
              tabIndex={0}
              aria-pressed={selectable ? (selectedIds?.has(t.id) ?? false) : undefined}
              onClick={(e) => {
                if (selectable || e.shiftKey) onSelect?.(t.id, e.shiftKey)
                else onOpen(t.id)
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  if (selectable || e.shiftKey) onSelect?.(t.id, e.shiftKey)
                  else onOpen(t.id)
                }
              }}
              className={`cursor-pointer ${selectable && selectedIds?.has(t.id) ? 'outline-2 -outline-offset-2 outline-primary' : ''}`}
            >
              {selectable && (
                <TableCell>
                  <span
                    aria-hidden
                    className={`flex size-5 items-center justify-center rounded-md ${
                      selectedIds?.has(t.id) ? 'bg-primary/15 text-primary' : 'inset-neu text-transparent'
                    }`}
                  >
                    <CheckIcon className="size-3.5" strokeWidth={2} />
                  </span>
                </TableCell>
              )}
              <TableCell>
                <TaskKey taskKey={taskKey(projectKey, t)} status={t.status} />
              </TableCell>
              <TableCell className="max-w-96 truncate font-medium">{t.title}</TableCell>
              <TableCell className={`text-xs ${STATUS_TEXT[t.status] ?? ''}`}>
                {STATUS_LABEL[t.status]}
              </TableCell>
              <TableCell>
                <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <PriorityDot priority={t.priority} />
                  {t.priority}
                </span>
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {t.assignee?.name ?? '—'}
              </TableCell>
              <TableCell>
                <LabelChips labels={t.labels} />
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {t.due_date ?? '—'}
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {fmtDate(t.updated_at)}
              </TableCell>
              {rowAction && <TableCell onClick={(e) => e.stopPropagation()}>{rowAction(t)}</TableCell>}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function LabelChips({ labels }: { labels: Label[] }) {
  if (labels.length === 0) return <span className="text-xs text-muted-foreground">—</span>
  return (
    <span className="flex flex-wrap gap-1">
      {labels.map((l) => (
        <span
          key={l.id}
          className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs"
          style={{ backgroundColor: `${l.color}22`, color: l.color }}
        >
          <span className="size-1.5 rounded-full" style={{ backgroundColor: l.color }} />
          {l.name}
        </span>
      ))}
    </span>
  )
}

export function fmtDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toISOString().slice(0, 10)
}
