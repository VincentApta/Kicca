// Loading skeletons shaped like the content they stand in for (DESIGN.md).

export function BoardSkeleton() {
  return (
    <div className="flex flex-1 items-start gap-4 overflow-hidden p-4 pb-8" aria-hidden>
      {Array.from({ length: 6 }, (_, c) => (
        <div key={c} className="flex w-80 shrink-0 flex-col gap-2">
          <div className="mx-1 h-5 w-28 animate-pulse rounded bg-secondary" />
          <div className="inset-neu flex min-h-40 flex-1 flex-col gap-2 p-2">
            {Array.from({ length: Math.max(1, 4 - Math.abs(c - 2)) }, (_, i) => (
              <div key={i} className="card-neu p-3">
                <div className="h-3.5 w-3/4 animate-pulse rounded bg-secondary" />
                <div className="mt-2.5 h-3 w-1/3 animate-pulse rounded bg-secondary" />
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

export function ListSkeleton() {
  return (
    <div className="flex-1 p-4" aria-hidden>
      <div className="w-full">
        <div className="flex gap-4 border-b border-border pb-3">
          {['w-16', 'flex-1', 'w-20', 'w-16', 'w-28', 'w-32', 'w-20', 'w-20'].map((w, i) => (
            <div key={i} className={`h-3.5 ${w} shrink-0 animate-pulse rounded bg-secondary`} />
          ))}
        </div>
        {Array.from({ length: 8 }, (_, r) => (
          <div key={r} className="flex items-center gap-4 border-b border-border py-3.5">
            <div className="h-3.5 w-16 shrink-0 animate-pulse rounded bg-secondary" />
            <div className="h-3.5 flex-1 animate-pulse rounded bg-secondary" style={{ maxWidth: `${70 - (r % 3) * 12}%` }} />
            <div className="h-3.5 w-20 shrink-0 animate-pulse rounded bg-secondary" />
            <div className="h-3.5 w-16 shrink-0 animate-pulse rounded bg-secondary" />
          </div>
        ))}
      </div>
    </div>
  )
}

export function EmptyState({
  title,
  hint,
  action,
}: {
  title: string
  hint?: string
  action?: React.ReactNode
}) {
  return (
    <div className="card-neu mx-auto mt-16 flex w-full max-w-sm flex-col items-center gap-2 p-8 text-center">
      <p className="text-sm font-medium text-foreground">{title}</p>
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      {action}
    </div>
  )
}
