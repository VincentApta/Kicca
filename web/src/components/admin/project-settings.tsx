// Project settings + FirstProjectDialog (moved from admin-pages.tsx)
import { useEffect, useState, type FormEvent } from 'react'
import { Field, RowSkeletons, loadAllUsers, PROJECT_ROLE_LABELS } from './shared'
import {
  PencilIcon,
  TagIcon,
  Trash2Icon
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { GithubIcon } from '@/components/github-icon'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
  type FieldErrors
} from '@/lib/admin'
import type {
  Label as LabelT,
  Project,
  ProjectDetail,
  ProjectMember,
  ProjectRole,
  Team,
  User
} from '@/lib/types'


export function FirstProjectDialog({
  open,
  onClose,
  onCreated
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
    ...Object.fromEntries((teams ?? []).map((t) => [t.id, t.name]))
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
        key: key.trim()
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
              className="inset-neu font-mono"
              aria-invalid={!!errors.key}
              value={key}
              onChange={(e) => setKey(e.target.value.toUpperCase())}
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
  { id: 'github', label: 'GitHub' }
]

export function ProjectSettingsPage({
  project,
  detail,
  labels,
  canManage,
  onRefresh,
  onDeleted
}: {
  project: Project
  detail: ProjectDetail | null
  labels: LabelT[]
  canManage: boolean
  onRefresh: () => void
  onDeleted: () => void
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
          <GeneralTab key={detail?.id ?? 'loading'} detail={detail} onRefresh={onRefresh} onDeleted={onDeleted} />
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
  onDeleted
}: {
  detail: ProjectDetail | null
  onRefresh: () => void
  onDeleted: () => void
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
        description: description.trim()
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
      <Field label="Team" htmlFor="project-team">
        <Input
          id="project-team"
          className="inset-neu"
          value={detail.team_name ?? '—'}
          disabled
        />
      </Field>
      <p className="-mt-2 text-xs text-muted-foreground">
        Owning team is chosen at creation and cannot be moved.
      </p>
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
          className="inset-neu font-mono"
          aria-invalid={!!errors.key}
          value={key}
          onChange={(e) => setKey(e.target.value.toUpperCase())}
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
      {editable && (
        <div className="mt-6 border-t border-border pt-4">
          <h2 className="font-heading text-base font-semibold text-destructive">
            Danger zone
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Deleting is permanent (soft delete; board and history are hidden).
          </p>
          <Button
            type="button"
            variant="ghost"
            className="mt-3 bg-destructive/10 text-destructive hover:bg-destructive/20"
            disabled={busy}
            onClick={() => {
              if (window.confirm(`Delete project "${detail.name}"? This cannot be undone.`)) {
                setBusy(true)
                api.deleteProject(detail.id).then(
                  () => {
                    toast('Project deleted')
                    onDeleted()
                  },
                  (err) => {
                    setBusy(false)
                    if (err instanceof ApiError && err.status === 403) {
                      toast('Only a global admin can delete projects', 'error')
                    } else {
                      toast('Delete failed', 'error')
                    }
                  },
                )
              }
            }}
          >
            Delete project
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
      // project members are team users only — clients never sit on a project
      // team surface (their access is the client portal via project links)
      loadAllUsers().then(
        (users) => setCandidates(users.filter((u) => u.global_role !== 'client')),
        () => setCandidates([]),
      )
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
  onRefresh
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
  const [editing, setEditing] = useState<LabelT | null>(null)
  const [editName, setEditName] = useState('')
  const [editColor, setEditColor] = useState('#94a3b8')
  const [editBusy, setEditBusy] = useState(false)
  const [editErr, setEditErr] = useState<FieldErrors>({})

  function openEdit(l: LabelT) {
    setEditing(l)
    setEditName(l.name)
    setEditColor(l.color)
    setEditErr({})
  }

  async function saveEdit(e: FormEvent) {
    e.preventDefault()
    if (!editing) return
    const v = validateLabel({ name: editName, color: editColor })
    setEditErr(v)
    if (Object.keys(v).length > 0) return
    setEditBusy(true)
    try {
      await api.patchLabel(editing.id, { name: editName.trim(), color: editColor })
      toast('Label updated')
      setEditing(null)
      onRefresh()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setEditErr({ name: 'A label with this name already exists.' })
      } else {
        setEditErr({ form: 'Could not update label.' })
      }
    } finally {
      setEditBusy(false)
    }
  }

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
              aria-label={`Edit label ${l.name}`}
              onClick={() => openEdit(l)}
            >
              <PencilIcon strokeWidth={1.5} />
            </Button>
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

      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Edit label</DialogTitle>
            <DialogDescription>Rename or recolor «{editing?.name}».</DialogDescription>
          </DialogHeader>
          <form onSubmit={saveEdit} className="flex items-end gap-2">
            <div className="w-16">
              <Label htmlFor="edit-label-color" className="text-xs text-muted-foreground">Color</Label>
              <input
                id="edit-label-color"
                type="color"
                className="inset-neu mt-2 h-8 w-16 cursor-pointer p-0.5"
                value={editColor}
                onChange={(e) => setEditColor(e.target.value)}
              />
            </div>
            <div className="flex-1">
              <Field label="Name" htmlFor="edit-label-name" error={editErr.name}>
                <Input
                  id="edit-label-name"
                  className="inset-neu"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                />
              </Field>
            </div>
            <Button type="submit" disabled={editBusy}>
              <PencilIcon strokeWidth={1.5} />
              {editBusy ? 'Saving…' : 'Save'}
            </Button>
          </form>
          {editErr.form && (
            <p role="alert" className="text-sm text-destructive">{editErr.form}</p>
          )}
        </DialogContent>
      </Dialog>
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
