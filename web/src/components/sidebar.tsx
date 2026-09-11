import {
  ChevronLeftIcon,
  ChevronsLeftIcon,
  FoldersIcon,
  LayoutDashboardIcon,
  ListTodoIcon,
  SettingsIcon,
  SquareKanbanIcon,
  Trash2Icon,
  UserCogIcon,
  UsersIcon,
} from 'lucide-react'
import type { User } from '@/lib/types'

export type View = 'overview' | 'projects' | 'mytasks' | 'board' | 'list' | 'trash' | 'teams' | 'users' | 'settings' | 'profile'

function NavButton({
  label,
  icon,
  active,
  collapsed,
  onClick,
}: {
  label: string
  icon: React.ReactNode
  active: boolean
  collapsed: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={collapsed ? label : undefined}
      aria-current={active ? 'page' : undefined}
      className={`btn-neu flex h-9 w-full items-center gap-2.5 px-3 text-sm font-medium ${
        active ? 'text-primary' : 'text-muted-foreground hover:text-foreground'
      } ${collapsed ? 'justify-center px-0' : ''}`}
      data-active={active || undefined}
    >
      {icon}
      {!collapsed && <span className="truncate">{label}</span>}
    </button>
  )
}

export function Sidebar({
  currentProjectId,
  view,
  onNavigate,
  onMyTasks,
  collapsed,
  onToggleCollapsed,
  me,
  myTasksActive,
  settingsAvailable,
}: {
  currentProjectId: string | null
  view: View
  onNavigate: (v: View) => void
  onMyTasks: () => void
  collapsed: boolean
  onToggleCollapsed: () => void
  me: User
  myTasksActive: boolean
  settingsAvailable: boolean
}) {
  const isAdmin = me.global_role === 'admin'

  return (
    <aside
      className="flex shrink-0 flex-col gap-4 border-r border-sidebar-border bg-sidebar p-3 transition-[width] duration-150"
      style={{ width: collapsed ? 56 : 240 }}
    >
      {/* logo + collapse */}
      <div className="flex items-center gap-2 px-1 pt-1">
        <div className="inset-neu flex size-8 shrink-0 items-center justify-center">
          <SquareKanbanIcon className="size-4 text-primary" strokeWidth={1.5} />
        </div>
        {!collapsed && <span className="font-heading text-lg font-semibold text-foreground">kica</span>}
        {!collapsed && (
          <button
            type="button"
            aria-label="Collapse sidebar"
            onClick={onToggleCollapsed}
            className="ml-auto rounded-md p-1 text-muted-foreground hover:text-foreground"
          >
            <ChevronsLeftIcon className="size-4" strokeWidth={1.5} />
          </button>
        )}
      </div>

      {collapsed && (
        <button
          type="button"
          aria-label="Expand sidebar"
          onClick={onToggleCollapsed}
          className="btn-neu flex h-9 items-center justify-center text-muted-foreground"
        >
          <ChevronLeftIcon className="size-4 rotate-180" strokeWidth={1.5} />
        </button>
      )}

      <nav className="flex flex-1 flex-col gap-3" aria-label="Main">
        <NavButton
          label="Dashboard"
          icon={<LayoutDashboardIcon className="size-4 shrink-0" strokeWidth={1.5} />}
          active={view === 'overview'}
          collapsed={collapsed}
          onClick={() => onNavigate('overview')}
        />
        <NavButton
          label="Projects"
          icon={<FoldersIcon className="size-4 shrink-0" strokeWidth={1.5} />}
          active={view === 'projects'}
          collapsed={collapsed}
          onClick={() => onNavigate('projects')}
        />
        <NavButton
          label="My Tasks"
          icon={<ListTodoIcon className="size-4 shrink-0" strokeWidth={1.5} />}
          active={myTasksActive}
          collapsed={collapsed}
          onClick={onMyTasks}
        />
        <NavButton
          label="Trash"
          icon={<Trash2Icon className="size-4 shrink-0" strokeWidth={1.5} />}
          active={view === 'trash'}
          collapsed={collapsed}
          onClick={() => onNavigate('trash')}
        />
      </nav>

      {/* project context — appears when a project is open; Board + settings
          grouped so no implicit "workspace" lingers in the main nav */}
      {(currentProjectId || settingsAvailable) && (
        <div className="flex flex-col gap-3">
          {!collapsed && (
            <p className="px-3 pb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
              Project
            </p>
          )}
          {currentProjectId && (
            <NavButton
              label="Board"
              icon={<SquareKanbanIcon className="size-4 shrink-0" strokeWidth={1.5} />}
              active={view === 'board' || view === 'list' || view === 'trash'}
              collapsed={collapsed}
              onClick={() => onNavigate('board')}
            />
          )}
          {settingsAvailable && (
            <NavButton
              label="Project settings"
              icon={<SettingsIcon className="size-4 shrink-0" strokeWidth={1.5} />}
              active={view === 'settings'}
              collapsed={collapsed}
              onClick={() => onNavigate('settings')}
            />
          )}
        </div>
      )}

      {isAdmin && (
        <div className="flex flex-col gap-3">
          {!collapsed && (
            <p className="px-3 pb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
              Admin
            </p>
          )}
          <NavButton
            label="Teams"
            icon={<UsersIcon className="size-4 shrink-0" strokeWidth={1.5} />}
            active={view === 'teams'}
            collapsed={collapsed}
            onClick={() => onNavigate('teams')}
          />
          <NavButton
            label="Users"
            icon={<UserCogIcon className="size-4 shrink-0" strokeWidth={1.5} />}
            active={view === 'users'}
            collapsed={collapsed}
            onClick={() => onNavigate('users')}
          />
        </div>
      )}
    </aside>
  )
}
