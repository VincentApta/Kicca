// Attachment thumbnail (#34): images preview inline from the streaming
// endpoint, videos render an icon + filename. onDelete is team-only (the
// client portal never passes it).
import { FilmIcon, XIcon } from 'lucide-react'
import { api } from '@/lib/api'
import type { Attachment } from '@/lib/types'

/** accept= filter matching the server-side sniff allowlist */
export const ATTACHMENT_ACCEPT =
  'image/png,image/jpeg,image/webp,image/gif,video/mp4,video/quicktime,video/webm'

export function AttachmentThumb({ a, onDelete }: { a: Attachment; onDelete?: (id: string) => void }) {
  const isImage = a.content_type.startsWith('image/')
  return (
    <div className="inset-neu group relative flex size-20 items-center justify-center overflow-hidden rounded-lg">
      {isImage ? (
        <img
          src={api.attachmentUrl(a.id)}
          alt={a.filename}
          loading="lazy"
          className="size-full object-cover"
        />
      ) : (
        <div className="flex w-full flex-col items-center gap-1 px-1" title={a.filename}>
          <FilmIcon className="size-5 shrink-0 text-muted-foreground" strokeWidth={1.5} />
          <span className="w-full truncate text-center text-[9px] text-muted-foreground">
            {a.filename}
          </span>
        </div>
      )}
      {onDelete && (
        <button
          type="button"
          aria-label={`Delete ${a.filename}`}
          onClick={() => onDelete(a.id)}
          className="absolute top-1 right-1 flex size-5 items-center justify-center rounded-full bg-background/80 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 hover:text-destructive focus-visible:opacity-100"
        >
          <XIcon className="size-3" strokeWidth={1.5} />
        </button>
      )}
    </div>
  )
}
