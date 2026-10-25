import { useMutation, useQueryClient } from '@tanstack/react-query'
import { aiQuery, createAlertsChannel, saveAiAlerts, type AiConnection } from '../lib/ai'
import { useSetup } from '../lib/ticketSetupContext'
import { MISSING_PERMISSIONS, ticketConfigQuery, type TicketConfig } from '../lib/tickets'
import { toast } from '../lib/toast'
import { SelectField } from './SelectField'
import { Button } from './ui/Button'

type Props = { guildId: string; connection: AiConnection }

// Canal privado del staff donde el asistente avisa de los tickets que pasa a una persona. Allí va
// el motivo; el usuario, en su ticket, solo ve un mensaje neutro.
export function AiAlerts({ guildId, connection }: Props) {
  const queryClient = useQueryClient()
  const { config } = useSetup()

  const save = useMutation({
    mutationFn: (channelId: string) => saveAiAlerts(guildId, channelId),
    onSuccess: (saved) => {
      queryClient.setQueryData(aiQuery(guildId).queryKey, saved)
      toast(saved.alertsChannelId ? 'Canal de avisos guardado' : 'Canal de avisos quitado')
    },
  })

  const create = useMutation({
    mutationFn: () => createAlertsChannel(guildId),
    onSuccess: async (option) => {
      // El canal nuevo ya está guardado: se añade a las opciones y se refresca la conexión.
      queryClient.setQueryData<TicketConfig>(
        ticketConfigQuery(guildId).queryKey,
        (old) => old && { ...old, textChannels: [...old.textChannels, option] },
      )
      await queryClient.invalidateQueries({ queryKey: aiQuery(guildId).queryKey })
      toast('Canal de avisos creado')
    },
  })

  // Los errores de permisos los explica el propio campo; los demás (por ejemplo, falta el rol
  // de staff) se muestran tal como los dio el servidor.
  const createError = create.error?.message === MISSING_PERMISSIONS ? create.error : null
  const otherError = create.isError && !createError ? create.error.message : save.isError ? save.error.message : null

  return (
    <section className="settings-section">
      <h2>Avisos al staff</h2>
      <p>
        Cuando el asistente pasa un ticket a una persona, el aviso con el motivo va a este canal. El
        usuario solo ve un mensaje neutro en su ticket. Sin canal, se menciona al rol en el ticket,
        sin el motivo.
      </p>

      <div className="form ai-behavior-form">
        <SelectField
          label="Canal de avisos"
          help="Elige un canal que solo vea el staff, o deja que el bot cree uno privado."
          value={connection.alertsChannelId}
          options={config.textChannels}
          onChange={(id) => save.mutate(id)}
          optionPrefix="# "
          createLabel="Crear canal privado"
          onCreate={() => create.mutate()}
          creating={create.isPending}
          createError={createError}
          missingPermissionsHint='El bot no tiene permisos. Dale "Gestionar canales" en Discord, o vuelve a invitarlo desde el panel.'
        />
        {connection.alertsChannelId && (
          <div className="form-actions">
            <Button variant="outline" onClick={() => save.mutate('')} loading={save.isPending}>
              Quitar canal
            </Button>
          </div>
        )}
        {otherError && <small className="error">{otherError}</small>}
      </div>
    </section>
  )
}
