import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { useState } from 'react'
import { BoardColumn } from './board-column'
import { TaskCardOverlay } from './task-card'
import { BOARD_STATUSES } from '@/lib/board'
import type { Status, Task } from '@/lib/types'

export function BoardView({
  tasks,
  projectKey,
  onOpen,
  onAdd,
  onMove,
}: {
  tasks: Task[]
  projectKey: string
  onOpen: (id: string) => void
  onAdd: (status: Status) => void
  onMove: (dragId: string, toStatus: Status, toIndex: number | null) => void
}) {
  const [dragId, setDragId] = useState<string | null>(null)
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))

  function onDragStart(e: DragStartEvent) {
    setDragId(String(e.active.id))
  }

  function onDragEnd(e: DragEndEvent) {
    setDragId(null)
    const { active, over } = e
    if (!over) return
    const dragId = String(active.id)
    if (active.id === over.id) return

    const data = over.data.current as
      | { sortable?: { containerId: unknown; index: number }; type?: string; status?: Status }
      | undefined

    let toStatus: Status | undefined
    let toIndex: number | null = null
    if (data?.sortable) {
      toStatus = String(data.sortable.containerId).replace('col-', '') as Status
      toIndex = data.sortable.index
    } else if (data?.type === 'column') {
      toStatus = data.status
    }
    if (!toStatus || toStatus === 'trash') return
    onMove(dragId, toStatus, toIndex)
  }

  const columns = BOARD_STATUSES.map((status) => ({
    status,
    tasks: tasks.filter((t) => t.status === status),
  }))
  const dragging = tasks.find((t) => t.id === dragId)

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragCancel={() => setDragId(null)}
    >
      <div className="flex flex-1 items-start gap-4 overflow-x-auto p-4 pb-8">
        {columns.map((c) => (
          <BoardColumn
            key={c.status}
            status={c.status}
            tasks={c.tasks}
            projectKey={projectKey}
            onOpen={onOpen}
            onAdd={onAdd}
          />
        ))}
      </div>
      <DragOverlay>{dragging && <TaskCardOverlay task={dragging} projectKey={projectKey} />}</DragOverlay>
    </DndContext>
  )
}
