// Pure admin-form validation + membership guards — UI-free so Vitest covers
// them directly. Rules mirror the backend handlers (api/internal/handlers).
import type { ProjectRole } from './types'

export type FieldErrors = Record<string, string>

// Contract: project key `^[A-Z][A-Z0-9]{1,9}$`, unique.
export const PROJECT_KEY_RE = /^[A-Z][A-Z0-9]{1,9}$/

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
// Backend label color: `^#[0-9a-fA-F]{6}$`.
const HEX_COLOR_RE = /^#[0-9a-fA-F]{6}$/

export function validateUserCreate(input: {
  email: string
  name: string
  password: string
}): FieldErrors {
  const errors: FieldErrors = {}
  if (!input.name.trim()) errors.name = 'Name is required.'
  if (!input.email.trim()) errors.email = 'Email is required.'
  else if (!EMAIL_RE.test(input.email.trim())) errors.email = 'Enter a valid email address.'
  if (!input.password) errors.password = 'Password is required.'
  else if (input.password.length < 8) errors.password = 'Use at least 8 characters.'
  return errors
}

export function validateUserEdit(name: string, password: string): FieldErrors {
  const errors: FieldErrors = {}
  if (!name.trim()) errors.name = 'Name is required.'
  if (password && password.length < 8) errors.password = 'Use at least 8 characters.'
  return errors
}

export function validateTeamName(name: string): FieldErrors {
  return name.trim() ? {} : { name: 'Name is required.' }
}

export function validateProjectGeneral(input: {
  name: string
  key: string
}): FieldErrors {
  const errors: FieldErrors = {}
  if (!input.name.trim()) errors.name = 'Name is required.'
  if (!PROJECT_KEY_RE.test(input.key)) {
    errors.key = '2–10 characters: an uppercase letter, then A–Z or 0–9 (e.g. KIC).'
  }
  return errors
}

export function validateLabel(input: { name: string; color: string }): FieldErrors {
  const errors: FieldErrors = {}
  if (!input.name.trim()) errors.name = 'Name is required.'
  if (input.color && !HEX_COLOR_RE.test(input.color)) {
    errors.color = 'Use a hex color like #0ea5e9.'
  }
  return errors
}

// PUT /projects/:id/members must keep ≥1 project_admin (server: 422).
// soleProjectAdmin returns the only remaining admin's user_id, else null.
export function soleProjectAdmin(
  members: { user_id: string; role: string }[],
): string | null {
  const admins = members.filter((m) => m.role === 'project_admin')
  return admins.length === 1 ? admins[0].user_id : null
}

export function canChangeMemberRole(
  members: { user_id: string; role: string }[],
  userId: string,
): boolean {
  return soleProjectAdmin(members) !== userId
}

export function canRemoveMember(
  members: { user_id: string; role: string }[],
  userId: string,
): boolean {
  return soleProjectAdmin(members) !== userId
}

// PUT body roles must be project_admin|member — a stray 'admin' (global-admin
// display role) normalizes to member.
export function toMemberPayload(
  members: { user_id: string; role: string }[],
): { user_id: string; role: ProjectRole }[] {
  return members.map((m) => ({
    user_id: m.user_id,
    role: m.role === 'project_admin' ? 'project_admin' : 'member',
  }))
}
