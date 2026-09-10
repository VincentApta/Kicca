// Admin surface: Users + Teams (global admin nav) and Project settings
// (admin/project_admin, reached from the sidebar project context).
import { useEffect, useState, type FormEvent } from 'react'
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  PencilIcon,
  PlusIcon,
  SearchIcon,
  TagIcon,
  Trash2Icon,
  UserPlusIcon,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { GithubIcon } from '@/components/github-icon'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { ApiError, api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { SelectLabel } from '@/lib/labels'
import { useToast } from '@/lib/toast'
import {
  canChangeMemberRole,
  canRemoveMember,
  toMemberPayload,
  validateLabel,
  validateProjectGeneral,
  validateProjectGithub,
  validateTeamName,
  validateUserCreate,
  validateUserEdit,
  type FieldErrors,
} from '@/lib/admin'
import type {
  GlobalRole,
  Label as LabelT,
  Paginated,
  Project,
  ProjectDetail,
  ProjectMember,
  ProjectRole,
  Team,
  User,
  UserPatch,
} from '@/lib/types'

function PageShell({
  title,
  description,
  actions,
  children,
}: {
  title: string
  description?: string
  actions?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <div className="flex-1 overflow-auto p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="font-heading text-2xl font-semibold text-foreground">{title}</h1>
          {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions}
      </div>
      <div className="card-neu mt-6 p-4">{children}</div>
    </div>
  )
}

function Field({
  label,
  htmlFor,
  error,
  children,
}: {
  label: string
  htmlFor?: string
  error?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {error && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}

function RowSkeletons({ n, cols }: { n: number; cols: number }) {
  return (
    <div className="flex flex-col gap-3" aria-hidden>
      {Array.from({ length: n }, (_, r) => (
        <div key={r} className="flex gap-6">
          {Array.from({ length: cols }, (_, c) => (
            <div
              key={c}
              className="h-3.5 animate-pulse rounded bg-secondary"
              style={{ width: c === 0 ? 140 : 100 }}
            />
          ))}
        </div>
      ))}
    </div>
  )
}

// Users (GET /users) paginate 50/page — member dialogs need everyone.
// ponytail: caps at 10 pages (500 users); add a proper picker when exceeded.
async function loadAllUsers(): Promise<User[]> {
  const out: User[] = []
  for (let page = 1; page <= 10; page++) {
    const res = await api.listUsers(page)
    out.push(...res.data)
    if (out.length >= res.total || res.data.length === 0) break
  }
  return out
}

// Trigger display maps — Select.Value would render the raw stored value.
const GLOBAL_ROLE_LABELS: Record<string, string> = { member: 'Member', admin: 'Admin' }
const ACCOUNT_STATUS_LABELS: Record<string, string> = { active: 'Active', disabled: 'Disabled' }
const PROJECT_ROLE_LABELS: Record<string, string> = { project_admin: 'Project admin', member: 'Member' }

// ---------------------------------------------------------------- Users page

export function UsersPage() {
  const { state } = useAuth()
  const me = state.phase === 'authenticated' ? state.user : null

  const [page, setPage] = useState(1)
  const [res, setRes] = useState<Paginated<User> | null>(null)
  const [error, setError] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<User | null>(null)
  // PATCH /users/:id accepts `disabled` but the wire `user` shape never
  // reports it back — track toggles locally for the Status column.
  const [disabledIds, setDisabledIds] = useState<ReadonlySet<string>>(new Set())

  function load(p: number) {
    setRes(null)
    api.listUsers(p).then(setRes, () => setError(true))
  }

  useEffect(() => {
    load(page)
  }, [page])

  const pages = res ? Math.max(1, Math.ceil(res.total / res.per_page)) : 1

  return (
    <PageShell
      title="Users"
      description={res ? `${res.total} user${res.total === 1 ? '' : 's'} in the workspace` : undefined}
      actions={
        <Button onClick={() => setCreateOpen(true)}>
          <UserPlusIcon strokeWidth={1.5} />
          New user
        </Button>
      }
    >
      {error && <p className="text-sm text-destructive">Could not load users.</p>}
      {!error && res === null && <RowSkeletons n={5} cols={4} />}
      {res && res.data.length === 0 && (
        <p className="py-4 text-sm text-muted-foreground">No users.</p>
      )}
      {res && res.data.length > 0 && (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-16" aria-label="Actions" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {res.data.map((u) => {
                const disabled = disabledIds.has(u.id)
                return (
                  <TableRow key={u.id}>
                    <TableCell className="font-medium">{u.name}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">{u.email}</TableCell>
                    <TableCell className="text-xs">
                      {u.global_role === 'admin' ? 'Admin' : 'Member'}
                    </TableCell>
                    <TableCell className="text-xs">
                      <span className={disabled ? 'text-status-blocked' : 'text-status-done'}>
                        {disabled ? 'Disabled' : 'Active'}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={`Edit ${u.name}`}
                        onClick={() => setEditing(u)}
                      >
                        <PencilIcon strokeWidth={1.5} />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
          <div className="mt-4 flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">
              Page {res.page} of {pages}
            </span>
            <div className="flex gap-2">
              <Button
                variant="ghost"
                size="sm"
                disabled={res.page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                <ChevronLeftIcon strokeWidth={1.5} />
                Prev
              </Button>
              <Button
                variant="ghost"
                size="sm"
                disabled={res.page >= pages}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
                <ChevronRightIcon strokeWidth={1.5} />
              </Button>
            </div>
          </div>
        </>
      )}

      <CreateUserDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => {
          setCreateOpen(false)
          if (page === 1) load(1)
          else setPage(1)
        }}
      />
      <EditUserDialog
        user={editing}
        currentlyDisabled={editing ? disabledIds.has(editing.id) : false}
        isSelf={!!editing && !!me && editing.id === me.id}
        onClose={() => setEditing(null)}
        onSaved={(updated, nowDisabled) => {
          setRes((r) =>
            r
              ? {
                  ...r,
                  data: r.data.map((u) =>
                    u.id === updated.id
                      ? { ...u, name: updated.name, global_role: updated.global_role }
                      : u,
                  ),
                }
              : r,
          )
          setDisabledIds((ids) => {
            const next = new Set(ids)
            if (nowDisabled) next.add(updated.id)
            else next.delete(updated.id)
            return next
          })
          setEditing(null)
        }}
      />
    </PageShell>
  )
}

function CreateUserDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: () => void
}) {
  const toast = useToast()
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<GlobalRole>('member')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (open) {
      setEmail('')
      setName('')
      setPassword('')
      setRole('member')
      setErrors({})
    }
  }, [open])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const v = validateUserCreate({ email, name, password })
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      await api.createUser({ email: email.trim(), name: name.trim(), password, global_role: role })
      toast('User created')
      onCreated()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setErrors({ email: 'A user with this email already exists.' })
      } else {
        setErrors({ form: 'Could not create user.' })
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>New user</DialogTitle>
          <DialogDescription>Adds a teammate with a temporary password.</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <Field label="Email" htmlFor="new-user-email" error={errors.email}>
            <Input
              id="new-user-email"
              type="email"
              className="inset-neu"
              aria-invalid={!!errors.email}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoFocus
            />
          </Field>
          <Field label="Name" htmlFor="new-user-name" error={errors.name}>
            <Input
              id="new-user-name"
              className="inset-neu"
              aria-invalid={!!errors.name}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Field label="Password" htmlFor="new-user-password" error={errors.password}>
            <Input
              id="new-user-password"
              type="password"
              className="inset-neu"
              aria-invalid={!!errors.password}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          <Field label="Global role" htmlFor="new-user-role">
            <Select id="new-user-role" value={role} onValueChange={(v) => setRole(v as GlobalRole)}>
              <SelectTrigger className="inset-neu w-full border-0">
                <SelectValue>
                  <SelectLabel value={role} labelMap={GLOBAL_ROLE_LABELS} />
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="member">Member</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          {errors.form && (
            <p role="alert" className="text-sm text-destructive">
              {errors.form}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy ? 'Creating…' : 'Create user'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function EditUserDialog({
  user,
  currentlyDisabled,
  isSelf,
  onClose,
  onSaved,
}: {
  user: User | null
  currentlyDisabled: boolean
  isSelf: boolean
  onClose: () => void
  onSaved: (updated: User, nowDisabled: boolean) => void
}) {
  const toast = useToast()
  const [name, setName] = useState('')
  const [role, setRole] = useState<GlobalRole>('member')
  const [status, setStatus] = useState<'active' | 'disabled'>('active')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (user) {
      setName(user.name)
      setRole(user.global_role)
      setStatus(currentlyDisabled ? 'disabled' : 'active')
      setPassword('')
      setErrors({})
    }
  }, [user, currentlyDisabled])

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!user) return
    const v = validateUserEdit(name, password)
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      const patch: UserPatch = { name: name.trim(), global_role: role }
      if (password) patch.password = password
      const nextDisabled = status === 'disabled'
      if (nextDisabled !== currentlyDisabled) patch.disabled = nextDisabled
      const updated = await api.patchUser(user.id, patch)
      toast('User updated')
      onSaved(updated, nextDisabled)
    } catch {
      setErrors({ form: 'Could not update user.' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={!!user} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Edit user</DialogTitle>
          <DialogDescription>{user?.email}</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <Field label="Name" htmlFor="edit-user-name" error={errors.name}>
            <Input
              id="edit-user-name"
              className="inset-neu"
              aria-invalid={!!errors.name}
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </Field>
          <Field label="Global role" htmlFor="edit-user-role">
            <Select id="edit-user-role" value={role} onValueChange={(v) => setRole(v as GlobalRole)}>
              <SelectTrigger className="inset-neu w-full border-0">
                <SelectValue>
                  <SelectLabel value={role} labelMap={GLOBAL_ROLE_LABELS} />
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="member">Member</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field
            label="Status"
            htmlFor="edit-user-status"
            error={isSelf ? 'You cannot disable your own account.' : undefined}
          >
            <Select
              id="edit-user-status"
              value={status}
              onValueChange={(v) => setStatus(v as 'active' | 'disabled')}
              disabled={isSelf}
            >
              <SelectTrigger className="inset-neu w-full border-0">
                <SelectValue>
                  <SelectLabel value={status} labelMap={ACCOUNT_STATUS_LABELS} />
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="active">Active</SelectItem>
                <SelectItem value="disabled">Disabled</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field label="New password" htmlFor="edit-user-password" error={errors.password}>
            <Input
              id="edit-user-password"
              type="password"
              className="inset-neu"
              placeholder="Leave blank to keep current"
              aria-invalid={!!errors.password}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          {errors.form && (
            <p role="alert" className="text-sm text-destructive">
              {errors.form}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy ? 'Saving…' : 'Save changes'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ---------------------------------------------------------------- Teams page

export function TeamsPage() {
  const toast = useToast()
  const [teams, setTeams] = useState<Team[] | null>(null)
  const [error, setError] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<Team | null>(null)
  const [managing, setManaging] = useState<Team | null>(null)

  useEffect(() => {
    api.listTeams().then(
      ({ data }) => setTeams(data),
      () => setError(true),
    )
  }, [])

  function removeTeam(team: Team) {
    api.deleteTeam(team.id).then(
      () => {
        setTeams((ts) => ts?.filter((t) => t.id !== team.id) ?? ts)
        toast('Team deleted')
      },
      (err) => {
        if (err instanceof ApiError && err.status === 409) {
          toast('Team still has projects — move or delete them first', 'error')
        } else {
          toast('Delete failed', 'error')
        }
      },
    )
  }

  return (
    <PageShell
      title="Teams"
      description="Group users; projects belong to a team."
      actions={
        <Button onClick={() => setCreateOpen(true)}>
          <PlusIcon strokeWidth={1.5} />
          New team
        </Button>
      }
    >
      {error && <p className="text-sm text-destructive">Could not load teams.</p>}
      {!error && teams === null && <RowSkeletons n={4} cols={3} />}
      {teams && teams.length === 0 && (
        <p className="py-4 text-sm text-muted-foreground">No teams yet.</p>
      )}
      {teams && teams.length > 0 && (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {teams.map((t) => (
            <div key={t.id} className="card-neu flex flex-col gap-3 p-4">
              <div className="flex items-baseline justify-between gap-2">
                <h2 className="font-medium text-foreground">{t.name}</h2>
                <Badge variant="secondary">
                  {t.member_count ?? 0} member{t.member_count === 1 ? '' : 's'}
                </Badge>
              </div>
              <p className="font-mono text-xs text-muted-foreground">{t.slug}</p>
              <div className="mt-auto flex gap-2">
                <Button size="sm" variant="ghost" onClick={() => setManaging(t)}>
                  <UserPlusIcon strokeWidth={1.5} />
                  Members
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setEditing(t)}>
                  <PencilIcon strokeWidth={1.5} />
                  Edit
                </Button>
                <Button
                  size="sm"
                  variant="destructive"
                  className="ml-auto"
                  aria-label={`Delete ${t.name}`}
                  onClick={() => removeTeam(t)}
                >
                  <Trash2Icon strokeWidth={1.5} />
                  Delete
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <TeamDialog
        open={createOpen}
        team={null}
        onClose={() => setCreateOpen(false)}
        onSaved={(saved) => {
          setCreateOpen(false)
          setTeams((ts) => (ts ? [...ts, saved] : ts))
        }}
      />
      <TeamDialog
        open={!!editing}
        team={editing}
        onClose={() => setEditing(null)}
        onSaved={(saved) => {
          setEditing(null)
          setTeams((ts) => (ts ? ts.map((t) => (t.id === saved.id ? saved : t)) : ts))
        }}
      />
      <TeamMembersDialog
        team={managing}
        onClose={() => setManaging(null)}
        onSaved={(saved) => {
          setTeams((ts) => (ts ? ts.map((t) => (t.id === saved.id ? saved : t)) : ts))
        }}
      />
    </PageShell>
  )
}

function TeamDialog({
  open,
  team,
  onClose,
  onSaved,
}: {
  open: boolean
  team: Team | null
  onClose: () => void
  onSaved: (saved: Team) => void
}) {
  const toast = useToast()
  const [name, setName] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (open) {
      setName(team?.name ?? '')
      setErrors({})
    }
  }, [open, team])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const v = validateTeamName(name)
    setErrors(v)
    if (v.name) return
    setBusy(true)
    try {
      const saved = team ? await api.patchTeam(team.id, name.trim()) : await api.createTeam(name.trim())
      toast(team ? 'Team renamed' : 'Team created')
      onSaved(saved)
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setErrors({ name: 'A team with this name already exists.' })
      } else {
        setErrors({ form: team ? 'Could not update team.' : 'Could not create team.' })
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{team ? 'Edit team' : 'New team'}</DialogTitle>
          {team ? (
            <DialogDescription>
              Slug <span className="font-mono">{team.slug}</span> stays stable across renames.
            </DialogDescription>
          ) : (
            <DialogDescription>Slug is generated from the name.</DialogDescription>
          )}
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <Field label="Name" htmlFor="team-name" error={errors.name}>
            <Input
              id="team-name"
              className="inset-neu"
              aria-invalid={!!errors.name}
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </Field>
          {errors.form && (
            <p role="alert" className="text-sm text-destructive">
              {errors.form}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy ? 'Saving…' : team ? 'Save changes' : 'Create team'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function TeamMembersDialog({
  team,
  onClose,
  onSaved,
}: {
  team: Team | null
  onClose: () => void
  onSaved: (saved: Team) => void
}) {
  const toast = useToast()
  const [users, setUsers] = useState<User[] | null>(null)
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  const [query, setQuery] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (team) {
      setUsers(null)
      setSelected(new Set())
      setQuery('')
      loadAllUsers().then(setUsers, () => setUsers([]))
    }
  }, [team])

  function toggle(id: string) {
    setSelected((s) => {
      const next = new Set(s)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function save() {
    if (!team) return
    setBusy(true)
    try {
      const saved = await api.replaceTeamMembers(team.id, [...selected])
      toast('Members updated')
      onSaved(saved)
      onClose()
    } catch {
      toast('Save failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  const q = query.trim().toLowerCase()
  const filtered = (users ?? []).filter(
    (u) => !q || u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q),
  )

  return (
    <Dialog open={!!team} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Team members — {team?.name}</DialogTitle>
          {/* ponytail: GET /teams/:id/members doesn't exist, so current
              membership can't be preselected — saving replaces it wholesale. */}
          <DialogDescription>
            Saving replaces the team's whole membership with the selection.
          </DialogDescription>
        </DialogHeader>
        <div className="relative">
          <SearchIcon
            className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
            strokeWidth={1.5}
          />
          <Input
            className="inset-neu border-0 pl-8"
            placeholder="Search users…"
            aria-label="Search users"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <div className="inset-neu flex max-h-64 flex-col gap-0.5 overflow-y-auto p-1.5">
          {users === null && <RowSkeletons n={5} cols={2} />}
          {users?.length === 0 && (
            <p className="p-2 text-sm text-muted-foreground">No users found.</p>
          )}
          {filtered.map((u) => (
            <label
              key={u.id}
              className="flex cursor-pointer items-center gap-3 rounded-lg px-2 py-1.5 hover:bg-accent"
            >
              <input
                type="checkbox"
                className="size-4 accent-[var(--primary)]"
                checked={selected.has(u.id)}
                onChange={() => toggle(u.id)}
              />
              <span className="flex-1 truncate text-sm text-foreground">{u.name}</span>
              <span className="truncate font-mono text-xs text-muted-foreground">{u.email}</span>
            </label>
          ))}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={() => void save()} disabled={busy || users === null}>
            {busy ? 'Saving…' : `Save ${selected.size} member${selected.size === 1 ? '' : 's'}`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ------------------------------------------------ First project bootstrap

// Chicken-and-egg guard (issue #16): with zero projects no board exists to
// create anything from, so the admin empty state opens this directly.
// Creates the team too when none exists yet.
export function FirstProjectDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (project: Project) => void
}) {
  const toast = useToast()
  const [teams, setTeams] = useState<Team[] | null>(null)
  const [teamId, setTeamId] = useState<string | null>(null) // '__new__' = create team
  const [teamName, setTeamName] = useState('')
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (open) {
      setTeams(null)
      setTeamName('')
      setName('')
      setKey('')
      setErrors({})
      api.listTeams().then(
        ({ data }) => {
          setTeams(data)
          setTeamId(data.length > 0 ? data[0].id : '__new__')
        },
        () => setTeams([]),
      )
    }
  }, [open])

  const teamLabels = {
    __new__: 'New team…',
    ...Object.fromEntries((teams ?? []).map((t) => [t.id, t.name])),
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const creatingTeam = teamId === '__new__'
    const v = validateProjectGeneral({ name, key })
    const teamErr = creatingTeam ? validateTeamName(teamName).name : undefined
    if (teamErr) v.team = teamErr
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      const team = creatingTeam ? await api.createTeam(teamName.trim()) : null
      const project = await api.createProject({
        team_id: team ? team.id : teamId!,
        name: name.trim(),
        key: key.trim(),
      })
      toast('Project created')
      onCreated(project)
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setErrors({ key: 'A project with this key already exists.' })
      } else {
        setErrors({ form: 'Could not create project.' })
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create the first project</DialogTitle>
          <DialogDescription>
            Projects belong to a team — a new one is created if none exists yet.
          </DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          {teams === null ? (
            <RowSkeletons n={1} cols={1} />
          ) : teams.length > 0 ? (
            <Field label="Team" htmlFor="first-project-team">
              <Select
                id="first-project-team"
                value={teamId}
                onValueChange={(v) => setTeamId(v ?? '__new__')}
              >
                <SelectTrigger className="inset-neu w-full border-0">
                  <SelectValue>
                    <SelectLabel value={teamId} labelMap={teamLabels} />
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {teams.map((t) => (
                    <SelectItem key={t.id} value={t.id}>
                      {t.name}
                    </SelectItem>
                  ))}
                  <SelectItem value="__new__">New team…</SelectItem>
                </SelectContent>
              </Select>
            </Field>
          ) : (
            <Field label="Team" htmlFor="first-project-team" error={errors.team}>
              <Input
                id="first-project-team"
                className="inset-neu"
                aria-invalid={!!errors.team}
                value={teamName}
                onChange={(e) => setTeamName(e.target.value)}
                autoFocus
              />
            </Field>
          )}
          {teamId === '__new__' && (teams?.length ?? 0) > 0 && (
            <Field label="New team name" htmlFor="first-project-team-name" error={errors.team}>
              <Input
                id="first-project-team-name"
                className="inset-neu"
                aria-invalid={!!errors.team}
                value={teamName}
                onChange={(e) => setTeamName(e.target.value)}
              />
            </Field>
          )}
          <Field label="Project name" htmlFor="first-project-name" error={errors.name}>
            <Input
              id="first-project-name"
              className="inset-neu"
              aria-invalid={!!errors.name}
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </Field>
          <Field label="Key" htmlFor="first-project-key" error={errors.key}>
            <Input
              id="first-project-key"
              className="inset-neu font-mono uppercase"
              aria-invalid={!!errors.key}
              value={key}
              onChange={(e) => setKey(e.target.value)}
              maxLength={10}
            />
          </Field>
          <p className="-mt-2 text-xs text-muted-foreground">
            Task prefix — <span className="font-mono">{key || 'KEY'}-123</span>.
          </p>
          {errors.form && (
            <p role="alert" className="text-sm text-destructive">
              {errors.form}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy ? 'Creating…' : 'Create project'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// -------------------------------------------------------- Project settings

type SettingsTab = 'general' | 'members' | 'labels' | 'github'

const SETTINGS_TABS: { id: SettingsTab; label: string }[] = [
  { id: 'general', label: 'General' },
  { id: 'members', label: 'Members' },
  { id: 'labels', label: 'Labels' },
  { id: 'github', label: 'GitHub' },
]

export function ProjectSettingsPage({
  project,
  detail,
  labels,
  canManage,
  onRefresh,
}: {
  project: Project
  detail: ProjectDetail | null
  labels: LabelT[]
  canManage: boolean
  onRefresh: () => void
}) {
  const [tab, setTab] = useState<SettingsTab>('general')

  return (
    <div className="flex-1 overflow-auto p-6">
      <h1 className="font-heading text-2xl font-semibold text-foreground">Project settings</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        {project.name} · <span className="font-mono text-xs">{project.key}</span>
      </p>

      <div className="inset-neu mt-6 inline-flex items-center gap-0.5 rounded-lg p-0.5" role="group" aria-label="Settings sections">
        {SETTINGS_TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            aria-pressed={tab === t.id}
            onClick={() => setTab(t.id)}
            className={`flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium ${
              tab === t.id ? 'bg-card text-primary shadow-[var(--shadow-raised)]' : 'text-muted-foreground'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div className="card-neu mt-4 max-w-3xl p-4">
        {!canManage && (
          <p className="text-sm text-muted-foreground">
            You need the project admin role to manage this project.
          </p>
        )}
        {canManage && tab === 'general' && (
          <GeneralTab key={detail?.id ?? 'loading'} detail={detail} onRefresh={onRefresh} />
        )}
        {canManage && tab === 'members' && detail && (
          <MembersTab key={detail.id} detail={detail} onRefresh={onRefresh} />
        )}
        {canManage && tab === 'labels' && (
          <LabelsTab projectId={project.id} labels={labels} onRefresh={onRefresh} />
        )}
        {canManage && tab === 'github' && (
          <GithubTab key={detail?.id ?? 'loading'} detail={detail} onRefresh={onRefresh} />
        )}
      </div>
    </div>
  )
}

function GeneralTab({
  detail,
  onRefresh,
}: {
  detail: ProjectDetail | null
  onRefresh: () => void
}) {
  const toast = useToast()
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  const [description, setDescription] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (detail) {
      setName(detail.name)
      setKey(detail.key)
      setDescription(detail.description)
      setErrors({})
    }
  }, [detail])

  // PATCH /projects/:id is global-admin-only server-side; project admins
  // get read-only fields instead of a form that 403s on save.
  const editable = detail?.my_role === 'admin'

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!detail || !editable) return
    const v = validateProjectGeneral({ name, key })
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      await api.patchProject(detail.id, {
        name: name.trim(),
        key: key.trim(),
        description: description.trim(),
      })
      toast('Project updated')
      onRefresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setErrors({ key: 'A project with this key already exists.' })
      } else {
        setErrors({ form: 'Could not update project.' })
      }
    } finally {
      setBusy(false)
    }
  }

  if (!detail) return <RowSkeletons n={4} cols={2} />

  return (
    <form className="flex max-w-lg flex-col gap-4" onSubmit={submit}>
      {!editable && (
        <p className="text-sm text-muted-foreground">
          Only a global admin can edit project details.
        </p>
      )}
      <Field label="Name" htmlFor="project-name" error={errors.name}>
        <Input
          id="project-name"
          className="inset-neu"
          aria-invalid={!!errors.name}
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={!editable}
        />
      </Field>
      <Field
        label="Key"
        htmlFor="project-key"
        error={errors.key}
      >
        <Input
          id="project-key"
          className="inset-neu font-mono uppercase"
          aria-invalid={!!errors.key}
          value={key}
          onChange={(e) => setKey(e.target.value)}
          disabled={!editable}
          maxLength={10}
        />
      </Field>
      <p className="-mt-2 text-xs text-muted-foreground">
        Task prefix — <span className="font-mono">{key || 'KEY'}-123</span>. Changing it re-keys the board.
      </p>
      <Field label="Description" htmlFor="project-description">
        <Textarea
          id="project-description"
          className="inset-neu min-h-24"
          placeholder="Markdown supported"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          disabled={!editable}
        />
      </Field>
      {errors.form && (
        <p role="alert" className="text-sm text-destructive">
          {errors.form}
        </p>
      )}
      {editable && (
        <div className="flex justify-end">
          <Button type="submit" disabled={busy}>
            {busy ? 'Saving…' : 'Save changes'}
          </Button>
        </div>
      )}
    </form>
  )
}

function MembersTab({ detail, onRefresh }: { detail: ProjectDetail; onRefresh: () => void }) {
  const toast = useToast()
  const { state } = useAuth()
  const me = state.phase === 'authenticated' ? state.user : null
  // Adding members needs the user list — global admin only (GET /users).
  const canAdd = me?.global_role === 'admin'

  const [draft, setDraft] = useState<ProjectMember[]>(detail.members)
  const [candidates, setCandidates] = useState<User[] | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (canAdd && candidates === null) {
      loadAllUsers().then(setCandidates, () => setCandidates([]))
    }
  }, [canAdd, candidates])

  function setRole(userId: string, role: ProjectRole) {
    setDraft((ms) => ms.map((m) => (m.user_id === userId ? { ...m, role } : m)))
  }

  function addMember(u: User) {
    setDraft((ms) =>
      ms.some((m) => m.user_id === u.id)
        ? ms
        : [...ms, { user_id: u.id, role: 'member' as const, name: u.name, email: u.email }],
    )
  }

  async function save() {
    setBusy(true)
    try {
      await api.replaceProjectMembers(detail.id, toMemberPayload(draft))
      toast('Members updated')
      onRefresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        toast(err.message, 'error') // server-side last-admin / validation guard
      } else {
        toast('Save failed', 'error')
      }
    } finally {
      setBusy(false)
    }
  }

  const addable = (candidates ?? []).filter((u) => !draft.some((m) => m.user_id === u.id))

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      {canAdd && (
        <Field label="Add member" htmlFor="add-project-member">
          <Select
            id="add-project-member"
            value={null}
            onValueChange={(v) => {
              const u = candidates?.find((c) => c.id === v)
              if (u) addMember(u)
            }}
          >
            <SelectTrigger className="inset-neu w-full border-0">
              <SelectValue placeholder={candidates === null ? 'Loading users…' : 'Pick a user'} />
            </SelectTrigger>
            <SelectContent>
              {addable.length === 0 && (
                <div className="px-2 py-1 text-xs text-muted-foreground">Everyone is a member</div>
              )}
              {addable.map((u) => (
                <SelectItem key={u.id} value={u.id}>
                  {u.name} · {u.email}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
      )}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Email</TableHead>
            <TableHead>Role</TableHead>
            <TableHead className="w-16" aria-label="Actions" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {draft.map((m) => {
            const guarded = !canChangeMemberRole(draft, m.user_id)
            return (
              <TableRow key={m.user_id}>
                <TableCell className="font-medium">{m.name}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{m.email}</TableCell>
                <TableCell>
                  <Select
                    value={m.role === 'admin' ? 'member' : m.role}
                    onValueChange={(v) => setRole(m.user_id, v as ProjectRole)}
                    disabled={guarded}
                  >
                    <SelectTrigger
                      size="sm"
                      className="inset-neu border-0 text-xs"
                      aria-label={`Role for ${m.name}`}
                    >
                      <SelectValue>
                        <SelectLabel
                          value={m.role === 'admin' ? 'member' : m.role}
                          labelMap={PROJECT_ROLE_LABELS}
                        />
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="project_admin">Project admin</SelectItem>
                      <SelectItem value="member">Member</SelectItem>
                    </SelectContent>
                  </Select>
                  {guarded && (
                    <span className="ml-2 text-xs text-muted-foreground">last project admin</span>
                  )}
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Remove ${m.name}`}
                    disabled={!canRemoveMember(draft, m.user_id)}
                    onClick={() =>
                      setDraft((ms) => ms.filter((x) => x.user_id !== m.user_id))
                    }
                  >
                    <Trash2Icon strokeWidth={1.5} />
                  </Button>
                </TableCell>
              </TableRow>
            )
          })}
          {draft.length === 0 && (
            <TableRow>
              <TableCell colSpan={4} className="text-sm text-muted-foreground">
                No members — save needs at least one project admin.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      <div className="flex justify-end">
        <Button disabled={busy} onClick={() => void save()}>
          {busy ? 'Saving…' : 'Save members'}
        </Button>
      </div>
    </div>
  )
}

function LabelsTab({
  projectId,
  labels,
  onRefresh,
}: {
  projectId: string
  labels: LabelT[]
  onRefresh: () => void
}) {
  const toast = useToast()
  const [name, setName] = useState('')
  const [color, setColor] = useState('#94a3b8')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  async function create(e: FormEvent) {
    e.preventDefault()
    const v = validateLabel({ name, color })
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      await api.createLabel(projectId, { name: name.trim(), color })
      toast('Label created')
      setName('')
      onRefresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setErrors({ name: 'A label with this name already exists.' })
      } else {
        setErrors({ form: 'Could not create label.' })
      }
    } finally {
      setBusy(false)
    }
  }

  function remove(label: LabelT) {
    api.deleteLabel(label.id).then(
      () => {
        toast('Label deleted')
        onRefresh()
      },
      () => toast('Delete failed', 'error'),
    )
  }

  return (
    <div className="flex max-w-lg flex-col gap-4">
      {/* ponytail: label editing (rename/recolor) needs a PATCH /labels/:id
          endpoint — the contract ships create/delete only. */}
      <form className="flex items-end gap-2" onSubmit={create}>
        <div className="w-16">
          <Label htmlFor="label-color" className="text-xs text-muted-foreground">Color</Label>
          <input
            id="label-color"
            type="color"
            className="inset-neu mt-2 h-8 w-16 cursor-pointer p-0.5"
            value={color}
            onChange={(e) => setColor(e.target.value)}
            aria-invalid={!!errors.color}
          />
        </div>
        <div className="flex-1">
          <Field label="New label" htmlFor="label-name" error={errors.name}>
            <Input
              id="label-name"
              className="inset-neu"
              aria-invalid={!!errors.name}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. bug"
            />
          </Field>
        </div>
        <Button type="submit" disabled={busy}>
          <TagIcon strokeWidth={1.5} />
          {busy ? 'Adding…' : 'Add label'}
        </Button>
      </form>
      {errors.color && (
        <p role="alert" className="text-xs text-destructive">
          {errors.color}
        </p>
      )}
      {errors.form && (
        <p role="alert" className="text-sm text-destructive">
          {errors.form}
        </p>
      )}

      <div className="flex flex-col gap-1.5">
        {labels.length === 0 && (
          <p className="text-sm text-muted-foreground">No labels in this project.</p>
        )}
        {labels.map((l) => (
          <div key={l.id} className="flex items-center gap-3 rounded-lg px-1 py-1.5">
            <span
              className="size-3 shrink-0 rounded-full"
              style={{ backgroundColor: l.color, boxShadow: `0 0 0 1px ${l.color}` }}
              aria-hidden
            />
            <span className="flex-1 text-sm text-foreground">{l.name}</span>
            <span className="font-mono text-xs text-muted-foreground">{l.color}</span>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Delete label ${l.name}`}
              onClick={() => remove(l)}
            >
              <Trash2Icon strokeWidth={1.5} />
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}

function GithubTab({ detail, onRefresh }: { detail: ProjectDetail | null; onRefresh: () => void }) {
  const toast = useToast()
  const [repo, setRepo] = useState('')
  const [token, setToken] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (detail) {
      setRepo(detail.gh_repo ?? '')
      setToken('')
      setErrors({})
    }
  }, [detail])

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!detail) return
    const v = validateProjectGithub({ repo, token, hasStoredToken: detail.gh_repo != null })
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      await api.saveProjectGithub(detail.id, { repo: repo.trim(), token: token.trim() })
      toast('GitHub settings saved')
      setToken('')
      onRefresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        setErrors({ form: err.message })
      } else {
        setErrors({ form: 'Could not save GitHub settings.' })
      }
    } finally {
      setBusy(false)
    }
  }

  if (!detail) return <RowSkeletons n={4} cols={2} />

  return (
    <form className="flex max-w-lg flex-col gap-4" onSubmit={submit}>
      <Field label="Repository" htmlFor="project-gh-repo" error={errors.repo}>
        <Input
          id="project-gh-repo"
          className="inset-neu font-mono"
          placeholder="owner/name"
          aria-invalid={!!errors.repo}
          value={repo}
          onChange={(e) => setRepo(e.target.value)}
          autoFocus
        />
      </Field>
      <Field label="Personal access token" htmlFor="project-gh-token" error={errors.token}>
        <Input
          id="project-gh-token"
          type="password"
          className="inset-neu"
          placeholder={detail.gh_repo ? 'Leave blank to keep the stored token' : 'Paste a personal access token'}
          aria-invalid={!!errors.token}
          autoComplete="new-password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
      </Field>
      <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <GithubIcon className="size-3.5" strokeWidth={1.5} />
        The token is encrypted at rest and never displayed again — it only needs repo issue-write access.
      </p>
      {errors.form && (
        <p role="alert" className="text-sm text-destructive">
          {errors.form}
        </p>
      )}
      <div className="flex justify-end">
        <Button type="submit" disabled={busy}>
          {busy ? 'Saving…' : 'Save GitHub settings'}
        </Button>
      </div>
    </form>
  )
}
