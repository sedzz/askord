import { createFileRoute } from '@tanstack/react-router'
import { TicketTypes } from '../components/TicketTypes'
import { useSetup } from '../lib/ticketSetupContext'

export const Route = createFileRoute('/guilds/$guildId/settings/tickets')({
  component: TicketSettings,
})

function TicketSettings() {
  const { guildId } = Route.useParams()
  const { config } = useSetup()

  return (
    <div>
      <h1>Tickets</h1>
      <p>Define qué tipos de ticket pueden abrir tus miembros y quién los atiende.</p>

      {config.settings ? (
        <TicketTypes guildId={guildId} />
      ) : (
        <p className="hint">
          Completa antes la configuración general (rol de staff y categoría) para poder crear tipos.
        </p>
      )}
    </div>
  )
}
