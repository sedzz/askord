import { createFileRoute, Link } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { AppShell } from '../components/AppShell'
import { RemoveBotDialog } from '../components/RemoveBotDialog'
import { SetupTour } from '../components/SetupTour'
import { CategoryField, PanelField, RoleField } from '../components/TicketFields'
import { Button } from '../components/ui/Button'
import { requireUser } from '../lib/auth'
import { guildsQuery } from '../lib/guilds'
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
  component: GuildPage,
})

function GuildPage() {
  const { guildId } = Route.useParams()
  // key: al cambiar de servidor se reinicia todo (formulario y diálogos abiertos).
  return (
    <AppShell key={guildId} activeGuildId={guildId}>
      <GuildSettings guildId={guildId} />
    </AppShell>
  )
}

function GuildSettings({ guildId }: { guildId: string }) {
  const setup = useTicketSetup(guildId)
  const { config, save } = setup

  // Mientras falte algo (servidor nuevo o a medias), el tour se abre solo y no se puede
  // cerrar hasta terminarlo. Se decide al entrar: publicar el panel no lo cierra a mitad.
  const [tourOpen, setTourOpen] = useState(!config.settings?.panelChannelId)
  const [removeOpen, setRemoveOpen] = useState(false)

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    save.mutate()
  }

  return (
    <div className="content-centered">
      <h1>Tickets</h1>
      <p>Sigue los tres pasos para que tus miembros puedan abrir tickets desde Discord.</p>

      <form className="form" onSubmit={onSubmit}>
        <RoleField setup={setup} />
        <CategoryField setup={setup} />

        <div className="form-actions">
          <Button
            type="submit"
            disabled={!setup.categoryId || !setup.staffRoleId}
            loading={save.isPending}
          >
            {save.isPending ? 'Guardando' : 'Guardar'}
          </Button>
          {save.isSuccess && <span className="saved">Guardado</span>}
          {save.isError && <span className="error">No se pudo guardar</span>}
        </div>
      </form>

      {config.settings ? (
        <section className="form panel">
          <PanelField setup={setup} />
        </section>
      ) : (
        <p className="hint">Guarda los pasos 1 y 2 para poder publicar el panel (paso 3).</p>
      )}

      <section className="danger-zone">
        <h2>Zona de peligro</h2>
        <p>Quita el bot de este servidor y borra todos sus datos, transcripciones incluidas.</p>
        <Button variant="danger" onClick={() => setRemoveOpen(true)}>
          Quitar bot del servidor
        </Button>
      </section>

      <RemoveBotDialog guildId={guildId} open={removeOpen} onOpenChange={setRemoveOpen} />
      <SetupTour setup={setup} open={tourOpen} onOpenChange={setTourOpen} />
    </div>
  )
}
