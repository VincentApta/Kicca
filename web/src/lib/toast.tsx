// Minimal toast system — no sonner (dep budget). Verb-matched messages live
// at call sites per DESIGN.md ("Moved to Review", "Move failed").
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { CheckCircle2Icon, CircleAlertIcon, XIcon } from 'lucide-react'

type ToastKind = 'success' | 'error'
type Toast = { id: number; kind: ToastKind; message: string }

const ToastCtx = createContext<(message: string, kind?: ToastKind) => void>(() => {})

export function useToast() {
  return useContext(ToastCtx)
}

let nextId = 1

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const timers = useRef<ReturnType<typeof setTimeout>[]>([])

  useEffect(() => () => timers.current.forEach(clearTimeout), [])

  const push = useCallback((message: string, kind: ToastKind = 'success') => {
    const id = nextId++
    setToasts((ts) => [...ts.slice(-3), { id, kind, message }])
    timers.current.push(setTimeout(() => {
      setToasts((ts) => ts.filter((t) => t.id !== id))
    }, 4000))
  }, [])

  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div
        role="status"
        aria-live="polite"
        className="pointer-events-none fixed right-4 bottom-4 z-100 flex flex-col gap-2"
      >
        {toasts.map((t) => (
          <div
            key={t.id}
            className="card-neu pointer-events-auto flex max-w-sm items-center gap-2 px-4 py-3 text-sm text-foreground"
          >
            {t.kind === 'success' ? (
              <CheckCircle2Icon className="size-4 shrink-0 text-primary" strokeWidth={1.5} />
            ) : (
              <CircleAlertIcon className="size-4 shrink-0 text-destructive" strokeWidth={1.5} />
            )}
            <span className="flex-1">{t.message}</span>
            <button
              type="button"
              aria-label="Dismiss"
              className="rounded-md p-1 text-muted-foreground hover:text-foreground"
              onClick={() => setToasts((ts) => ts.filter((x) => x.id !== t.id))}
            >
              <XIcon className="size-3.5" strokeWidth={1.5} />
            </button>
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  )
}
