import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, Outlet } from '@tanstack/react-router'
import { useState } from 'react'
import { AppShell } from '../components/AppShell'
import { GuildSubnav } from '../components/GuildSubnav'
import { SetupTour } from '../components/SetupTour'
import { requireUser } from '../lib/auth'
import { guildsQuery } from '../lib/guilds'
import { TicketSetupContext } from '../lib/ticketSetupContext'
import { ticketConfigQuery } from '../lib/tickets'
import { useTicketSetup } from '../lib/useTicketSetup'

export const Route = createFileRoute('/guilds/$guildId')({
  beforeLoad: ({ context }) => requireUser(context.queryClient),
  loader: ({ context, params }) =>
    Promise.all([
      context.queryClient.ensureQueryData(ticketConfigQuery(params.guildId)),
      context.queryClient.ensureQueryData(guildsQuery),
    ]),
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
      <GuildFrame guildId={guildId} />
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
