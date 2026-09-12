// Teams admin page + team dialogs (moved from admin-pages.tsx)
import { useEffect, useState, type FormEvent } from 'react'
import {
  PencilIcon,
  PlusIcon,
  SearchIcon,
  Trash2Icon,
  UserPlusIcon
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
import { ApiError, api } from '@/lib/api'
import { useToast } from '@/lib/toast'
import {
  validateTeamName,
  type FieldErrors
} from '@/lib/admin'
import type {
  Team,
  User
} from '@/lib/types'
import { PageShell, Field, RowSkeletons, loadAllUsers } from './shared'


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
  onSaved
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
  onSaved
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
      // Load all users AND current team members concurrently
      Promise.all([
        loadAllUsers(),
        api.listTeamMembers(team.id).catch(() => ({ members: [] }))
      ]).then(([allUsers, memberData]) => {
        setUsers(allUsers)
        setSelected(new Set(memberData.members.map((m) => m.user_id)))
      }, () => setUsers([]))
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
