import { PlusIcon } from 'lucide-react'
import { SortableContext } from '@dnd-kit/sortable'
import { useDroppable } from '@dnd-kit/core'
import { TaskCard } from './task-card'
import { STATUS_LABEL } from '@/lib/board'
import type { Status, Task } from '@/lib/types'

const EMPTY_CTA: Partial<Record<Status, string>> = {
  inbox: 'Drop raw requests here',
}

export function BoardColumn({
  status,
  tasks,
  projectKey,
  onOpen,
  onAdd,
  selectable = false,
  selectedIds,
  onSelect,
}: {
  status: Status
  tasks: Task[]
  projectKey: string
  onOpen: (id: string) => void
  onAdd: (status: Status) => void
  selectable?: boolean
  selectedIds?: Set<string>
  onSelect?: (id: string, shiftKey: boolean) => void
}) {
  const id = `col-${status}`
  const { setNodeRef, isOver } = useDroppable({ id, data: { type: 'column', status } })

  return (
    <section className="flex w-80 shrink-0 flex-col gap-2" aria-label={`${STATUS_LABEL[status]} column`}>
      <header className="flex items-center gap-2 px-1">
        <h2 className="text-sm font-medium text-foreground">{STATUS_LABEL[status]}</h2>
        <span className="font-mono text-xs text-muted-foreground">{tasks.length}</span>
        <button
          type="button"
          onClick={() => onAdd(status)}
          aria-label={`Add task to ${STATUS_LABEL[status]}`}
          className="btn-neu ml-auto flex size-6 items-center justify-center rounded-md text-muted-foreground hover:text-foreground"
        >
          <PlusIcon className="size-3.5" strokeWidth={1.5} />
        </button>
      </header>

      {/* inset well — the DESIGN.md signature: cards sit IN trays */}
      <div
        ref={setNodeRef}
        className={`inset-neu flex min-h-40 flex-1 flex-col gap-2 p-4 transition-shadow ${
          isOver ? 'shadow-[var(--shadow-inset),var(--shadow-glow)]' : ''
        }`}
      >
        <SortableContext id={id} items={tasks.map((t) => t.id)}>
          {tasks.map((t) => (
            <TaskCard
              key={t.id}
              task={t}
              projectKey={projectKey}
              onOpen={onOpen}
              selectable={selectable}
              selected={selectedIds?.has(t.id) ?? false}
              onSelect={onSelect}
            />
          ))}
        </SortableContext>
        {tasks.length === 0 && (
          <p className="flex flex-1 items-center justify-center px-2 text-center text-xs text-muted-foreground">
            {EMPTY_CTA[status] ?? 'No tasks'}
          </p>
        )}
      </div>
    </section>
  )
}
