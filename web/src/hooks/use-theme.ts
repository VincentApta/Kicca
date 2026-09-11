import { useCallback, useEffect, useState } from 'react'

export type Theme = 'dark' | 'light'

function read(): Theme {
  try {
    return localStorage.getItem('kica-theme') === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(read)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
    try {
      localStorage.setItem('kica-theme', theme)
    } catch {
      // private mode — session-only theme
    }
  }, [theme])

  const toggle = useCallback(() => setTheme((t) => (t === 'dark' ? 'light' : 'dark')), [])
  return { theme, toggle }
}
