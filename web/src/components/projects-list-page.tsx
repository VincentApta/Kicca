import { Button } from '@/components/ui/button'
import { PlusIcon, FolderKanbanIcon } from 'lucide-react'
import type { Project } from '@/lib/types'

export function ProjectsListPage({
  projects,
  onSwitchProject,
  onCreateProject,
}: {
  projects: Project[] | null
  onSwitchProject: (id: string) => void
  onCreateProject: () => void
}) {
  return (
    <div className="flex flex-1 flex-col gap-4 p-6">
      <div className="flex items-center justify-between">
        <h2 className="font-heading text-xl font-semibold text-foreground">Projects</h2>
        <Button
          className="btn-neu flex items-center gap-2 px-4 py-2 text-sm font-medium"
          onClick={onCreateProject}
        >
          <PlusIcon className="size-4" strokeWidth={1.5} />
          New project
        </Button>
      </div>

      {!projects ? (
        <div className="text-sm text-muted-foreground">Loading…</div>
      ) : projects.length === 0 ? (
        <div className="card-neu p-8 text-center">
          <FolderKanbanIcon className="mx-auto mb-3 size-8 text-muted-foreground" strokeWidth={1.5} />
          <p className="text-sm text-muted-foreground">No projects yet. Create one to get started.</p>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {projects.map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => onSwitchProject(p.id)}
              className="card-neu flex items-center gap-4 p-4 text-left transition-shadow hover:shadow-lg"
            >
              <FolderKanbanIcon className="size-5 text-primary" strokeWidth={1.5} />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-foreground truncate">{p.name}</p>
                <p className="font-mono text-xs text-muted-foreground mt-0.5">{p.key}</p>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
