// Users admin page + create/edit dialogs (moved from admin-pages.tsx)
import { useEffect, useState, type FormEvent } from 'react'
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  PencilIcon,
  UserCheckIcon,
  UserPlusIcon,
  UserXIcon
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { ApiError, api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { SelectLabel } from '@/lib/labels'
import { useToast } from '@/lib/toast'
import {
  validateUserCreate,
  validateUserEdit,
  type FieldErrors
} from '@/lib/admin'
import type {
  GlobalRole,
  Paginated,
  Project,
  User,
  UserPatch
} from '@/lib/types'
import { PageShell, Field, RowSkeletons } from './shared'


const GLOBAL_ROLE_LABELS: Record<string, string> = { member: 'Member', admin: 'Admin', client: 'Client' }
const ACCOUNT_STATUS_LABELS: Record<string, string> = { active: 'Active', disabled: 'Disabled' }

// ---------------------------------------------------------------- Users page

export function UsersPage() {
  const { state } = useAuth()
  const me = state.phase === 'authenticated' ? state.user : null
  const toast = useToast()

  const [page, setPage] = useState(1)
  const [res, setRes] = useState<Paginated<User> | null>(null)
  const [error, setError] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<User | null>(null)
  const [toggling, setToggling] = useState<string | null>(null)

  function load(p: number) {
    setRes(null)
    api.listUsers(p).then(setRes, () => setError(true))
  }

  // Row disable/enable toggle (#45). Self and last-enabled-admin are guarded
  // server-side (409) — surface the message instead of a generic failure.
  function toggleDisabled(u: User) {
    setToggling(u.id)
    api.patchUser(u.id, { disabled: !(u.disabled === true) }).then(
      (updated) => {
        setToggling(null)
        // disabled is omitempty on the wire — normalize instead of spreading
        setRes((r) =>
          r
            ? {
                ...r,
                data: r.data.map((x) =>
                  x.id === updated.id ? { ...x, ...updated, disabled: updated.disabled === true } : x,
                )
              }
            : r,
        )
        toast(updated.disabled ? `${updated.name} disabled` : `${updated.name} enabled`)
      },
      (err) => {
        setToggling(null)
        toast(err instanceof ApiError ? err.message : 'Could not update user', 'error')
      },
    )
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
                <TableHead className="w-24" aria-label="Actions" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {res.data.map((u) => {
                const disabled = u.disabled === true
                const isSelf = !!me && u.id === me.id
                return (
                  <TableRow key={u.id} className={disabled ? 'opacity-50' : undefined}>
                    <TableCell className="font-medium">{u.name}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">{u.email}</TableCell>
                    <TableCell className="text-xs">
                      {GLOBAL_ROLE_LABELS[u.global_role] ?? u.global_role}
                    </TableCell>
                    <TableCell className="text-xs">
                      {disabled ? (
                        <Badge variant="secondary">Disabled</Badge>
                      ) : (
                        <span className="text-status-done">Active</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={disabled ? `Enable ${u.name}` : `Disable ${u.name}`}
                          title={isSelf && !disabled ? 'You cannot disable your own account' : undefined}
                          disabled={toggling === u.id || (isSelf && !disabled)}
                          onClick={() => toggleDisabled(u)}
                        >
                          {disabled ? (
                            <UserCheckIcon strokeWidth={1.5} />
                          ) : (
                            <UserXIcon strokeWidth={1.5} />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`Edit ${u.name}`}
                          onClick={() => setEditing(u)}
                        >
                          <PencilIcon strokeWidth={1.5} />
                        </Button>
                      </div>
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
        currentlyDisabled={editing?.disabled === true}
        isSelf={!!editing && !!me && editing.id === me.id}
        onClose={() => setEditing(null)}
        onSaved={(updated) => {
          setRes((r) =>
            r
              ? {
                  ...r,
                  data: r.data.map((u) =>
                    u.id === updated.id ? { ...u, ...updated, disabled: updated.disabled === true } : u,
                  )
                }
              : r,
          )
          setEditing(null)
        }}
      />
    </PageShell>
  )
}

// Checkbox picker of the client's linked projects (client role only —
// projects they may submit tickets into). Native checkboxes, team-members
// dialog style.
function ProjectLinksPicker({
  projects,
  selected,
  error,
  onToggle
}: {
  projects: Project[] | null
  selected: ReadonlySet<string>
  error?: string
  onToggle: (id: string) => void
}) {
  return (
    <Field label="Linked projects" htmlFor="client-project-links" error={error}>
      <div
        id="client-project-links"
        className="inset-neu flex max-h-48 flex-col gap-0.5 overflow-y-auto p-1.5"
      >
        {projects === null && (
          <p className="p-2 text-sm text-muted-foreground">Loading projects…</p>
        )}
        {projects?.length === 0 && (
          <p className="p-2 text-sm text-muted-foreground">No projects exist yet.</p>
        )}
        {projects?.map((p) => (
          <label
            key={p.id}
            className="flex cursor-pointer items-center gap-3 rounded-lg px-2 py-1.5 hover:bg-accent"
          >
            <input
              type="checkbox"
              className="size-4 accent-[var(--primary)]"
              checked={selected.has(p.id)}
              onChange={() => onToggle(p.id)}
            />
            <span className="flex-1 truncate text-sm text-foreground">{p.name}</span>
            <span className="font-mono text-xs text-muted-foreground">{p.key}</span>
          </label>
        ))}
      </div>
    </Field>
  )
}

function CreateUserDialog({
  open,
  onClose,
  onCreated
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
  const [projects, setProjects] = useState<Project[] | null>(null)
  const [linked, setLinked] = useState<ReadonlySet<string>>(new Set())
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (open) {
      setEmail('')
      setName('')
      setPassword('')
      setRole('member')
      setLinked(new Set())
      setErrors({})
      setProjects(null)
      // lazy: only fetched when a client might be created
      api.listProjects().then(({ data }) => setProjects(data), () => setProjects([]))
    }
  }, [open])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const v = validateUserCreate({ email, name, password })
    if (role === 'client' && linked.size === 0) v.projects = 'A client must be linked to at least one project.'
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      await api.createUser({
        email: email.trim(),
        name: name.trim(),
        password,
        global_role: role,
        ...(role === 'client' ? { project_ids: [...linked] } : {})
      })
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
                <SelectItem value="client">Client</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          {role === 'client' && (
            <ProjectLinksPicker
              projects={projects}
              selected={linked}
              error={errors.projects}
              onToggle={(id) =>
                setLinked((s) => {
                  const next = new Set(s)
                  if (next.has(id)) next.delete(id)
                  else next.add(id)
                  return next
                })
              }
            />
          )}
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
  onSaved
}: {
  user: User | null
  currentlyDisabled: boolean
  isSelf: boolean
  onClose: () => void
  onSaved: (updated: User) => void
}) {
  const toast = useToast()
  const [name, setName] = useState('')
  const [role, setRole] = useState<GlobalRole>('member')
  const [status, setStatus] = useState<'active' | 'disabled'>('active')
  const [password, setPassword] = useState('')
  const [projects, setProjects] = useState<Project[] | null>(null)
  const [linked, setLinked] = useState<ReadonlySet<string>>(new Set())
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (user) {
      setName(user.name)
      setRole(user.global_role)
      setStatus(currentlyDisabled ? 'disabled' : 'active')
      setPassword('')
      setLinked(new Set(user.project_ids ?? []))
      setErrors({})
      // cheap unconditional load: role can be switched to client in-form
      setProjects(null)
      api.listProjects().then(({ data }) => setProjects(data), () => setProjects([]))
    }
  }, [user, currentlyDisabled])

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!user) return
    const v = validateUserEdit(name, password)
    if (role === 'client' && linked.size === 0) v.projects = 'A client must be linked to at least one project.'
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      const patch: UserPatch = { name: name.trim(), global_role: role }
      if (role === 'client') patch.project_ids = [...linked]
      if (password) patch.password = password
      const nextDisabled = status === 'disabled'
      if (nextDisabled !== currentlyDisabled) patch.disabled = nextDisabled
      const updated = await api.patchUser(user.id, patch)
      toast('User updated')
      onSaved(updated)
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
                <SelectItem value="client">Client</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          {role === 'client' && (
            <ProjectLinksPicker
              projects={projects}
              selected={linked}
              error={errors.projects}
              onToggle={(id) =>
                setLinked((s) => {
                  const next = new Set(s)
                  if (next.has(id)) next.delete(id)
                  else next.add(id)
                  return next
                })
              }
            />
          )}
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
