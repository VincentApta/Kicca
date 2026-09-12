// SearchDropdown — global search results under the topbar input.
// Shows when q >= 2 chars; debounced 250ms; click navigates. Purely additive:
// the input still applies the board filter on Enter as before.
import { useEffect, useRef, useState } from 'react'
import { api } from '@/lib/api'
import type { SearchResult } from '@/lib/types'

export function SearchDropdown({
  q,
  onOpenTask,
  onOpenProject,
}: {
  q: string
  onOpenTask: (taskId: string, projectId: string) => void
  onOpenProject: (projectId: string) => void
}) {
  const [res, setRes] = useState<SearchResult | null>(null)
  const [open, setOpen] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const wrap = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const query = q.trim()
    if (timer.current) clearTimeout(timer.current)
    if (query.length < 2) {
      setRes(null)
      setOpen(false)
      return
    }
    timer.current = setTimeout(() => {
      api
        .search(query)
        .then((r) => {
          setRes(r.data)
          setOpen(true)
        })
        .catch(() => { /* swallow — dropdown is best-effort */ })
    }, 250)
    return () => {
      if (timer.current) clearTimeout(timer.current)
    }
  }, [q])

  // close on click-outside / Escape
  useEffect(() => {
    if (!open) return
    const down = (e: MouseEvent) => { if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false) }
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', down)
    document.addEventListener('keydown', key)
    return () => {
      document.removeEventListener('mousedown', down)
      document.removeEventListener('keydown', key)
    }
  }, [open])

  if (!open || !res) return null
  const empty = res.tasks.length === 0 && res.projects.length === 0

  return (
    <div
      ref={wrap}
      className="absolute left-0 top-full z-50 mt-2 w-80 max-h-96 overflow-y-auto rounded-xl border border-border bg-background p-0 shadow-xl"
    >
      {empty ? (
        <p className="px-4 py-6 text-center text-sm text-muted-foreground">
          No matches for “{q.trim()}”.
        </p>
      ) : (
        <>
          {res.projects.length > 0 && (
            <>
              <p className="border-b border-border px-4 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                Projects
              </p>
              <ul>
                {res.projects.map((p) => (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => { setOpen(false); onOpenProject(p.id) }}
                      className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm hover:bg-accent"
                    >
                      <span className="font-mono text-[10px] uppercase text-muted-foreground">{p.key}</span>
                      <span className="truncate text-foreground">{p.name}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </>
          )}
          {res.tasks.length > 0 && (
            <>
              <p className="border-b border-border px-4 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                Tasks
              </p>
              <ul>
                {res.tasks.map((t) => (
                  <li key={t.id}>
                    <button
                      type="button"
                      onClick={() => { setOpen(false); onOpenTask(t.id, t.project_id) }}
                      className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm hover:bg-accent"
                    >
                      <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                        {t.project_key}#{t.number}
                      </span>
                      <span className="truncate text-foreground">{t.title}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </>
          )}
        </>
      )}
    </div>
  )
}
