// Shared admin surface primitives — PageShell, Field, RowSkeletons,
// loadAllUsers, PROJECT_ROLE_LABELS (moved from admin-pages.tsx).
import type { ReactNode } from 'react'
import { api } from '@/lib/api'
import { Label } from '@/components/ui/label'
import type { User } from '@/lib/types'

export function PageShell({
  title,
  description,
  actions,
  children
}: {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
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
      <div className="card-neu mt-6">{children}</div>
    </div>
  )
}

export function Field({
  label,
  htmlFor,
  error,
  children
}: {
  label: string
  htmlFor?: string
  error?: string
  children: ReactNode
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

export function RowSkeletons({ n, cols }: { n: number; cols: number }) {
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
export async function loadAllUsers(): Promise<User[]> {
  const out: User[] = []
  for (let page = 1; page <= 10; page++) {
    const res = await api.listUsers(page)
    out.push(...res.data)
    if (out.length >= res.total || res.data.length === 0) break
  }
  return out
}

export const PROJECT_ROLE_LABELS: Record<string, string> = { project_admin: 'Project admin', member: 'Member' }
