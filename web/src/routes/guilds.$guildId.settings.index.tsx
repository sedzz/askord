import { createFileRoute } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { RemoveBotDialog } from '../components/RemoveBotDialog'
import { CategoryField, PanelField, RoleField } from '../components/TicketFields'
import { Button } from '../components/ui/Button'
import { toast } from '../lib/toast'
import { useSetup } from '../lib/ticketSetupContext'

export const Route = createFileRoute('/guilds/$guildId/settings/')({
  component: Settings,
})

function Settings() {
  const { guildId } = Route.useParams()
  const setup = useSetup()
  const { config, save } = setup
  const [removeOpen, setRemoveOpen] = useState(false)

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    save.mutate(undefined, { onSuccess: () => toast('Cambios guardados') })
  }

  return (
    <div className="settings-page">
      <h1>Configuración</h1>
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
    </div>
  )
}
