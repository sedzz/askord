import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  createChannel,
  MISSING_PERMISSIONS,
  publishPanel,
  ticketConfigQuery,
  type TicketConfig,
} from '../lib/tickets'
import { Select } from './Select'

type Props = {
  guildId: string
  channels: TicketConfig['textChannels']
  currentChannelId: string
}

// Paso 3: publica el mensaje con el botón "Crear ticket" en el canal elegido.
export function PanelSection({ guildId, channels, currentChannelId }: Props) {
  const queryClient = useQueryClient()
  const queryKey = ticketConfigQuery(guildId).queryKey
  const [channelId, setChannelId] = useState(currentChannelId)

  const publish = useMutation({
    mutationFn: () => publishPanel(guildId, channelId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey }),
  })

  const makeChannel = useMutation({
    mutationFn: () => createChannel(guildId),
    onSuccess: (option) => {
      queryClient.setQueryData<TicketConfig>(
        queryKey,
        (old) => old && { ...old, textChannels: [...old.textChannels, option] },
      )
      setChannelId(option.id)
    },
  })

  return (
    <section className="form panel">
      <Select
        label="3. Panel de tickets"
        help="Canal donde se publicará el mensaje con el botón para abrir tickets. Debe verlo todo el mundo, como #soporte."
        value={channelId}
        options={channels}
        onChange={setChannelId}
        optionPrefix="# "
        createLabel="Crear canal"
        onCreate={() => makeChannel.mutate()}
        creating={makeChannel.isPending}
        createError={makeChannel.error}
        missingPermissionsHint='El bot no tiene permisos. Dale "Gestionar canales" en Discord, o vuelve a invitarlo desde el panel.'
        action={{
          label: currentChannelId ? 'Volver a publicar' : 'Publicar panel',
          onClick: () => publish.mutate(),
          disabled: !channelId || publish.isPending,
        }}
      />
      {publish.isSuccess && <small className="saved">Panel publicado</small>}
      {publish.isError && (
        <small className="error">
          {publish.error.message === MISSING_PERMISSIONS
            ? 'El bot no puede escribir en ese canal. Revisa sus permisos en Discord.'
            : 'No se pudo publicar el panel. Inténtalo de nuevo.'}
        </small>
      )}
    </section>
  )
}
