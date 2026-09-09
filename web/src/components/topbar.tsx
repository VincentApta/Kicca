import { ListIcon, LogOutIcon, MoonIcon, SearchIcon, SquareKanbanIcon, SunIcon, XIcon } from 'lucide-react'
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
import type { Label, ProjectMember, User } from '@/lib/types'
import type { View } from './sidebar'

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
  return (
    <Select value={value || null} onValueChange={(v) => onChange(v ?? '')} disabled={disabled}>
      <SelectTrigger size="sm" className="inset-neu border-0 text-xs" aria-label={label}>
        <SelectValue placeholder={label} />
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
  onLogout,
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
  onLogout: () => void
}) {
  const showBoardControls = view === 'board' || view === 'list' || view === 'trash'
  const nActive = showBoardControls ? filtersActive(filters) + (search.trim() ? 1 : 0) : 0

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border bg-background/60 px-4">
      <div className="hidden min-w-40 flex-col lg:flex">
        <span className="truncate text-sm font-medium text-foreground">{project?.name ?? 'kicca'}</span>
        {project && (
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

          <div className="relative ml-2 min-w-48 flex-1 max-w-xs">
            <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" strokeWidth={1.5} />
            <Input
              className="inset-neu border-0 pl-8"
              placeholder="Search tasks…"
              aria-label="Search tasks"
              value={search}
              onChange={(e) => onSearch(e.target.value)}
            />
          </div>

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
              options={PRIORITY_ORDER.map((p) => ({ value: p, label: p[0].toUpperCase() + p.slice(1) }))}
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
              options={BOARD_STATUSES.map((s) => ({ value: s, label: STATUS_LABEL[s] }))}
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
        </>
      )}

      <div className="ml-auto flex items-center gap-2">
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
