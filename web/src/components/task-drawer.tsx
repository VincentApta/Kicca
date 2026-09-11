import { useEffect, useRef, useState } from 'react'
import Markdown from 'react-markdown'
import { CheckIcon, PencilIcon, SendIcon, Trash2Icon, XIcon } from 'lucide-react'
import { GithubIcon } from '@/components/github-icon'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { TaskKey } from './board/priority-dot'
import { BOARD_STATUSES, PRIORITY_ORDER, taskKey } from '@/lib/board'
import { PRIORITY_LABELS, STATUS_LABELS, SelectLabel } from '@/lib/labels'
import { ApiError, api } from '@/lib/api'
import { useToast } from '@/lib/toast'
import type {
  Comment,
  GhLink,
  Label as LabelT,
  Priority,
  ProjectMember,
  Status,
  Task,
  TaskPatch,
} from '@/lib/types'

export function TaskDrawer({
  task,
  members,
  labels,
  projectKey,
  onClose,
  onPatch,
  onTrash,
  onGhLink,
}: {
  task: Task | null
  members: ProjectMember[]
  labels: LabelT[]
  projectKey: string
  onClose: () => void
  onPatch: (id: string, patch: TaskPatch) => Promise<Task>
  onTrash: (id: string) => void
  onGhLink: (id: string, link: GhLink) => void
}) {
  return (
    <Sheet open={!!task} onOpenChange={(o) => !o && onClose()}>
      <SheetContent
        side="right"
        className="w-full gap-0 bg-card p-0 sm:max-w-none sm:w-[480px]"
        showCloseButton={false}
      >
        {task ? (
          <DrawerBody
            key={task.id}
            task={task}
            members={members}
            labels={labels}
            projectKey={projectKey}
            onClose={onClose}
            onPatch={onPatch}
            onTrash={onTrash}
            onGhLink={onGhLink}
          />
        ) : (
          <DrawerSkeleton />
        )}
      </SheetContent>
    </Sheet>
  )
}

function DrawerBody({
  task,
  members,
  labels,
  projectKey,
  onClose,
  onPatch,
  onTrash,
  onGhLink,
}: {
  task: Task
  members: ProjectMember[]
  labels: LabelT[]
  projectKey: string
  onClose: () => void
  onPatch: (id: string, patch: TaskPatch) => Promise<Task>
  onTrash: (id: string) => void
  onGhLink: (id: string, link: GhLink) => void
}) {
  const toast = useToast()
  const [title, setTitle] = useState(task.title)
  const [editingDesc, setEditingDesc] = useState(false)
  const [desc, setDesc] = useState(task.description)
  const [editingAssess, setEditingAssess] = useState(false)
  const [assess, setAssess] = useState(task.assessment)
  const [comments, setComments] = useState<Comment[] | null>(null)
  const [commentBody, setCommentBody] = useState('')
  const [sending, setSending] = useState(false)
  const [creatingGh, setCreatingGh] = useState(false)
  const titleRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    setComments(null)
    api.listComments(task.id).then(
      ({ data }) => setComments(data),
      () => setComments([]),
    )
  }, [task.id])

  async function save(patch: TaskPatch, message = 'Saved') {
    try {
      await onPatch(task.id, patch)
      toast(message)
    } catch {
      toast('Save failed', 'error')
    }
  }

  function commitTitle() {
    const t = title.trim()
    if (!t || t === task.title) {
      setTitle(task.title)
      return
    }
    setTitle(t)
    void save({ title: t }, 'Renamed')
  }

  async function submitComment() {
    const body = commentBody.trim()
    if (!body) return
    setSending(true)
    try {
      const c = await api.addComment(task.id, body)
      setComments((cs) => [...(cs ?? []), c])
      setCommentBody('')
      toast('Comment added')
    } catch {
      toast('Comment failed', 'error')
    } finally {
      setSending(false)
    }
  }

  async function createGithubIssue() {
    setCreatingGh(true)
    try {
      const link = await api.createGithubIssue(task.id)
      onGhLink(task.id, link)
      toast(`GitHub issue #${link.issue_number} created`)
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        toast('Project has no GitHub configuration', 'error')
      } else if (err instanceof ApiError && err.status === 409) {
        toast('Task already has a GitHub issue', 'error')
      } else {
        toast('Could not create GitHub issue', 'error')
      }
    } finally {
      setCreatingGh(false)
    }
  }

  const labelIds = task.labels.map((l) => l.id)

  const assigneeLabels = {
    unassigned: 'Unassigned',
    ...Object.fromEntries(members.map((m) => [m.user_id, m.name])),
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <SheetHeader className="flex-row items-center gap-3 border-b border-border py-3">
        <TaskKey taskKey={taskKey(projectKey, task)} status={task.status} />
        <SheetTitle className="sr-only">{task.title}</SheetTitle>
        <Button
          variant="ghost"
          size="icon-sm"
          className="ml-auto"
          onClick={onClose}
          aria-label="Close task"
        >
          <XIcon strokeWidth={1.5} />
        </Button>
      </SheetHeader>

      <div className="flex-1 overflow-y-auto p-5">
        {/* inline-edit title */}
        <input
          ref={titleRef}
          className="w-full rounded-md bg-transparent font-heading text-lg font-semibold text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"
          value={title}
          aria-label="Task title"
          onChange={(e) => setTitle(e.target.value)}
          onBlur={commitTitle}
          onKeyDown={(e) => e.key === 'Enter' && (e.currentTarget as HTMLInputElement).blur()}
        />

        {/* description: markdown edit + render */}
        <section className="mt-4" aria-label="Description">
          <div className="flex items-center justify-between">
            <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              Description
            </h3>
            {!editingDesc && (
              <button
                type="button"
                onClick={() => {
                  setDesc(task.description)
                  setEditingDesc(true)
                }}
                className="flex items-center gap-1 rounded-md p-1 text-xs text-muted-foreground hover:text-foreground"
              >
                <PencilIcon className="size-3" strokeWidth={1.5} /> Edit
              </button>
            )}
          </div>
          {editingDesc ? (
            <div className="mt-2 flex flex-col gap-2">
              <Textarea
                className="inset-neu min-h-40"
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder="Markdown supported"
                autoFocus
              />
              <div className="flex gap-2">
                <Button
                  size="sm"
                  onClick={() => {
                    setEditingDesc(false)
                    if (desc !== task.description) void save({ description: desc }, 'Description saved')
                  }}
                >
                  <CheckIcon strokeWidth={1.5} /> Save
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setEditingDesc(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          ) : task.description ? (
            <div className="prose-neutral mt-1 text-sm leading-relaxed text-foreground [&_a]:text-primary [&_code]:rounded [&_code]:bg-secondary [&_code]:px-1 [&_code]:font-mono [&_code]:text-xs [&_h1]:mt-3 [&_h1]:text-base [&_h2]:mt-3 [&_h2]:text-sm [&_li]:my-0.5 [&_ol]:list-decimal [&_ol]:pl-5 [&_p]:my-2 [&_pre]:bg-secondary [&_pre]:p-2 [&_pre]:font-mono [&_pre]:text-xs [&_ul]:list-disc [&_ul]:pl-5">
                <Markdown>{task.description}</Markdown>
              </div>
          ) : (
            <button
              type="button"
              onClick={() => setEditingDesc(true)}
              className="mt-1 w-full rounded-lg py-3 text-left text-sm text-muted-foreground hover:text-foreground"
            >
              Add a description…
            </button>
          )}
        </section>

        {/* assessment: triage note — becomes the GitHub issue body */}
        <section className="mt-4" aria-label="Assessment">
          <div className="flex items-center justify-between">
            <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              Assessment
            </h3>
            {!editingAssess && (
              <button
                type="button"
                onClick={() => {
                  setAssess(task.assessment)
                  setEditingAssess(true)
                }}
                className="flex items-center gap-1 rounded-md p-1 text-xs text-muted-foreground hover:text-foreground"
              >
                <PencilIcon className="size-3" strokeWidth={1.5} /> Edit
              </button>
            )}
          </div>
          <p className="mt-0.5 text-[10px] text-muted-foreground">
            Triage conclusion — used as the GitHub issue body (description kept as context) when creating an issue.
          </p>
          {editingAssess ? (
            <div className="mt-2 flex flex-col gap-2">
              <Textarea
                className="inset-neu min-h-32"
                value={assess}
                onChange={(e) => setAssess(e.target.value)}
                placeholder="Markdown supported"
                autoFocus
              />
              <div className="flex gap-2">
                <Button
                  size="sm"
                  onClick={() => {
                    setEditingAssess(false)
                    if (assess !== task.assessment) void save({ assessment: assess }, 'Assessment saved')
                  }}
                >
                  <CheckIcon strokeWidth={1.5} /> Save
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setEditingAssess(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          ) : task.assessment ? (
            <div className="prose-neutral mt-1 text-sm leading-relaxed text-foreground [&_a]:text-primary [&_code]:rounded [&_code]:bg-secondary [&_code]:px-1 [&_code]:font-mono [&_code]:text-xs [&_h1]:mt-3 [&_h1]:text-base [&_h2]:mt-3 [&_h2]:text-sm [&_li]:my-0.5 [&_ol]:list-decimal [&_ol]:pl-5 [&_p]:my-2 [&_pre]:bg-secondary [&_pre]:p-2 [&_pre]:font-mono [&_pre]:text-xs [&_ul]:list-disc [&_ul]:pl-5">
              <Markdown>{task.assessment}</Markdown>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setEditingAssess(true)}
              className="mt-1 w-full rounded-lg py-3 text-left text-sm text-muted-foreground hover:text-foreground"
            >
              Add an assessment…
            </button>
          )}
        </section>

        {/* fields grid */}
        <section className="mt-6 grid grid-cols-2 gap-4" aria-label="Task fields">
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs text-muted-foreground">Status</Label>
            <Select
              value={task.status}
              onValueChange={(v) => v && save({ status: v as Status }, `Moved to ${STATUS_LABELS[v as Status]}`)}
            >
              <SelectTrigger className="inset-neu w-full border-0">
                <SelectValue>
                  <SelectLabel value={task.status} labelMap={STATUS_LABELS} />
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {BOARD_STATUSES.map((s) => (
                  <SelectItem key={s} value={s}>
                    {STATUS_LABELS[s]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs text-muted-foreground">Priority</Label>
            <Select
              value={task.priority}
              onValueChange={(v) => save({ priority: v as Priority }, 'Priority updated')}
            >
              <SelectTrigger className="inset-neu w-full border-0">
                <SelectValue>
                  <SelectLabel value={task.priority} labelMap={PRIORITY_LABELS} />
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {PRIORITY_ORDER.map((p) => (
                  <SelectItem key={p} value={p}>
                    {PRIORITY_LABELS[p]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs text-muted-foreground">Assignee</Label>
            <Select
              value={task.assignee?.id ?? null}
              onValueChange={(v) =>
                save(
                  { assignee_id: !v || v === 'unassigned' ? null : v },
                  v && v !== 'unassigned' ? 'Assigned' : 'Unassigned',
                )
              }
            >
              <SelectTrigger className="inset-neu w-full border-0">
                <SelectValue placeholder="Unassigned">
                  {task.assignee ? (
                    <SelectLabel value={task.assignee.id} labelMap={assigneeLabels} />
                  ) : null}
                </SelectValue>
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
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs text-muted-foreground" htmlFor="due">
              Due date
            </Label>
            <Input
              id="due"
              type="date"
              className="inset-neu"
              value={task.due_date ?? ''}
              onChange={(e) => save({ due_date: e.target.value || null }, 'Due date updated')}
            />
          </div>
          <div className="col-span-2 flex flex-col gap-1.5">
            <Label className="text-xs text-muted-foreground">Labels</Label>
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
                    onClick={() => {
                      const next = on
                        ? task.labels.filter((x) => x.id !== l.id).map((x) => x.id)
                        : [...labelIds, l.id]
                      void save({ label_ids: next }, 'Labels updated')
                    }}
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
        </section>

        {/* GH link — create when unlinked, badge links to the issue */}
        <section className="mt-6" aria-label="GitHub">
          {task.gh_link ? (
            <a
              href={task.gh_link.issue_url}
              target="_blank"
              rel="noreferrer"
              className="card-neu flex items-center gap-2 p-3 text-sm text-foreground hover:text-primary"
            >
              <GithubIcon className="size-4" strokeWidth={1.5} />
              <span className="font-mono text-xs">
                {task.gh_link.repo} #{task.gh_link.issue_number}
              </span>
            </a>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              className="text-muted-foreground"
              disabled={creatingGh}
              onClick={() => void createGithubIssue()}
            >
              <GithubIcon strokeWidth={1.5} />
              {creatingGh ? 'Creating…' : 'Create GitHub issue'}
            </Button>
          )}
        </section>

        {/* comments */}
        <section className="mt-6" aria-label="Comments">
          <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
            Comments
          </h3>
          <div className="mt-2 flex flex-col gap-3">
            {comments === null && <CommentSkeletons />}
            {comments?.length === 0 && (
              <p className="text-xs text-muted-foreground">No comments yet.</p>
            )}
            {comments?.map((c) => {
              const author = members.find((m) => m.user_id === c.user_id)
              return (
                <div key={c.id} className="card-neu p-3">
                  <p className="flex items-baseline gap-2">
                    <span className="text-xs font-medium text-foreground">
                      {author?.name ?? 'User'}
                    </span>
                    <span className="font-mono text-xs text-muted-foreground">
                      {new Date(c.created_at).toLocaleDateString()}
                    </span>
                  </p>
                  <p className="mt-1 text-sm whitespace-pre-wrap text-foreground">{c.body}</p>
                </div>
              )
            })}
          </div>
          <div className="mt-3 flex items-end gap-2">
            <Textarea
              className="inset-neu min-h-10 resize-none"
              rows={2}
              placeholder="Write a comment…"
              value={commentBody}
              onChange={(e) => setCommentBody(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) void submitComment()
              }}
            />
            <Button
              size="icon-sm"
              aria-label="Add comment"
              disabled={sending || !commentBody.trim()}
              onClick={() => void submitComment()}
            >
              <SendIcon strokeWidth={1.5} />
            </Button>
          </div>
        </section>
      </div>

      <footer className="flex items-center justify-between border-t border-border p-4">
        <span className="font-mono text-xs text-muted-foreground">
          {taskKey(projectKey, task)} · updated {new Date(task.updated_at).toLocaleDateString()}
        </span>
        <Button variant="destructive" size="sm" onClick={() => onTrash(task.id)}>
          <Trash2Icon strokeWidth={1.5} />
          Trash
        </Button>
      </footer>
    </div>
  )
}

function CommentSkeletons() {
  return (
    <>
      {[0, 1].map((i) => (
        <div key={i} className="card-neu p-3">
          <div className="h-3 w-24 animate-pulse rounded bg-secondary" />
          <div className="mt-2 h-3 w-full animate-pulse rounded bg-secondary" />
        </div>
      ))}
    </>
  )
}

function DrawerSkeleton() {
  return (
    <div className="flex h-full flex-col gap-4 p-5">
      <div className="h-4 w-20 animate-pulse rounded bg-secondary" />
      <div className="h-7 w-3/4 animate-pulse rounded bg-secondary" />
      <div className="h-3 w-full animate-pulse rounded bg-secondary" />
      <div className="h-3 w-2/3 animate-pulse rounded bg-secondary" />
      <div className="mt-4 grid grid-cols-2 gap-4">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="h-9 animate-pulse rounded-lg bg-secondary" />
        ))}
      </div>
    </div>
  )
}
