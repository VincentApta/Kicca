// Typed API client — paths and shapes per docs/api-contract.md.
// Cookie session: same-origin fetch sends kicca_session automatically.

import type {
  Comment,
  GhLink,
  Label,
  MovePayload,
  Paginated,
  Priority,
  Project,
  ProjectDetail,
  ProjectMember,
  ProjectRole,
  Status,
  Task,
  TaskCreate,
  TaskPatch,
  Team,
  User,
  UserCreate,
  UserPatch,
} from './types'

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

async function req<T>(
  path: string,
  init?: Omit<RequestInit, 'body'> & { body?: unknown },
): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method: init?.method ?? 'GET',
    headers: init?.body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: init?.body !== undefined ? JSON.stringify(init.body) : undefined,
    signal: init?.signal,
  })
  if (res.status === 204) return undefined as T
  const json = await res.json().catch(() => null)
  if (!res.ok) {
    const err = (json as { error?: { code?: string; message?: string } } | null)?.error
    throw new ApiError(res.status, err?.code ?? 'unknown', err?.message ?? res.statusText)
  }
  return json as T
}

export const api = {
  // Auth
  login: (email: string, password: string) =>
    req<{ user: User }>('/auth/login', { method: 'POST', body: { email, password } }),
  logout: () => req<void>('/auth/logout', { method: 'POST' }),
  me: (signal?: AbortSignal) => req<{ user: User }>('/auth/me', { signal }),

  // Users (global admin)
  listUsers: (page = 1) =>
    req<Paginated<User>>(`/users?page=${page}`),
  createUser: (body: UserCreate) =>
    req<User>('/users', { method: 'POST', body }),
  patchUser: (id: string, body: UserPatch) =>
    req<User>(`/users/${id}`, { method: 'PATCH', body }),

  // Teams
  listTeams: () => req<{ data: Team[] }>('/teams'),
  createTeam: (name: string) =>
    req<Team>('/teams', { method: 'POST', body: { name } }),
  patchTeam: (id: string, name: string) =>
    req<Team>(`/teams/${id}`, { method: 'PATCH', body: { name } }),
  deleteTeam: (id: string) =>
    req<void>(`/teams/${id}`, { method: 'DELETE' }),
  replaceTeamMembers: (id: string, user_ids: string[]) =>
    req<Team>(`/teams/${id}/members`, { method: 'PUT', body: { user_ids } }),
  listTeamMembers: (id: string) =>
    req<{ members: { user_id: string; name: string; email: string }[] }>(`/teams/${id}/members`),

  // Projects
  listProjects: () => req<{ data: Project[] }>('/projects'),
  createProject: (body: { team_id: string; name: string; key: string; description?: string }) =>
    req<Project>('/projects', { method: 'POST', body }),
  getProject: (id: string) => req<ProjectDetail>(`/projects/${id}`),
  patchProject: (
    id: string,
    body: Partial<Pick<Project, 'name' | 'key' | 'description'>>,
  ) => req<Project>(`/projects/${id}`, { method: 'PATCH', body }),
  saveProjectGithub: (id: string, body: { repo: string; token?: string }) =>
    req<{ repo: string }>(`/projects/${id}/github`, { method: 'PUT', body }),
  replaceProjectMembers: (
    id: string,
    members: { user_id: string; role: ProjectRole }[],
  ) =>
    req<{ members: ProjectMember[] }>(`/projects/${id}/members`, {
      method: 'PUT',
      body: { members },
    }),
  listLabels: (projectId: string) =>
    req<{ data: Label[] }>(`/projects/${projectId}/labels`),
  createLabel: (projectId: string, body: { name: string; color?: string }) =>
    req<Label>(`/projects/${projectId}/labels`, { method: 'POST', body }),
  deleteLabel: (id: string) =>
    req<void>(`/labels/${id}`, { method: 'DELETE' }),

  // My Tasks (global, cross-project)
  myFetchMyTasks: (assigneeId: string, page = 1, perPage = 50) =>
    req<Paginated<Task>>(`/tasks?assignee_id=${assigneeId}&page=${page}&per_page=${perPage}`),

  // Tasks
  listTasks: (
    projectId: string,
    f: {
      status?: Status
      assignee_id?: string
      priority?: Priority
      label?: string
      q?: string
      page?: number
      per_page?: number
    } = {},
  ) => {
    const p = new URLSearchParams()
    for (const [k, v] of Object.entries(f)) {
      if (v !== undefined && v !== '') p.set(k, String(v))
    }
    const qs = p.toString()
    return req<Paginated<Task>>(`/projects/${projectId}/tasks${qs ? `?${qs}` : ''}`)
  },
  getTask: (id: string) => req<Task>(`/tasks/${id}`),
  createTask: (projectId: string, body: TaskCreate) =>
    req<Task>(`/projects/${projectId}/tasks`, { method: 'POST', body }),
  patchTask: (id: string, body: TaskPatch) =>
    req<Task>(`/tasks/${id}`, { method: 'PATCH', body }),
  deleteTask: (id: string) => req<void>(`/tasks/${id}`, { method: 'DELETE' }),
  restoreTask: (id: string) =>
    req<Task>(`/tasks/${id}/restore`, { method: 'POST' }),
  moveTask: (id: string, body: MovePayload) =>
    req<Task>(`/tasks/${id}/move`, { method: 'POST', body }),

  // Comments
  listComments: (taskId: string) =>
    req<{ data: Comment[] }>(`/tasks/${taskId}/comments`),
  addComment: (taskId: string, body: string) =>
    req<Comment>(`/tasks/${taskId}/comments`, { method: 'POST', body: { body } }),

  // GitHub
  createGithubIssue: (taskId: string) =>
    req<NonNullable<GhLink>>(`/tasks/${taskId}/github/issue`, { method: 'POST', body: {} }),
}
