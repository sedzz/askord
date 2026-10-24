import { createFileRoute, redirect } from '@tanstack/react-router'
import { buttonClassName } from '../components/ui/button-styles'
import '../components/ui/button.css'
import { meQuery } from '../lib/auth'

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>): { error?: string } =>
    typeof search.error === 'string' ? { error: search.error } : {},
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQuery)
    if (user) throw redirect({ to: '/' })
  },
  component: Login,
})

function Login() {
  const { error } = Route.useSearch()

  return (
    <main className="page login">
      <h1>Askord</h1>
      <p>Soporte e incidencias de tu servidor, conectado a tu sistema.</p>
      {error && <p className="error">No se pudo iniciar sesión. Inténtalo de nuevo.</p>}
      {/* Navegación completa: el flujo OAuth lo gestiona el backend. */}
      <a className={buttonClassName()} href="/auth/login">
        Entrar con Discord
      </a>
    </main>
  )
}
