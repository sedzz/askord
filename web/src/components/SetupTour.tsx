import { useState } from 'react'
import type { TicketSetup } from '../lib/useTicketSetup'
import { CategoryField, PanelField, RoleField } from './TicketFields'
import { CategoryPreview, PanelPreview, WelcomePreview } from './TicketPreviews'
import { Button } from './ui/Button'
import { Dialog } from './ui/Dialog'
import { Stepper } from './ui/Stepper'

const WELCOME = 0
const ROLE = 1
const CATEGORY = 2
const PANEL = 3
const DONE = 4
const TOTAL_STEPS = 5

type Props = {
  setup: TicketSetup
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Configuración guiada de tickets, paso a paso, en un diálogo.
export function SetupTour({ setup, open, onOpenChange }: Props) {
  const { config, save } = setup
  // Si ya hay algo guardado (se recargó a mitad), se retoma en el panel.
  const [step, setStep] = useState(config.settings ? PANEL : WELCOME)

  const hasRole = setup.staffRoleId !== ''
  const hasCategory = setup.categoryId !== ''

  const canContinue =
    step === ROLE
      ? hasRole
      : step === CATEGORY
        ? hasRole && hasCategory && !save.isPending
        : step === PANEL
          ? setup.channelId !== '' && !setup.publish.isPending
          : true

  function next() {
    if (step === CATEGORY) {
      // La configuración se guarda al salir de este paso.
      save.mutate(undefined, { onSuccess: () => setStep(PANEL) })
      return
    }
    if (step === PANEL) {
      // Finalizar publica el panel en el canal elegido y pasa al resumen.
      setup.publish.mutate(undefined, { onSuccess: () => setStep(DONE) })
      return
    }
    setStep(step + 1)
  }

  const nextLabel =
    step === WELCOME
      ? 'Empezar'
      : step === PANEL
        ? setup.publish.isPending
          ? 'Publicando…'
          : 'Finalizar'
        : save.isPending
          ? 'Guardando…'
          : 'Siguiente'

  const roleName = config.roles.find((r) => r.id === setup.staffRoleId)?.name
  const categoryName = config.categories.find((c) => c.id === setup.categoryId)?.name
  const channelName = config.textChannels.find(
    (c) => c.id === config.settings?.panelChannelId,
  )?.name
  // En la vista previa se usa el canal que se está eligiendo, aún sin publicar.
  const selectedChannelName = config.textChannels.find((c) => c.id === setup.channelId)?.name

  return (
    // No se puede cerrar hasta terminar: la configuración es obligatoria.
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Configura los tickets"
      size="lg"
      dismissible={false}
    >
      <Stepper current={step} total={TOTAL_STEPS} />

      <div className="tour-step">
        {step === WELCOME && (
          <>
            <h3>Vamos a dejar los tickets listos</h3>
            <p>
              En unos pasos tus miembros podrán abrir tickets privados desde Discord. Elegiremos
              quién los atiende, dónde se crean y dónde se publica el botón para abrirlos. Si falta
              algo, lo creamos por ti.
            </p>
          </>
        )}
        {step === ROLE && (
          <>
            <RoleField setup={setup} />
            <WelcomePreview roleName={roleName} />
          </>
        )}
        {step === CATEGORY && (
          <>
            <CategoryField setup={setup} />
            <CategoryPreview categoryName={categoryName} />
          </>
        )}
        {step === PANEL && (
          <>
            <PanelField setup={setup} publishButton={false} />
            <PanelPreview channelName={selectedChannelName} />
          </>
        )}
        {step === DONE && (
          <>
            <h3>¡Todo listo!</h3>
            <p>Tus miembros ya pueden abrir tickets desde el panel.</p>
            <dl className="tour-summary">
              <dt>Rol de staff</dt>
              <dd>{roleName ?? '—'}</dd>
              <dt>Categoría</dt>
              <dd>{categoryName ?? '—'}</dd>
              <dt>Panel</dt>
              <dd>{channelName ? `# ${channelName}` : '—'}</dd>
            </dl>
          </>
        )}
        {save.isError && step === CATEGORY && (
          <small className="error">No se pudo guardar. Inténtalo de nuevo.</small>
        )}
      </div>

      <div className="tour-actions">
        <Button
          variant="outline"
          onClick={() => setStep(step - 1)}
          disabled={step === WELCOME || step === DONE}
        >
          Atrás
        </Button>
        {step === DONE ? (
          <Button onClick={() => onOpenChange(false)}>Terminar</Button>
        ) : (
          <Button onClick={next} disabled={!canContinue}>
            {nextLabel}
          </Button>
        )}
      </div>
    </Dialog>
  )
}
