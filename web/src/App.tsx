// T1 stub — proves the DESIGN.md token pipeline (neumorphic surfaces, dark default).
// Board, sidebar, and topbar arrive with later tickets.
function App() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-8">
      <div className="card-neu w-full max-w-md p-8">
        <h1 className="font-heading text-2xl font-semibold text-foreground">
          kicca
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Internal task manager. Scaffold only — board lands in T4.
        </p>
        <div className="inset-neu mt-6 p-4">
          <p className="font-mono text-xs text-muted-foreground">
            KEY-123 · mono data style
          </p>
        </div>
        <button type="button" className="btn-neu mt-6 px-4 py-2 text-sm font-medium text-foreground">
          Neumorphic button
        </button>
      </div>
    </main>
  )
}

export default App
