// Typed API client — paths and shapes per docs/api-contract.md.
// Cookie session: same-origin fetch sends kica_session automatically.

import type {
  Attachment,
  ClientComment,
  ClientProjectRef,
  ClientTicket,
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
  TaskEventsDay,
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
  const isForm = init?.body instanceof FormData
  const res = await fetch(`/api${path}`, {
    method: init?.method ?? 'GET',
    headers: init?.body !== undefined && !isForm ? { 'Content-Type': 'application/json' } : undefined,
    body: isForm
      ? (init!.body as FormData)
      : init?.body !== undefined
        ? JSON.stringify(init.body)
        : undefined,
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

/** multipart `file` body for the attachment endpoints */
function fileForm(file: File): FormData {
  const fd = new FormData()
  fd.append('file', file)
  return fd
}

/** Cookie-authenticated CSV download (#50) — the response carries
 *  Content-Disposition: attachment, so the browser downloads, no navigation. */
export function downloadCsv(path: string) {
  const a = document.createElement('a')
  a.href = path
  document.body.appendChild(a)
  a.click()
  a.remove()
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
  deleteProject: (id: string) =>
    req<void>(`/projects/${id}`, { method: 'DELETE' }),
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

  // Tasks feed (global, cross-project). assigneeId optional — omit for all members.
  myFetchMyTasks: (assigneeId?: string, page = 1, perPage = 100, assigneeIds?: string[]) =>
    req<Paginated<Task>>(
      `/tasks?page=${page}&per_page=${perPage}${
        assigneeId
          ? `&assignee_id=${assigneeId}`
          : assigneeIds && assigneeIds.length
            ? `&assignee_ids=${assigneeIds.join(',')}`
            : ''
      }`,
    ),

  // Analytics: per-day status counts over the last `days` days (capped 90),
  // visibility-scoped server-side; optional project filter.
  fetchTaskEvents: (days = 30, projectId?: string) =>
    req<{ days: number; data: TaskEventsDay[] }>(
      `/tasks/events?days=${days}${projectId ? `&project_id=${projectId}` : ''}`,
    ),

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

  // Attachments (#34)
  listAttachments: (taskId: string) =>
    req<{ data: Attachment[] }>(`/tasks/${taskId}/attachments`),
  uploadAttachment: (taskId: string, file: File) =>
    req<Attachment>(`/tasks/${taskId}/attachments`, { method: 'POST', body: fileForm(file) }),
  deleteAttachment: (id: string) =>
    req<void>(`/attachments/${id}`, { method: 'DELETE' }),
  attachmentUrl: (id: string) => `/api/attachments/${id}`,

  // GitHub
  createGithubIssue: (taskId: string) =>
    req<NonNullable<GhLink>>(`/tasks/${taskId}/github/issue`, { method: 'POST', body: {} }),

  // Client portal (client role only; team users get 403)
  clientListProjects: () =>
    req<{ data: ClientProjectRef[] }>('/client/projects'),
  clientListTickets: (
    f: { q?: string; project_id?: string; status?: 'open' | 'closed' } = {},
    page = 1,
    perPage = 50,
  ) => {
    const p = new URLSearchParams({ page: String(page), per_page: String(perPage) })
    for (const [k, v] of Object.entries(f)) {
      if (v !== undefined && v !== '') p.set(k, v)
    }
    return req<Paginated<ClientTicket>>(`/client/tickets?${p}`)
  },
  clientCreateTicket: (body: { project_id: string; title: string; description: string }) =>
    req<ClientTicket>('/client/tickets', { method: 'POST', body }),
  clientListTicketAttachments: (ticketId: string) =>
    req<{ data: Attachment[] }>(`/client/tickets/${ticketId}/attachments`),
  clientUploadTicketAttachment: (ticketId: string, file: File) =>
    req<Attachment>(`/client/tickets/${ticketId}/attachments`, { method: 'POST', body: fileForm(file) }),

  // Client ticket comments (#43) — author name only in responses
  clientListTicketComments: (ticketId: string) =>
    req<{ data: ClientComment[] }>(`/client/tickets/${ticketId}/comments`),
  clientAddTicketComment: (ticketId: string, body: string) =>
    req<ClientComment>(`/client/tickets/${ticketId}/comments`, { method: 'POST', body: { body } }),

  // CSV export (#50) — same filters as the corresponding list endpoints
  exportTasks: (
    f: { project_id?: string; status?: string; assignee_id?: string; priority?: string; label?: string; q?: string } = {},
  ) => {
    const p = new URLSearchParams()
    for (const [k, v] of Object.entries(f)) {
      if (v !== undefined && v !== '') p.set(k, v)
    }
    const qs = p.toString()
    downloadCsv(`/api/tasks/export${qs ? `?${qs}` : ''}`)
  },
  clientExportTickets: (f: { q?: string; project_id?: string; status?: 'open' | 'closed' } = {}) => {
    const p = new URLSearchParams()
    for (const [k, v] of Object.entries(f)) {
      if (v !== undefined && v !== '') p.set(k, v)
    }
    const qs = p.toString()
    downloadCsv(`/api/client/tickets/export${qs ? `?${qs}` : ''}`)
  },
}
