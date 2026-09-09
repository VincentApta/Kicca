import { AuthProvider, useAuth } from '@/lib/auth'
import { ToastProvider } from '@/lib/toast'
import { LoginPage } from '@/components/login-page'
import { Workspace } from '@/components/workspace'
import { BoardSkeleton } from '@/components/skeletons'

function AuthGate() {
  const { state } = useAuth()
  if (state.phase === 'booting') return <BoardSkeleton />
  if (state.phase === 'anonymous') return <LoginPage />
  return <Workspace />
}

function App() {
  return (
    <AuthProvider>
      <ToastProvider>
        <AuthGate />
      </ToastProvider>
    </AuthProvider>
  )
}

export default App
