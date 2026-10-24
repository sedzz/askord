import { MISSING_PERMISSIONS } from '../lib/tickets'
import type { TicketSetup } from '../lib/useTicketSetup'
import { SelectField } from './SelectField'

type Props = { setup: TicketSetup }

// Los tres campos de la configuración, usados tanto en la página como en el tour.

export function RoleField({ setup }: Props) {
  return (
    <SelectField
      label="1. Rol de staff"
      help="Las personas con este rol verán y atenderán todos los tickets."
      value={setup.staffRoleId}
      options={setup.config.roles}
      onChange={setup.setStaffRoleId}
      createLabel="Crear rol"
      onCreate={() => setup.makeRole.mutate()}
      creating={setup.makeRole.isPending}
      createError={setup.makeRole.error}
      missingPermissionsHint='El bot no tiene permisos. Dale "Gestionar roles" en Discord, o vuelve a invitarlo desde el panel.'
    />
  )
}

export function CategoryField({ setup }: Props) {
  return (
    <SelectField
      label="2. Categoría de tickets"
      help="Aquí se crearán los canales privados de cada ticket (ticket-0001, ticket-0002…)."
      value={setup.categoryId}
      options={setup.config.categories}
      onChange={setup.setCategoryId}
      createLabel="Crear categoría"
      onCreate={() => setup.makeCategory.mutate()}
      creating={setup.makeCategory.isPending}
      createError={setup.makeCategory.error}
      missingPermissionsHint='El bot no tiene permisos. Dale "Gestionar canales" en Discord, o vuelve a invitarlo desde el panel.'
    />
  )
}

// Con publishButton={false} no hay botón: quien lo use (el tour) publica por su cuenta.
export function PanelField({ setup, publishButton = true }: Props & { publishButton?: boolean }) {
  const { publish } = setup
  const published = Boolean(setup.config.settings?.panelChannelId)

  return (
    <>
      <SelectField
        label="3. Panel de tickets"
        help="Canal donde se publicará el mensaje con el botón para abrir tickets. Debe verlo todo el mundo, como #soporte."
        value={setup.channelId}
        options={setup.config.textChannels}
        onChange={setup.setChannelId}
        optionPrefix="# "
        createLabel="Crear canal"
        onCreate={() => setup.makeChannel.mutate()}
        creating={setup.makeChannel.isPending}
        createError={setup.makeChannel.error}
        missingPermissionsHint='El bot no tiene permisos. Dale "Gestionar canales" en Discord, o vuelve a invitarlo desde el panel.'
        action={
          publishButton
            ? {
                label: published ? 'Volver a publicar' : 'Publicar panel',
                onClick: () => publish.mutate(),
                disabled: !setup.channelId || publish.isPending,
              }
            : undefined
        }
      />
      {publishButton && publish.isSuccess && <small className="saved">Panel publicado</small>}
      {publish.isError && (
        <small className="error">
          {publish.error.message === MISSING_PERMISSIONS
            ? 'El bot no puede escribir en ese canal. Revisa sus permisos en Discord.'
            : 'No se pudo publicar el panel. Inténtalo de nuevo.'}
        </small>
      )}
    </>
  )
}
