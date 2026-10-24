import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { Suspense, useState } from 'react'
import { AppShell } from '../components/AppShell'
import { GuildSubnav } from '../components/GuildSubnav'
import { GuildFrameSkeleton } from '../components/skeletons'
import { SetupTour } from '../components/SetupTour'
import { requireUser } from '../lib/auth'
import { guildsQuery } from '../lib/guilds'
import { TicketSetupContext } from '../lib/ticketSetupContext'
import { ticketConfigQuery } from '../lib/tickets'
import { useTicketSetup } from '../lib/useTicketSetup'

export const Route = createFileRoute('/guilds/$guildId')({
  beforeLoad: ({ context }) => requireUser(context.queryClient),
  // Precarga sin esperar: se entra al servidor al instante y el contenido enseña un
  // esqueleto hasta que llegan los datos.
  loader: ({ context, params }) => {
    void context.queryClient.prefetchQuery(ticketConfigQuery(params.guildId))
    void context.queryClient.prefetchQuery(guildsQuery)
  },
  pendingComponent: GuildPending,
  errorComponent: () => (
    <main className="page">
      <Link to="/">← Volver</Link>
      <p className="error">No se pudo cargar este servidor.</p>
    </main>
  ),
  component: GuildLayout,
})

function GuildLayout() {
  const { guildId } = Route.useParams()
  // key: al cambiar de servidor se reinicia todo (formularios y diálogos abiertos).
  return (
    <AppShell key={guildId} activeGuildId={guildId}>
      {/* La cabecera y la barra de servidores salen ya; el marco espera a sus datos. */}
      <Suspense fallback={<GuildFrameSkeleton />}>
        <GuildFrame guildId={guildId} />
      </Suspense>
    </AppShell>
  )
}

// Mientras se descarga la pantalla del servidor por primera vez.
function GuildPending() {
  const { guildId } = Route.useParams()
  return (
    <AppShell activeGuildId={guildId}>
      <GuildFrameSkeleton />
    </AppShell>
  )
}

// Marco de un servidor: sub-barra lateral con sus secciones y, a la derecha, la sección abierta.
function GuildFrame({ guildId }: { guildId: string }) {
  const setup = useTicketSetup(guildId)
  const { data: guilds } = useQuery(guildsQuery)
  const guild = guilds?.find((g) => g.id === guildId)

  // Mientras falte algo (servidor nuevo o a medias), el tour se abre solo, en cualquier
  // sección, y no se puede cerrar hasta terminarlo. Se decide al entrar: publicar el
  // panel no lo cierra a mitad.
  const [tourOpen, setTourOpen] = useState(!setup.config.settings?.panelChannelId)

  return (
    <TicketSetupContext.Provider value={setup}>
      <div className="guild-layout">
        <GuildSubnav guildId={guildId} guild={guild} />
        <div className="guild-page">
          <Outlet />
        </div>
      </div>
      <SetupTour setup={setup} open={tourOpen} onOpenChange={setTourOpen} />
    </TicketSetupContext.Provider>
  )
}
