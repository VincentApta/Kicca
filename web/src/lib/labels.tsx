// Human display labels for wire values. Base UI's Select.Value renders the
// raw stored value by default — SelectLabel maps it back to display text for
// triggers; item lists reuse the same maps.
import type { Priority, Status } from './types'

export const PRIORITY_LABELS = {
  urgent: 'Urgent',
  high: 'High',
  medium: 'Medium',
  low: 'Low',
} satisfies Record<Priority, string>

export const STATUS_LABELS = {
  inbox: 'Inbox',
  backlog: 'Backlog',
  in_progress: 'In Progress',
  review: 'Review',
  done: 'Done',
  blocked: 'Blocked',
  trash: 'Trash',
} satisfies Record<Status, string>

/** Renders the human label for a stored Select value (or fallback, or the value). */
export function SelectLabel({
  value,
  labelMap,
  fallback,
}: {
  value?: string | null
  labelMap: Record<string, string>
  fallback?: string
}) {
  if (!value) return null
  return <>{labelMap[value] ?? fallback ?? value}</>
}
