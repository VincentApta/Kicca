// Wire types — mirrors docs/api-contract.md exactly.

export type Status =
  | 'inbox'
  | 'backlog'
  | 'in_progress'
  | 'review'
  | 'done'
  | 'blocked'
  | 'trash'

export type Priority = 'urgent' | 'high' | 'medium' | 'low'
export type TaskType = 'task' | 'bug' | 'feature' | 'chore'
export type GlobalRole = 'admin' | 'member' | 'client'
export type ProjectRole = 'project_admin' | 'member'

export type User = {
  id: string
  email: string
  name: string
  global_role: GlobalRole
  project_ids?: string[] // client role only, admin user-management responses
}

export type UserCreate = {
  email: string
  name: string
  password: string
  global_role: GlobalRole
  project_ids?: string[] // client only: projects they may submit tickets into
}

export type UserPatch = Partial<{
  name: string
  global_role: GlobalRole
  password: string
  disabled: boolean
  project_ids: string[] // client only: replaces the link set
}>

export type Project = {
  id: string
  team_id: string
  name: string
  key: string
  description: string
}

export type ProjectMember = {
  user_id: string
  role: ProjectRole | 'admin'
  name: string
  email: string
}

export type ProjectDetail = Project & {
  my_role: ProjectRole | 'admin'
  members: ProjectMember[]
  gh_repo: string | null // owner/name when the GitHub integration is configured
}

export type Label = { id: string; name: string; color: string }

export type GhLink = {
  repo: string
  issue_number: number
  issue_url: string
} | null

export type Task = {
  id: string
  project_id: string
  project_key?: string
  project_name?: string
  number: number
  title: string
  description: string
  status: Status
  priority: Priority
  type: TaskType
  estimate: number | null // story points, >= 0
  assessment: string // triage note; used as GitHub issue body when non-empty
  assignee: User | null
  labels: Label[]
  due_date: string | null // YYYY-MM-DD
  position: number
  started_at: string | null // analytics: first in_progress/review touch
  done_at: string | null // analytics: entry into done
  created_by: string
  created_at: string
  updated_at: string
  gh_link: GhLink
}

export type Comment = {
  id: string
  task_id: string
  user_id: string
  body: string
  created_at: string
  author: string // resolved display name (client-portal authors have no members row)
}

export type Paginated<T> = {
  data: T[]
  page: number
  per_page: number
  total: number
}

export type Team = {
  id: string
  name: string
  slug: string
  member_count?: number
}

export type MovePayload = {
  status: Status
  before_task_id?: string | null
  after_task_id?: string | null
}

export type TaskCreate = {
  title: string
  description?: string
  status?: Status
  priority?: Priority
  type?: TaskType
  estimate?: number | null
  assignee_id?: string | null
  due_date?: string | null
  label_ids?: string[]
}

export type TaskPatch = Partial<{
  title: string
  description: string
  assessment: string
  status: Status
  priority: Priority
  type: TaskType
  estimate: number | null
  assignee_id: string | null
  due_date: string | null
  label_ids: string[]
}>

/** GET /api/tasks/events — per-day end-of-day status counts. */
export type TaskEventsDay = {
  date: string // YYYY-MM-DD
  counts: Record<string, number>
}

/** GET /api/client/projects — linked project for the submit-form picker. */
export type ClientProjectRef = { id: string; key: string; name: string }

/**
 * GET /api/client/tickets — own tickets, read-only tracking. Deliberately no
 * assessment/assignee/labels: team-only fields never reach the client.
 */
export type ClientTicket = {
  id: string
  project_id: string
  project_key: string
  number: number
  title: string
  description: string
  status: Status
  created_at: string
  updated_at: string
}

/**
 * Client ticket comments (#43): {id, body, created_at, user:{name}} — no
 * user_id/email; team comments show the author name only.
 */
export type ClientComment = {
  id: string
  body: string
  created_at: string
  user: { name: string }
}

/**
 * Task attachments (#34). Team responses carry task_id/created_by; the
 * client portal shape is minimal ({id, filename, content_type, size_bytes,
 * created_at}) — those two stay absent there.
 */
export type Attachment = {
  id: string
  filename: string
  content_type: string
  size_bytes: number
  created_at: string
  task_id?: string
  created_by?: string
}
