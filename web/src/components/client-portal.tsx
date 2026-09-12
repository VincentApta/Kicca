// Client portal (#32): the whole app surface for global_role=client — submit
// tickets into linked projects, track own tickets read-only. No board, no
// team nav; assessment and other team-only fields are never rendered (the
// API doesn't even send them).
import { useEffect, useState, type FormEvent } from 'react'
import { DownloadIcon, LogOutIcon, MoonIcon, PaperclipIcon, PlusIcon, SearchIcon, SendIcon, SunIcon, TicketIcon } from 'lucide-react'
import { ATTACHMENT_ACCEPT, AttachmentThumb } from '@/components/attachments'
import { ActivityTimeline } from '@/components/activity-timeline'
import { NotificationBell } from '@/components/notifications'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import { api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { useTheme } from '@/hooks/use-theme'
import { STATUS_LABELS, SelectLabel } from '@/lib/labels'
import { useToast } from '@/lib/toast'
import type { Attachment, ClientComment, ClientProjectRef, ClientTicket, Status, TaskActivityEvent } from '@/lib/types'

// Same palette as my-tasks-page pills (light-mode readable via dark: variants).
const STATUS_COLORS: Record<string, string> = {
  inbox: 'text-muted-foreground/70 bg-muted/30 border-border/40',
  backlog: 'text-blue-700 bg-blue-100 border-blue-300 dark:text-blue-400/80 dark:bg-blue-900/15 dark:border-blue-400/25',
  in_progress: 'text-amber-700 bg-amber-100 border-amber-300 dark:text-amber-400 dark:bg-amber-900/20 dark:border-amber-400/30',
  review: 'text-violet-700 bg-violet-100 border-violet-300 dark:text-violet-400 dark:bg-violet-900/15 dark:border-violet-400/25',
  done: 'text-emerald-700 bg-emerald-100 border-emerald-300 dark:text-emerald-400 dark:bg-emerald-900/15 dark:border-emerald-400/25',
  blocked: 'text-red-700 bg-red-100 border-red-300 dark:text-red-400 dark:bg-red-900/15 dark:border-red-400/25',
  trash: 'text-muted-foreground/40 bg-muted/10 border-border/30 line-through',
}

function StatusChip({ status }: { status: Status }) {
  return (
    <span
      className={`rounded-md border px-2 py-0.5 text-xs font-semibold ${STATUS_COLORS[status] ?? STATUS_COLORS.inbox}`}
    >
      {STATUS_LABELS[status]}
    </span>
  )
}

function fmtDate(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function ClientPortal() {
  const { state, logout } = useAuth()
  const { theme, toggle } = useTheme()
  const toast = useToast()
  const me = state.phase === 'authenticated' ? state.user : null

  const [projects, setProjects] = useState<ClientProjectRef[] | null>(null)
  const projectLabels = Object.fromEntries((projects ?? []).map((p) => [p.id, `${p.name} · ${p.key}`]))
  const [tickets, setTickets] = useState<ClientTicket[] | null>(null)
  const [detail, setDetail] = useState<ClientTicket | null>(null)
  const [detailAtts, setDetailAtts] = useState<Attachment[] | null>(null)
  const [detailEvents, setDetailEvents] = useState<TaskActivityEvent[] | null>(null)
  const [detailComments, setDetailComments] = useState<ClientComment[] | null>(null)
  const [commentBody, setCommentBody] = useState('')
  const [sending, setSending] = useState(false)

  // my-tickets search + filters (#47) — server-side, same as the team list
  const [search, setSearch] = useState('')
  const [ticketsQ, setTicketsQ] = useState('') // debounced search
  const [filterProject, setFilterProject] = useState('') // '' = all linked
  const [filterStatus, setFilterStatus] = useState('') // '' | open | closed

  // submit form
  const [projectId, setProjectId] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [errors, setErrors] = useState<{ title?: string; project?: string }>({})
  const [busy, setBusy] = useState(false)

  // debounce the search box → ticketsQ
  useEffect(() => {
    const t = setTimeout(() => setTicketsQ(search), 250)
    return () => clearTimeout(t)
  }, [search])

  // filtered fetch — q, linked project, open/closed
  function fetchTickets() {
    api.clientListTickets({
      q: ticketsQ || undefined,
      project_id: filterProject || undefined,
      status: filterStatus === '' ? undefined : (filterStatus as 'open' | 'closed'),
    }).then(
      ({ data }) => setTickets(data),
      () => setTickets([]),
    )
  }

  useEffect(() => {
    fetchTickets()
  }, [ticketsQ, filterProject, filterStatus])

  useEffect(() => {
    api.clientListProjects().then(
      ({ data }) => {
        setProjects(data)
        setProjectId(data[0]?.id ?? null)
      },
      () => setProjects([]),
    )
  }, [])

  // ticket detail loads its attachments (client-created ones only — the
  // server filters to what this client may stream), activity timeline (#44)
  // and the comment thread (#43: team comments arrive with the author name only)
  useEffect(() => {
    if (!detail) {
      setDetailAtts(null)
      setDetailEvents(null)
      setDetailComments(null)
      setCommentBody('')
      return
    }
    api.clientListTicketAttachments(detail.id).then(
      ({ data }) => setDetailAtts(data),
      () => setDetailAtts([]),
    )
    api.clientTicketEvents(detail.id).then(
      ({ data }) => setDetailEvents(data),
      () => setDetailEvents([]),
    )
    api.clientListTicketComments(detail.id).then(
      ({ data }) => setDetailComments(data),
      () => setDetailComments([]),
    )
  }, [detail])

  async function submitComment() {
    if (!detail) return
    const body = commentBody.trim()
    if (!body) return
    setSending(true)
    try {
      const c = await api.clientAddTicketComment(detail.id, body)
      setDetailComments((cs) => [...(cs ?? []), c])
      setCommentBody('')
    } catch {
      toast('Could not post the comment. Try again.', 'error')
    } finally {
      setSending(false)
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const errs: typeof errors = {}
    if (!projectId) errs.project = 'Pick a project.'
    if (!title.trim()) errs.title = 'Title is required.'
    setErrors(errs)
    if (Object.keys(errs).length > 0) return
    setBusy(true)
    try {
      const ticket = await api.clientCreateTicket({
        project_id: projectId!,
        title: title.trim(),
        description: description.trim(),
      })
      // attachments ride the create: ticket exists → upload, failures skipped
      let failed = 0
      for (const f of files) {
        try {
          await api.clientUploadTicketAttachment(ticket.id, f)
        } catch {
          failed++
        }
      }
      toast(failed > 0 ? 'Ticket submitted; some attachments failed' : 'Ticket submitted — the team will pick it up from the Inbox')
      setTitle('')
      setDescription('')
      setFiles([])
      fetchTickets()
    } catch {
      setErrors({ title: 'Could not submit the ticket. Try again.' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-10 flex h-14 items-center gap-3 border-b border-border bg-background px-6">
        <span className="inset-neu flex size-8 items-center justify-center rounded-lg">
          <TicketIcon className="size-4 text-primary" strokeWidth={1.5} />
        </span>
        <span className="font-heading font-semibold text-foreground">kica</span>
        <span className="ml-2 hidden text-sm text-muted-foreground sm:inline">
          Support portal
        </span>
        <div className="ml-auto flex items-center gap-3">
          {me && (
            <span className="hidden text-sm text-muted-foreground md:inline">
              {me.name} <span className="font-mono text-xs">({me.email})</span>
            </span>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              api.clientExportTickets({
                q: ticketsQ || undefined,
                project_id: filterProject || undefined,
                status: filterStatus === '' ? undefined : (filterStatus as 'open' | 'closed'),
              })
            }
            aria-label="Export my tickets as CSV"
          >
            <DownloadIcon strokeWidth={1.5} />
            <span className="hidden sm:inline">Export</span>
          </Button>
          <NotificationBell
            onOpenTask={(taskId: string) => {
              // refetch list (may be filtered), find the ticket, open detail
              api.clientListTickets({ q: '', project_id: '', status: undefined }).then(({ data }) => {
                const t = data.find((x) => x.id === taskId)
                if (t) setDetail(t)
              }).catch(() => { /* swallow */ })
            }}
          />
          <Button
            variant="ghost"
            size="sm"
            onClick={toggle}
            aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
          >
            {theme === 'dark' ? <SunIcon strokeWidth={1.5} /> : <MoonIcon strokeWidth={1.5} />}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => void logout()}>
            <LogOutIcon strokeWidth={1.5} />
            Log out
          </Button>
        </div>
      </header>

      <main className="mx-auto grid max-w-5xl gap-6 p-6 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        {/* ---- submit form ---- */}
        <section className="card-neu flex flex-col gap-4 p-5">
          <h1 className="font-heading text-lg font-semibold text-foreground">
            New ticket
          </h1>
          <form className="flex flex-col gap-4" onSubmit={submit}>
            <div className="flex flex-col gap-2">
              <Label htmlFor="ticket-project">Project</Label>
              <Select
                id="ticket-project"
                value={projectId}
                onValueChange={(v) => setProjectId(v ?? null)}
                disabled={(projects?.length ?? 0) === 0}
              >
                <SelectTrigger className="inset-neu w-full border-0">
                  <SelectValue placeholder={projects === null ? 'Loading…' : 'Pick a project'}>
                    {projectId ? (
                      <SelectLabel value={projectId} labelMap={projectLabels} />
                    ) : null}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {(projects ?? []).map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name} · {p.key}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.project && (
                <p role="alert" className="text-xs text-destructive">
                  {errors.project}
                </p>
              )}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="ticket-title">Title</Label>
              <Input
                id="ticket-title"
                className="inset-neu"
                aria-invalid={!!errors.title}
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Short summary of the problem"
              />
              {errors.title && (
                <p role="alert" className="text-xs text-destructive">
                  {errors.title}
                </p>
              )}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="ticket-description">Description</Label>
              <Textarea
                id="ticket-description"
                className="inset-neu min-h-32"
                placeholder="What happened? What did you expect?"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="ticket-files">Attachments</Label>
              <label
                htmlFor="ticket-files"
                className="inset-neu flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-muted-foreground hover:text-foreground"
              >
                <PaperclipIcon className="size-3.5 shrink-0" strokeWidth={1.5} />
                <span className="truncate">
                  {files.length === 0 ? 'Optional images / videos' : `${files.length} file(s) selected`}
                </span>
                <input
                  id="ticket-files"
                  type="file"
                  className="sr-only"
                  accept={ATTACHMENT_ACCEPT}
                  multiple
                  disabled={busy}
                  onChange={(e) => setFiles(Array.from(e.target.files ?? []))}
                />
              </label>
            </div>
            <Button type="submit" disabled={busy || projects === null}>
              <PlusIcon strokeWidth={1.5} />
              {busy ? 'Submitting…' : 'Submit ticket'}
            </Button>
          </form>
          {projects?.length === 0 && (
            <p className="text-sm text-muted-foreground">
              No projects linked to your account yet — ask the team to link one.
            </p>
          )}
        </section>

        {/* ---- my tickets ---- */}
        <section className="card-neu flex flex-col gap-3 p-5">
          <h2 className="font-heading text-lg font-semibold text-foreground">My tickets</h2>
          {/* search + filters (#47) — server-side, mirrors the team list */}
          <div className="relative">
            <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" strokeWidth={1.5} />
            <Input
              className="inset-neu border-0 pl-8"
              placeholder="Search tickets…"
              aria-label="Search tickets"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className="flex gap-2">
            <Select
              value={filterProject || null}
              onValueChange={(v) => setFilterProject(v && v !== 'all' ? v : '')}
              disabled={(projects?.length ?? 0) === 0}
            >
              <SelectTrigger className="inset-neu w-full border-0 text-xs" aria-label="Filter by project">
                <SelectValue placeholder="All projects">
                  {filterProject ? <SelectLabel value={filterProject} labelMap={projectLabels} /> : null}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All projects</SelectItem>
                {(projects ?? []).map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name} · {p.key}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={filterStatus || null}
              onValueChange={(v) => setFilterStatus(v === 'all' ? '' : (v ?? ''))}
            >
              <SelectTrigger className="inset-neu w-40 border-0 text-xs" aria-label="Filter by status">
                <SelectValue placeholder="All statuses">
                  {filterStatus ? (
                    <span className="capitalize">{filterStatus}</span>
                  ) : null}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="open">Open</SelectItem>
                <SelectItem value="closed">Closed</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {tickets === null && (
            <div className="flex flex-col gap-2" aria-hidden>
              {[0, 1, 2].map((i) => (
                <div key={i} className="inset-neu h-12 animate-pulse rounded-lg" />
              ))}
            </div>
          )}
          {tickets?.length === 0 && (
            <p className="py-4 text-sm text-muted-foreground">
              {ticketsQ || filterProject || filterStatus
                ? 'No tickets match — adjust the search or filters.'
                : 'Nothing yet — your submitted tickets and their status show up here.'}
            </p>
          )}
          <ul className="flex flex-col gap-2">
            {(tickets ?? []).map((t) => (
              <li key={t.id}>
                <button
                  type="button"
                  onClick={() => setDetail(t)}
                  className="inset-neu flex w-full flex-col gap-1.5 rounded-lg p-3 text-left transition-shadow hover:shadow-[var(--shadow-raised)]"
                >
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs text-muted-foreground">
                      {t.project_key}-{t.number}
                    </span>
                    <StatusChip status={t.status} />
                    <span className="ml-auto font-mono text-[10px] text-muted-foreground/60">
                      {fmtDate(t.updated_at)}
                    </span>
                  </div>
                  <span className="truncate text-sm font-medium text-foreground">
                    {t.title}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      </main>

      {/* ---- read-only detail ---- */}
      <Dialog open={!!detail} onOpenChange={(o) => !o && setDetail(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{detail?.title}</DialogTitle>
            <DialogDescription>
              {detail && (
                <>
                  <span className="font-mono">
                    {detail.project_key}-{detail.number}
                  </span>{' '}
                  · submitted {fmtDate(detail.created_at)}
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          {detail && (
            <div className="flex flex-col gap-4">
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">Status</span>
                <StatusChip status={detail.status} />
              </div>
              <div className="inset-neu min-h-24 rounded-lg p-3">
                <p className="whitespace-pre-wrap text-sm text-foreground">
                  {detail.description || 'No description provided.'}
                </p>
              </div>
              <div className="flex flex-col gap-2">
                <span className="text-xs text-muted-foreground">Attachments</span>
                {detailAtts === null && (
                  <div className="inset-neu size-20 animate-pulse rounded-lg" aria-hidden />
                )}
                {detailAtts?.length === 0 && (
                  <p className="text-xs text-muted-foreground">None.</p>
                )}
                {detailAtts && detailAtts.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {detailAtts.map((a) => (
                      <AttachmentThumb key={a.id} a={a} />
                    ))}
                  </div>
                )}
              </div>
              <div className="flex flex-col gap-2">
                <span className="text-xs text-muted-foreground">Activity</span>
                {detailEvents === null && (
                  <div className="h-3 w-40 animate-pulse rounded bg-secondary" aria-hidden />
                )}
                {detailEvents?.length === 0 && (
                  <p className="text-xs text-muted-foreground">No activity yet.</p>
                )}
                {detailEvents && detailEvents.length > 0 && (
                  <ActivityTimeline events={detailEvents} />
                )}
              </div>
              {/* ---- comments thread (#43): read team questions, reply ---- */}
              <div className="flex flex-col gap-2">
                <span className="text-xs text-muted-foreground">Comments</span>
                {detailComments === null && (
                  <div className="inset-neu h-16 animate-pulse rounded-lg" aria-hidden />
                )}
                {detailComments?.length === 0 && (
                  <p className="text-xs text-muted-foreground">
                    No comments yet — the team may ask questions here.
                  </p>
                )}
                {detailComments && detailComments.length > 0 && (
                  <div className="flex flex-col gap-2">
                    {detailComments.map((c) => (
                      <div key={c.id} className="inset-neu rounded-lg p-3">
                        <p className="flex items-baseline gap-2">
                          <span className="text-xs font-medium text-foreground">{c.user.name}</span>
                          <span className="font-mono text-[10px] text-muted-foreground/60">
                            {fmtDate(c.created_at)}
                          </span>
                        </p>
                        <p className="mt-1 whitespace-pre-wrap text-sm text-foreground">{c.body}</p>
                      </div>
                    ))}
                  </div>
                )}
                <div className="flex items-end gap-2">
                  <Textarea
                    className="inset-neu min-h-10 resize-none"
                    rows={2}
                    placeholder="Write a reply…"
                    aria-label="Write a comment"
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
              </div>
              <p className="text-xs text-muted-foreground">
                Last updated {fmtDate(detail.updated_at)}
              </p>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
