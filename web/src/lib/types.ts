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
export type GlobalRole = 'admin' | 'member'
export type ProjectRole = 'project_admin' | 'member'

export type User = {
  id: string
  email: string
  name: string
  global_role: GlobalRole
}

export type UserCreate = {
  email: string
  name: string
  password: string
  global_role: GlobalRole
}

export type UserPatch = Partial<{
  name: string
  global_role: GlobalRole
  password: string
  disabled: boolean
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
  number: number
  title: string
  description: string
  status: Status
  priority: Priority
  assignee: User | null
  labels: Label[]
  due_date: string | null // YYYY-MM-DD
  position: number
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
  assignee_id?: string | null
  due_date?: string | null
  label_ids?: string[]
}

export type TaskPatch = Partial<{
  title: string
  description: string
  status: Status
  priority: Priority
  assignee_id: string | null
  due_date: string | null
  label_ids: string[]
}>
