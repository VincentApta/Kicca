import { useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '@/lib/auth'

type Tab = 'team' | 'client'

const COPY: Record<Tab, { title: string; caption: string; submit: string }> = {
  team: {
    title: 'Team sign in',
    caption: 'Boards, analytics and project settings for your team.',
    submit: 'Sign in to workspace',
  },
  client: {
    title: 'Client portal',
    caption: 'Submit tickets and follow their status. Accounts are issued by your project team.',
    submit: 'Enter client portal',
  },
}

export function LoginPage() {
  const { login } = useAuth()
  const [tab, setTab] = useState<Tab>('team')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const copy = COPY[tab]

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await login(email, password)
    } catch {
      setError('Invalid email or password.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-8">
      <div className="flex w-full max-w-md flex-col items-center gap-8">
        <header className="text-center">
          <h1 className="font-heading text-4xl font-semibold tracking-tight text-foreground">kica</h1>
          <p className="mt-2 text-sm text-muted-foreground">Tasks, triage and delivery — in one place.</p>
        </header>
        <div className="card-neu w-full p-8">
          <div className="inset-neu flex gap-1.5 rounded-xl p-1.5" role="tablist" aria-label="Sign in as">
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'team'}
              className={`flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                tab === 'team' ? 'btn-neu text-foreground' : 'text-muted-foreground hover:text-foreground'
              }`}
              onClick={() => setTab('team')}
            >
              Team
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'client'}
              className={`flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                tab === 'client' ? 'btn-neu text-foreground' : 'text-muted-foreground hover:text-foreground'
              }`}
              onClick={() => setTab('client')}
            >
              Client
            </button>
          </div>
          <h2 className="mt-6 font-heading text-xl font-semibold text-foreground">{copy.title}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{copy.caption}</p>
          <form className="mt-6 flex flex-col gap-4" onSubmit={onSubmit}>
            <div className="flex flex-col gap-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                required
                className="inset-neu"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                required
                className="inset-neu"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
            <Button type="submit" className="btn-neu mt-2 w-full" disabled={busy}>
              {busy ? 'Signing in…' : copy.submit}
            </Button>
          </form>
        </div>
      </div>
    </main>
  )
}
