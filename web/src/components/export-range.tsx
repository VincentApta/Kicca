// ExportRangeButton — export button + optional date-range popover.
// from/to (inclusive, YYYY-MM-DD) go straight into the CSV query.
import { useEffect, useRef, useState } from 'react'
import { DownloadIcon } from 'lucide-react'
import { Button } from './ui/button'
import { Input } from './ui/input'

export function ExportRangeButton({
  onExport,
  label = 'Export',
}: {
  onExport: (range: { from?: string; to?: string }) => void
  label?: string
}) {
  const [open, setOpen] = useState(false)
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const h = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', h)
    return () => document.removeEventListener('mousedown', h)
  }, [open])

  return (
    <div className="relative" ref={ref}>
      <Button variant="ghost" size="sm" onClick={() => setOpen((o) => !o)} aria-label={`Export ${label} as CSV`}>
        <DownloadIcon strokeWidth={1.5} />
        <span className="hidden sm:inline">{label}</span>
      </Button>
      {open && (
        <div className="absolute right-0 top-full z-50 mt-2 w-64 rounded-xl border border-border bg-background p-4 shadow-xl">
          <p className="mb-2 text-xs font-medium text-muted-foreground">Created date range (optional)</p>
          <div className="flex flex-col gap-2">
            <Input type="date" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} aria-label="From date" />
            <Input type="date" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} aria-label="To date" />
          </div>
          <div className="mt-3 flex justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>Cancel</Button>
            <Button
              size="sm"
              onClick={() => {
                setOpen(false)
                onExport({ from: from || undefined, to: to || undefined })
              }}
            >
              Export
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
