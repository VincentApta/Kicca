// Read-only admin listings — management forms arrive with the admin ticket.
import { useEffect, useState } from 'react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'
import type { Team, User } from '@/lib/types'

function PageShell({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex-1 overflow-auto p-6">
      <h1 className="font-heading text-2xl font-semibold text-foreground">{title}</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Read-only view — management forms arrive with the admin ticket.
      </p>
      <div className="card-neu mt-6 p-4">{children}</div>
    </div>
  )
}

export function UsersPage() {
  const [users, setUsers] = useState<User[] | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    api.listUsers().then(
      ({ data }) => setUsers(data),
      () => setError(true),
    )
  }, [])

  return (
    <PageShell title="Users">
      {error && <p className="text-sm text-destructive">Could not load users.</p>}
      {!error && users === null && <RowSkeletons n={5} cols={4} />}
      {users && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Global role</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((u) => (
              <TableRow key={u.id}>
                <TableCell className="font-medium">{u.name}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{u.email}</TableCell>
                <TableCell className="text-xs">{u.global_role}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </PageShell>
  )
}

export function TeamsPage() {
  const [teams, setTeams] = useState<Team[] | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    api.listTeams().then(
      ({ data }) => setTeams(data),
      () => setError(true),
    )
  }, [])

  return (
    <PageShell title="Teams">
      {error && <p className="text-sm text-destructive">Could not load teams.</p>}
      {!error && teams === null && <RowSkeletons n={4} cols={3} />}
      {teams && teams.length === 0 && (
        <p className="py-4 text-sm text-muted-foreground">No teams yet.</p>
      )}
      {teams && teams.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Slug</TableHead>
              {teams.some((t) => t.member_count !== undefined) && <TableHead>Members</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {teams.map((t) => (
              <TableRow key={t.id}>
                <TableCell className="font-medium">{t.name}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{t.slug}</TableCell>
                {t.member_count !== undefined && (
                  <TableCell className="font-mono text-xs">{t.member_count}</TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </PageShell>
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
