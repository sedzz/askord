import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useEffect, useState, type ReactNode } from 'react'
import { avatarUrl, logout, meQuery } from '../lib/auth'
import { guildsQuery } from '../lib/guilds'
import { setPendingInvite, usePendingInvite } from '../lib/pendingInvite'
import { AddServerDialog } from './AddServerDialog'
import { ServerRail } from './ServerRail'
import { Button } from './ui/Button'

type Props = {
  // Servidor abierto, para marcarlo en la barra lateral.
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

  const [addOpen, setAddOpen] = useState(false)

  // Tras pulsar "Añadir bot" se consulta cada pocos segundos hasta que el bot aparece en
  // el servidor; entonces se lleva al usuario a él (donde se abre el tour de configuración).
  const pendingInvite = usePendingInvite()
  const { data: guilds } = useQuery({
    ...guildsQuery,
    refetchInterval: pendingInvite ? 3000 : false,
  })
  useEffect(() => {
    if (!pendingInvite || !guilds?.some((g) => g.id === pendingInvite && g.botInstalled)) return
    setPendingInvite(null)
    void navigate({ to: '/guilds/$guildId', params: { guildId: pendingInvite } })
  }, [pendingInvite, guilds, navigate])
  const avatar = user ? avatarUrl(user) : null

  return (
    <div className="app">
      <header className="topbar">
        <Link to="/" className="brand">
          Askord
        </Link>
        {user && (
          <div className="user">
            <Button onClick={() => setAddOpen(true)}>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <path d="M12 5v14M5 12h14" />
              </svg>
              Añadir servidor
            </Button>
            {avatar && <img src={avatar} alt="" width={32} height={32} />}
            <span>{user.username}</span>
            <Button
              variant="outline"
              onClick={() => logoutMutation.mutate()}
              loading={logoutMutation.isPending}
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
      <AddServerDialog open={addOpen} onOpenChange={setAddOpen} />
    </div>
  )
}
