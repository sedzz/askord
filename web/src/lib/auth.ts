import { queryOptions, type QueryClient } from '@tanstack/react-query'
import { redirect } from '@tanstack/react-router'

export type User = {
  id: string
  username: string
  avatar: string | null
}

// Devuelve null si no hay sesión (401).
async function fetchMe(): Promise<User | null> {
  const res = await fetch('/api/me')
  if (res.status === 401) return null
  if (!res.ok) throw new Error(`GET /api/me falló: ${res.status}`)
  return res.json()
}

export const meQuery = queryOptions({
  queryKey: ['me'],
  queryFn: fetchMe,
  staleTime: 60_000,
})

export async function logout(): Promise<void> {
  const res = await fetch('/auth/logout', { method: 'POST' })
  if (!res.ok) throw new Error(`POST /auth/logout falló: ${res.status}`)
}

export function avatarUrl(user: User): string | null {
  if (!user.avatar) return null
  return `https://cdn.discordapp.com/avatars/${user.id}/${user.avatar}.png?size=64`
}

// Para beforeLoad de rutas protegidas: redirige a /login si no hay sesión.
export async function requireUser(queryClient: QueryClient): Promise<void> {
  const user = await queryClient.ensureQueryData(meQuery)
  if (!user) throw redirect({ to: '/login' })
}
