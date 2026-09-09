import { useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { PRIORITY_ORDER, STATUS_LABEL } from '@/lib/board'
import type { Label as LabelT, Priority, ProjectMember, Status, Task } from '@/lib/types'

export function CreateTaskDialog({
  open,
  status,
  members,
  labels,
  onClose,
  onCreated,
  onCreate,
}: {
  open: boolean
  status: Status
  members: ProjectMember[]
  labels: LabelT[]
  onClose: () => void
  onCreated: (t: Task) => void
  onCreate: (body: {
    title: string
    description?: string
    status: Status
    priority?: Priority
    assignee_id?: string | null
    due_date?: string | null
    label_ids?: string[]
  }) => Promise<Task>
}) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState<Priority>('medium')
  const [assigneeId, setAssigneeId] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [labelIds, setLabelIds] = useState<string[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  function reset() {
    setTitle('')
    setDescription('')
    setPriority('medium')
    setAssigneeId('')
    setDueDate('')
    setLabelIds([])
    setError('')
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!title.trim()) return
    setBusy(true)
    setError('')
    try {
      const task = await onCreate({
        title: title.trim(),
        description: description.trim() || undefined,
        status,
        priority,
        assignee_id: assigneeId || null,
        due_date: dueDate || null,
        label_ids: labelIds,
      })
      onCreated(task)
      reset()
      onClose()
    } catch {
      setError('Could not create task.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>New task in {STATUS_LABEL[status]}</DialogTitle>
          <DialogDescription>Defaults to Medium priority.</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <div className="flex flex-col gap-2">
            <Label htmlFor="task-title">Title</Label>
            <Input
              id="task-title"
              className="inset-neu"
              required
              maxLength={200}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="task-desc">Description</Label>
            <Textarea
              id="task-desc"
              className="inset-neu min-h-24"
              placeholder="Markdown supported"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="task-priority">Priority</Label>
              <Select id="task-priority" value={priority} onValueChange={(v) => setPriority(v as Priority)}>
                <SelectTrigger className="inset-neu w-full border-0">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PRIORITY_ORDER.map((p) => (
                    <SelectItem key={p} value={p}>
                      {p[0].toUpperCase() + p.slice(1)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="task-assignee">Assignee</Label>
              <Select
                id="task-assignee"
                value={assigneeId || null}
                onValueChange={(v) => setAssigneeId(v ?? '')}
              >
                <SelectTrigger className="inset-neu w-full border-0">
                  <SelectValue placeholder="Unassigned" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="unassigned">Unassigned</SelectItem>
                  {members.map((m) => (
                    <SelectItem key={m.user_id} value={m.user_id}>
                      {m.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="task-due">Due date</Label>
              <Input
                id="task-due"
                type="date"
                className="inset-neu"
                value={dueDate}
                onChange={(e) => setDueDate(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Labels</Label>
              <div className="flex flex-wrap gap-1.5">
                {labels.length === 0 && (
                  <span className="text-xs text-muted-foreground">None in project</span>
                )}
                {labels.map((l) => {
                  const on = labelIds.includes(l.id)
                  return (
                    <button
                      key={l.id}
                      type="button"
                      aria-pressed={on}
                      onClick={() =>
                        setLabelIds((ids) => (on ? ids.filter((x) => x !== l.id) : [...ids, l.id]))
                      }
                      className="rounded-md px-1.5 py-0.5 text-xs transition-colors"
                      style={
                        on
                          ? { backgroundColor: `${l.color}33`, color: l.color, boxShadow: `0 0 0 1px ${l.color}` }
                          : { color: l.color }
                      }
                    >
                      {l.name}
                    </button>
                  )
                })}
              </div>
            </div>
          </div>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || !title.trim()}>
              {busy ? 'Creating…' : 'Create task'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
