import { useEffect, useMemo, useState } from 'react'
import { PlusIcon, RotateCcwIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Sidebar, type View } from './sidebar'
import { Topbar } from './topbar'
import { LoginPage } from './login-page'
import { BoardView } from './board/board-view'
import { ListView } from './list-view'
import { CreateTaskDialog } from './create-task-dialog'
import { TaskDrawer } from './task-drawer'
import { FirstProjectDialog, ProjectSettingsPage, TeamsPage, UsersPage } from './admin-pages'
import MyTasksPage from './my-tasks-page'
import { DashboardPage } from './dashboard-page'
import { ProjectsListPage } from './projects-list-page'
import { BoardSkeleton, EmptyState, ListSkeleton } from './skeletons'
import { useAuth } from '@/lib/auth'
import { useTheme } from '@/hooks/use-theme'
import { useToast } from '@/lib/toast'
import { api } from '@/lib/api'
import {
  EMPTY_FILTERS,
  STATUS_LABEL,
  boardTasks,
  filtersActive,
  planMove,
  taskKey,
  type TaskFilters,
} from '@/lib/board'
import type { GhLink, Label, Priority, Project, ProjectDetail, Status, Task, TaskPatch, User } from '@/lib/types'

const PER_PAGE = 100

export function Workspace() {
  const { state, logout } = useAuth()
  const { theme, toggle } = useTheme()
  const toast = useToast()

  const [projects, setProjects] = useState<Project[] | null>(null)
  const [currentProjectId, setCurrentProjectId] = useState<string | null>(null)
  const [detail, setDetail] = useState<ProjectDetail | null>(null)
  const [labels, setLabels] = useState<Label[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [meta, setMeta] = useState({ page: 1, total: 0 })
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [filters, setFilters] = useState<TaskFilters>(EMPTY_FILTERS)
  const [search, setSearch] = useState('')
  const [view, setView] = useState<View>('overview')
  const [collapsed, setCollapsed] = useState(false)
  const [drawerId, setDrawerId] = useState<string | null>(null)
  const [createStatus, setCreateStatus] = useState<Status | null>(null)
  const [firstOpen, setFirstOpen] = useState(false)

  const me = state.phase === 'authenticated' ? state.user : null
  const project = projects?.find((p) => p.id === currentProjectId) ?? null
  const trashMode = view === 'trash'

  // projects once per session
  useEffect(() => {
    api.listProjects().then(
      ({ data }) => {
        setProjects(data)
        let first: string | null = data[0]?.id ?? null
        try {
          const stored = localStorage.getItem('kica-project')
          if (stored && data.some((p) => p.id === stored)) first = stored
        } catch {
          // ignore
        }
        setCurrentProjectId(first)
      },
      () => setProjects([]),
    )
  }, [])

  // project detail + labels
  useEffect(() => {
    if (!currentProjectId) return
    setDetail(null)
    setLabels([])
    refreshProject()
  }, [currentProjectId])

  // refetch current project detail/labels/projects — used after settings
  // mutations so board, topbar and switcher see the new state.
  function refreshProject() {
    if (!currentProjectId) return
    api.getProject(currentProjectId).then(setDetail, () => {})
    api.listLabels(currentProjectId).then(
      ({ data }) => setLabels(data),
      () => {},
    )
    api.listProjects().then(({ data }) => setProjects(data), () => {})
  }

  // debounce search box → filters.q
  useEffect(() => {
    const t = setTimeout(() => {
      setFilters((f) => (f.q === search ? f : { ...f, q: search }))
    }, 250)
    return () => clearTimeout(t)
  }, [search])

  // task fetch: server-side q/assignee/priority/label; status only for trash
  useEffect(() => {
    if (!currentProjectId) return
    const ctrl = new AbortController()
    setLoading(true)
    const t = setTimeout(() => {
      const params = trashMode
        ? { status: 'trash' as Status }
        : {
            q: filters.q,
            assignee_id: filters.assignee_id,
            priority: filters.priority !== '' ? (filters.priority as Priority) : undefined,
            label: filters.label_id,
          }
      api.listTasks(currentProjectId, params).then(
        ({ data, page, total }) => {
          if (ctrl.signal.aborted) return
          setTasks(data)
          setMeta({ page, total })
          setLoading(false)
        },
        () => {
          if (ctrl.signal.aborted) return
          setTasks([])
          setLoading(false)
          toast('Could not load tasks', 'error')
        },
      )
    }, 200)
    return () => {
      ctrl.abort()
      clearTimeout(t)
    }
  }, [currentProjectId, filters.q, filters.assignee_id, filters.priority, filters.label_id, trashMode, toast])

  function loadMore() {
    if (!currentProjectId || loadingMore) return
    setLoadingMore(true)
    const params = trashMode
      ? { status: 'trash' as Status, page: meta.page + 1, per_page: PER_PAGE }
      : {
          q: filters.q,
          assignee_id: filters.assignee_id,
          priority: filters.priority !== '' ? (filters.priority as Priority) : undefined,
          label: filters.label_id,
          page: meta.page + 1,
          per_page: PER_PAGE,
        }
    api.listTasks(currentProjectId, params).then(
      ({ data, page, total }) => {
        setTasks((ts) => [...ts, ...data])
        setMeta({ page, total })
        setLoadingMore(false)
      },
      () => setLoadingMore(false),
    )
  }

  const shown = useMemo(
    () => (trashMode ? tasks.filter((t) => t.status === 'trash') : boardTasks(tasks, filters)),
    [tasks, filters, trashMode],
  )

  // ---- mutations (optimistic where user-visible ordering is at stake) ----

  function handleMove(dragId: string, toStatus: Status, toIndex: number | null) {
    const snapshot = tasks
    const { next, move } = planMove(tasks, dragId, toStatus, toIndex)
    setTasks(next)
    api.moveTask(dragId, move).then(
      (updated) => {
        setTasks((ts) => ts.map((t) => (t.id === updated.id ? updated : t)))
        toast(`Moved to ${STATUS_LABEL[toStatus]}`)
      },
      () => {
        setTasks(snapshot)
        toast('Move failed', 'error')
      },
    )
  }

  async function handlePatch(id: string, patch: TaskPatch): Promise<Task> {
    const updated = await api.patchTask(id, patch)
    setTasks((ts) => ts.map((t) => (t.id === id ? updated : t)))
    return updated
  }

  function handleTrash(id: string) {
    const snapshot = tasks
    setTasks((ts) => ts.filter((t) => t.id !== id))
    setDrawerId((cur) => (cur === id ? null : cur))
    api.deleteTask(id).then(
      () => toast('Moved to trash'),
      () => {
        setTasks(snapshot)
        toast('Trash failed', 'error')
      },
    )
  }

  function handleGhLink(id: string, link: GhLink) {
    setTasks((ts) => ts.map((t) => (t.id === id ? { ...t, gh_link: link } : t)))
  }

  function handleRestore(id: string) {
    api.restoreTask(id).then(
      (updated) => {
        setTasks((ts) => ts.map((t) => (t.id === id ? updated : t)))
        toast('Restored to Backlog')
      },
      () => toast('Restore failed', 'error'),
    )
  }

  async function handleCreate(body: {
    title: string
    description?: string
    status: Status
    priority?: Priority
    assignee_id?: string | null
    due_date?: string | null
    label_ids?: string[]
  }): Promise<Task> {
    const t = await api.createTask(currentProjectId!, body)
    setTasks((ts) => [...ts, t])
    toast(project ? `Created ${taskKey(project.key, t)}` : 'Task created')
    return t
  }

  if (state.phase !== 'authenticated' || !me) {
    return state.phase === 'anonymous' ? <LoginPage /> : <BootSkeleton />
  }

  function switchProject(id: string) {
    setCurrentProjectId(id)
    setDrawerId(null)
    try {
      localStorage.setItem('kica-project', id)
    } catch {
      // ignore
    }
  }

  if (projects === null) return <BootSkeleton />

  if (projects.length === 0) {
    // Bootstrap state (issue #16): admins get a create-first-project CTA and
    // working admin nav; members can only be added by an admin.
    const isAdmin = me.global_role === 'admin'
    return (
      <ShellFrame projects={projects} me={me} theme={theme} onToggleTheme={toggle} onLogout={logout} view={view}
        onView={(v) => setView(v)} filters={filters} onFilters={setFilters} members={[]} labels={[]}
        search={search} onSearch={setSearch} project={null}
        collapsed={collapsed} onToggleCollapsed={() => setCollapsed((c) => !c)}
        currentProjectId={null} onSwitchProject={switchProject}
        onNavigate={setView} onMyTasks={() => setView('mytasks')} myTasksActive={false}
        settingsAvailable={false}>
        {view === 'teams' ? (
          <TeamsPage />
        ) : view === 'users' ? (
          <UsersPage />
        ) : (
          <EmptyState
            title="No projects yet"
            hint={isAdmin
              ? 'Create the first team and project to get the board rolling.'
              : 'Ask an admin to add you to a project.'}
            action={isAdmin && (
              <Button className="mt-3" onClick={() => setFirstOpen(true)}>
                <PlusIcon strokeWidth={1.5} />
                Create the first project
              </Button>
            )}
          />
        )}
        {isAdmin && (
          <FirstProjectDialog
            open={firstOpen}
            onClose={() => setFirstOpen(false)}
            onCreated={(p) => {
              setFirstOpen(false)
              setProjects([p])
              setCurrentProjectId(p.id)
              setView('board')
            }}
          />
        )}
      </ShellFrame>
    )
  }

  const members = detail?.members ?? []
  const drawerTask = tasks.find((t) => t.id === drawerId) ?? null
  const hasMore = tasks.length < meta.total
  const filtered = filtersActive(filters) + (search.trim() ? 1 : 0) > 0
  // project settings reach: global admin or project_admin (sidebar T5 pattern)
  const canManageProject =
    me.global_role === 'admin' || detail?.my_role === 'project_admin'

  return (
    <ShellFrame
      projects={projects}
      me={me}
      theme={theme}
      onToggleTheme={toggle}
      onLogout={logout}
      view={view}
      onView={(v) => setView(v)}
      filters={filters}
      onFilters={setFilters}
      members={members}
      labels={labels}
      search={search}
      onSearch={setSearch}
      project={project}
      collapsed={collapsed}
      onToggleCollapsed={() => setCollapsed((c) => !c)}
      currentProjectId={currentProjectId}
      onSwitchProject={switchProject}
      onNavigate={(v) => setView(v)}
      onMyTasks={() => setView('mytasks')}
      myTasksActive={view === 'mytasks'}
      settingsAvailable={canManageProject}
    >
      {view === 'overview' ? (
        <DashboardPage
          onSelectTask={(projectId, taskId) => {
            if (!taskId) {
              setCurrentProjectId(projectId)
              setView('board')
              return
            }
            setCurrentProjectId(projectId)
            setView('board')
            setTimeout(() => setDrawerId(taskId), 50)
          }}
        />
      ) : view === 'projects' ? (
        <ProjectsListPage
          projects={projects}
          onSwitchProject={(id) => { setCurrentProjectId(id); setView('board') }}
          onCreateProject={() => setFirstOpen(true)}
        />
      ) : view === 'settings' ? (
        project ? (
          <ProjectSettingsPage
            project={project}
            detail={detail}
            labels={labels}
            canManage={canManageProject}
            onRefresh={refreshProject}
            onDeleted={() => {
              setCurrentProjectId(null)
              refreshProject()
              setView('projects')
            }}
          />
        ) : (
          <BootSkeleton />
        )
      ) : view === 'teams' ? (
        <TeamsPage />
      ) : view === 'users' ? (
        <UsersPage />
      ) : view === 'mytasks' ? (
        <MyTasksPage
          onSelectTask={(projectId: string, taskId: string) => {
            setCurrentProjectId(projectId)
            setView('board')
            setTimeout(() => setDrawerId(taskId), 50)
          }}
        />
      ) : loading ? (
        view === 'list' || trashMode ? <ListSkeleton /> : <BoardSkeleton />
      ) : trashMode ? (
        shown.length === 0 ? (
          <EmptyState title="Trash is empty" hint="Deleted tasks rest here until purged." />
        ) : (
          <div className="flex flex-1 flex-col overflow-hidden">
            <ListView
              tasks={shown}
              projectKey={project?.key ?? ''}
              onOpen={setDrawerId}
              rowAction={(t) => (
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label={`Restore ${taskKey(project?.key ?? '', t)}`}
                  onClick={() => handleRestore(t.id)}
                >
                  <RotateCcwIcon strokeWidth={1.5} />
                  Restore
                </Button>
              )}
            />
            <LoadMore hasMore={hasMore} loading={loadingMore} onLoadMore={loadMore} />
          </div>
        )
      ) : view === 'list' ? (
        shown.length === 0 ? (
          <EmptyState
            title={filtered ? 'No tasks match the filters' : 'No tasks yet'}
            hint={filtered ? 'Adjust or clear the filters above.' : 'Create the first one from any column.'}
          />
        ) : (
          <div className="flex flex-1 flex-col overflow-hidden">
            <ListView tasks={shown} projectKey={project?.key ?? ''} onOpen={setDrawerId} />
            <LoadMore hasMore={hasMore} loading={loadingMore} onLoadMore={loadMore} />
          </div>
        )
      ) : (
        <>
          {shown.length === 0 && (
            <div className="px-4 pt-2">
              <p className="text-xs text-muted-foreground">
                {filtered
                  ? 'No tasks match the filters.'
                  : 'Empty board — create the first task from any column.'}
              </p>
            </div>
          )}
          <BoardView
            tasks={shown}
            projectKey={project?.key ?? ''}
            onOpen={setDrawerId}
            onAdd={setCreateStatus}
            onMove={handleMove}
          />
          {hasMore && <LoadMore hasMore loading={loadingMore} onLoadMore={loadMore} />}
        </>
      )}

      <CreateTaskDialog
        open={createStatus !== null}
        status={createStatus ?? 'backlog'}
        members={members}
        labels={labels}
        onClose={() => setCreateStatus(null)}
        onCreated={() => {}}
        onCreate={handleCreate}
      />

      <TaskDrawer
        task={drawerTask}
        members={members}
        labels={labels}
        projectKey={project?.key ?? ''}
        onClose={() => setDrawerId(null)}
        onPatch={handlePatch}
        onTrash={handleTrash}
        onGhLink={handleGhLink}
      />

      {me?.global_role === 'admin' && (
        <FirstProjectDialog
          open={firstOpen}
          onClose={() => setFirstOpen(false)}
          onCreated={(p) => {
            setFirstOpen(false)
            setProjects((prev) => {
              const list = prev ? [...prev, p] : [p]
              return list
            })
            setCurrentProjectId(p.id)
            setView('board')
          }}
        />
      )}
    </ShellFrame>
  )
}

type ShellProps = {
  children: React.ReactNode
  projects: Project[]
  me: User
  theme: 'dark' | 'light'
  onToggleTheme: () => void
  onLogout: () => void
  view: View
  onView: (v: 'board' | 'list') => void
  filters: TaskFilters
  onFilters: (f: TaskFilters) => void
  members: ProjectDetail['members']
  labels: Label[]
  search: string
  onSearch: (q: string) => void
  project: { name: string; key: string } | null
  collapsed: boolean
  onToggleCollapsed: () => void
  currentProjectId: string | null
  onSwitchProject: (id: string) => void
  onNavigate: (v: View) => void
  onMyTasks: () => void
  myTasksActive: boolean
  settingsAvailable: boolean
}

function ShellFrame({ children, ...shell }: ShellProps) {
  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar
        currentProjectId={shell.currentProjectId}
        view={shell.view}
        onNavigate={shell.onNavigate}
        onMyTasks={shell.onMyTasks}
        collapsed={shell.collapsed}
        onToggleCollapsed={shell.onToggleCollapsed}
        me={shell.me}
        myTasksActive={shell.myTasksActive}
        settingsAvailable={shell.settingsAvailable}
      />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar
          project={shell.project}
          view={shell.view}
          onView={shell.onView}
          filters={shell.filters}
          onFilters={shell.onFilters}
          members={shell.members}
          labels={shell.labels}
          search={shell.search}
          onSearch={shell.onSearch}
          theme={shell.theme}
          onToggleTheme={shell.onToggleTheme}
          me={shell.me}
          onLogout={shell.onLogout}
        />
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden">{children}</main>
      </div>
    </div>
  )
}

function LoadMore({
  hasMore,
  loading,
  onLoadMore,
}: {
  hasMore: boolean
  loading: boolean
  onLoadMore: () => void
}) {
  if (!hasMore) return null
  return (
    <div className="flex justify-center p-3">
      <Button variant="ghost" size="sm" onClick={onLoadMore} disabled={loading}>
        {loading ? 'Loading…' : 'Load more'}
      </Button>
    </div>
  )
}

function BootSkeleton() {
  return (
    <div className="flex h-screen bg-background" aria-hidden>
      <div className="w-60 shrink-0 border-r border-border p-3">
        <div className="mb-4 flex gap-2">
          <div className="inset-neu size-8 animate-pulse rounded-lg" />
          <div className="h-4 w-16 animate-pulse rounded bg-secondary" />
        </div>
        <div className="inset-neu mb-4 h-9 animate-pulse rounded-lg" />
        {[0, 1, 2].map((i) => (
          <div key={i} className="mb-1.5 h-9 animate-pulse rounded-lg bg-secondary" />
        ))}
      </div>
      <div className="flex flex-1 flex-col">
        <div className="flex h-14 items-center gap-2 border-b border-border px-4">
          <div className="inset-neu h-7 w-40 animate-pulse rounded-lg" />
          <div className="ml-auto inset-neu size-8 animate-pulse rounded-lg" />
        </div>
        <BoardSkeleton />
      </div>
    </div>
  )
}
