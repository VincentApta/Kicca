import {
  ListIcon, LogOutIcon, MoonIcon, SearchIcon, SquareCheckIcon,
  SquareKanbanIcon, SunIcon, Trash2Icon, UserIcon, XIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { BOARD_STATUSES, PRIORITY_ORDER, STATUS_LABEL, filtersActive, type TaskFilters } from '@/lib/board'
import { PRIORITY_LABELS, STATUS_LABELS, SelectLabel } from '@/lib/labels'
import type { Label, ProjectMember, Status, User } from '@/lib/types'
import type { View } from './sidebar'
import { NotificationBell } from './notifications'
import { ExportRangeButton } from './export-range'
import { SearchDropdown } from './search-dropdown'

function FilterSelect({
  label,
  value,
  options,
  onChange,
  disabled,
}: {
  label: string
  value: string
  options: { value: string; label: string }[]
  onChange: (v: string) => void
  disabled?: boolean
}) {
  const optionMap = {
    all: `All ${label.toLowerCase()}`,
    ...Object.fromEntries(options.map((o) => [o.value, o.label])),
  }
  return (
    <Select value={value || null} onValueChange={(v) => onChange(v ?? '')} disabled={disabled}>
      <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label={label}>
        <SelectValue placeholder={label}>
          {value ? <SelectLabel value={value} labelMap={optionMap} /> : null}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="all">{`All ${label.toLowerCase()}`}</SelectItem>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export function Topbar({
  project,
  view,
  onView,
  filters,
  onFilters,
  members,
  labels,
  search,
  onSearch,
  theme,
  onToggleTheme,
  me,
  onProfile,
  onLogout,
  selectMode = false,
  selectedCount = 0,
  onToggleSelectMode,
  onExitSelect,
  onBulkStatus,
  onBulkAssign,
  onBulkDelete,
  onExport,
  onOpenNotification,
  onOpenProject,
}: {
  project: { name: string; key: string } | null
  view: View
  onView: (v: 'board' | 'list') => void
  filters: TaskFilters
  onFilters: (f: TaskFilters) => void
  members: ProjectMember[]
  labels: Label[]
  search: string
  onSearch: (q: string) => void
  theme: 'dark' | 'light'
  onToggleTheme: () => void
  me: User
  onProfile: () => void
  onLogout: () => void
  onOpenNotification: (taskId: string, projectId: string) => void
  onOpenProject?: (projectId: string) => void
  selectMode?: boolean // multi-select mode (issue #46): bulk actions replace filters
  selectedCount?: number
  onToggleSelectMode?: () => void
  onExitSelect?: () => void
  onBulkStatus?: (s: Status) => void
  onBulkAssign?: (id: string | null) => void
  onBulkDelete?: () => void
  onExport: (range?: { from?: string; to?: string }) => void
}) {
  const showBoardControls = view === 'board' || view === 'list' || view === 'trash'
  const inProjectView = showBoardControls || view === 'settings' || view === 'analytics'
  const showFilters = showBoardControls && !selectMode
  const nActive = showFilters ? filtersActive(filters) + (search.trim() ? 1 : 0) : 0

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border bg-background/60 px-4">
      <div className="hidden min-w-40 flex-col lg:flex">
        <span className="truncate text-sm font-medium text-foreground">
          {inProjectView && project ? project.name : 'kica'}
        </span>
        {inProjectView && project && (
          <span className="font-mono text-xs text-muted-foreground">{project.key}</span>
        )}
      </div>

      {showBoardControls && (
        <>
          {/* view toggle */}
          <div className="inset-neu flex items-center gap-0.5 rounded-lg p-0.5" role="group" aria-label="View">
            <button
              type="button"
              aria-pressed={view === 'board'}
              onClick={() => onView('board')}
              className={`flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium ${
                view === 'board' ? 'bg-card text-primary shadow-[var(--shadow-raised)]' : 'text-muted-foreground'
              }`}
            >
              <SquareKanbanIcon className="size-3.5" strokeWidth={1.5} /> Board
            </button>
            <button
              type="button"
              aria-pressed={view === 'list'}
              onClick={() => onView('list')}
              className={`flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium ${
                view === 'list' ? 'bg-card text-primary shadow-[var(--shadow-raised)]' : 'text-muted-foreground'
              }`}
            >
              <ListIcon className="size-3.5" strokeWidth={1.5} /> List
            </button>
          </div>

          {selectMode ? (
            <div className="ml-2 flex flex-1 items-center gap-1.5" role="group" aria-label="Bulk actions">
              <span className="shrink-0 text-xs font-medium text-primary">
                {selectedCount} selected
              </span>
              <Select onValueChange={(v) => v !== null && onBulkStatus?.(v as Status)}>
                <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label="Move to column">
                  <SelectValue placeholder="Move to…" />
                </SelectTrigger>
                <SelectContent>
                  {BOARD_STATUSES.map((s) => (
                    <SelectItem key={s} value={s}>{STATUS_LABEL[s]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                onValueChange={(v) => {
                  if (v == null) return
                  const s = String(v)
                  onBulkAssign?.(s === 'unassign' ? null : s)
                }}
              >
                <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label="Assign to member">
                  <SelectValue placeholder="Assign…" />
                </SelectTrigger>
                <SelectContent>
                  {members.map((m) => (
                    <SelectItem key={m.user_id} value={m.user_id}>{m.name}</SelectItem>
                  ))}
                  <SelectItem value="unassign">Unassign</SelectItem>
                </SelectContent>
              </Select>
              <Button
                size="sm"
                variant="ghost"
                aria-label="Delete selected"
                onClick={onBulkDelete}
                className="text-destructive hover:text-destructive"
              >
                <Trash2Icon strokeWidth={1.5} />
                Delete
              </Button>
              <Button size="sm" variant="ghost" onClick={onExitSelect} aria-label="Deselect all">
                <XIcon strokeWidth={1.5} />
                Clear
              </Button>
            </div>
          ) : (
            <>
              {(view === 'board' || view === 'list') && (
                <button
                  type="button"
                  aria-pressed={false}
                  onClick={onToggleSelectMode}
                  className="btn-neu ml-2 flex h-7 items-center gap-1.5 rounded-lg px-2.5 text-xs font-medium text-muted-foreground hover:text-foreground"
                >
                  <SquareCheckIcon className="size-3.5" strokeWidth={1.5} />
                  Select
                </button>
              )}
              <div className="relative ml-2 min-w-48 flex-1 max-w-xs">
                <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" strokeWidth={1.5} />
                <Input
                  className="inset-neu border-0 pl-8"
                  placeholder="Search tasks…"
                  aria-label="Search tasks"
                  value={search}
                  onChange={(e) => onSearch(e.target.value)}
                />
                <SearchDropdown
                  q={search}
                  onOpenTask={onOpenNotification}
                  onOpenProject={onOpenProject ?? (() => {})}
                />
              </div>
            </>
          )}

          {showFilters && (
            <div className="hidden items-center gap-1.5 md:flex">
              <FilterSelect
                label="Assignee"
                value={filters.assignee_id}
                options={members.map((m) => ({ value: m.user_id, label: m.name }))}
                onChange={(v) => onFilters({ ...filters, assignee_id: v })}
              />
              <FilterSelect
                label="Priority"
                value={filters.priority}
                options={PRIORITY_ORDER.map((p) => ({ value: p, label: PRIORITY_LABELS[p] }))}
                onChange={(v) => onFilters({ ...filters, priority: v })}
              />
              <FilterSelect
                label="Label"
                value={filters.label_id}
                options={labels.map((l) => ({ value: l.id, label: l.name }))}
                onChange={(v) => onFilters({ ...filters, label_id: v })}
              />
              <FilterSelect
                label="Status"
                value={filters.status}
                options={BOARD_STATUSES.map((s) => ({ value: s, label: STATUS_LABELS[s] }))}
                onChange={(v) => onFilters({ ...filters, status: v })}
              />
              {nActive > 0 && (
                <button
                  type="button"
                  onClick={() => {
                    onFilters({ q: '', assignee_id: '', priority: '', label_id: '', status: '' })
                    onSearch('')
                  }}
                  className="btn-neu flex h-7 items-center gap-1 rounded-lg px-2 text-xs text-muted-foreground hover:text-foreground"
                  title="Clear filters"
                >
                  <XIcon className="size-3.5" strokeWidth={1.5} />
                  {nActive}
                </button>
              )}
            </div>
          )}
        </>
      )}

      <div className="ml-auto flex items-center gap-2">
        {showBoardControls && (
          <ExportRangeButton
            onExport={(range) => onExport(range)}
          />
        )}
        <NotificationBell
          onOpenTask={onOpenNotification}
        />
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onToggleTheme}
          aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
        >
          {theme === 'dark' ? <SunIcon strokeWidth={1.5} /> : <MoonIcon strokeWidth={1.5} />}
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger
            className="btn-neu flex h-8 items-center gap-2 rounded-lg px-2 text-sm"
            aria-label="User menu"
          >
            <span className="inset-neu flex size-6 items-center justify-center rounded-full font-mono text-xs text-primary">
              {initials(me.name || me.email)}
            </span>
            <span className="hidden max-w-32 truncate text-foreground sm:block">{me.name}</span>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuGroup>
              <DropdownMenuLabel>
                <div className="flex flex-col">
                  <span className="text-sm font-medium text-foreground">{me.name}</span>
                  <span className="truncate text-xs font-normal text-muted-foreground">{me.email}</span>
                </div>
              </DropdownMenuLabel>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            {/* self-service profile (#49) — non-admin; admins keep the Users page */}
            {me.global_role !== 'admin' && (
              <DropdownMenuItem onClick={onProfile}>
                <UserIcon strokeWidth={1.5} />
                Profile
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onClick={onLogout}>
              <LogOutIcon strokeWidth={1.5} />
              Log out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}

function initials(s: string): string {
  const parts = s.split(/[\s.@]+/).filter(Boolean)
  return ((parts[0]?.[0] ?? '?') + (parts[1]?.[0] ?? '')).toUpperCase()
}
