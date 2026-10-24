import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { avatarUrl, logout, meQuery } from '../lib/auth'
import { ServerRail } from './ServerRail'
import { Button } from './ui/Button'

type Props = {
  // Servidor abierto; sin él, la barra marca el inicio como activo.
  activeGuildId?: string
  children: ReactNode
}

// Estructura común: barra superior con el usuario y, debajo, servidores + contenido.
export function AppShell({ activeGuildId, children }: Props) {
  const { data: user } = useQuery(meQuery)
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      queryClient.setQueryData(meQuery.queryKey, null)
      await navigate({ to: '/login' })
    },
  })

  const avatar = user ? avatarUrl(user) : null

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand">Askord</span>
        {user && (
          <div className="user">
            {avatar && <img src={avatar} alt="" width={32} height={32} />}
            <span>{user.username}</span>
            <Button
              variant="outline"
              onClick={() => logoutMutation.mutate()}
              disabled={logoutMutation.isPending}
            >
              Salir
            </Button>
          </div>
        )}
      </header>
      <div className="app-body">
        <ServerRail activeGuildId={activeGuildId} />
        <main className="content">{children}</main>
      </div>
    </div>
  )
}
