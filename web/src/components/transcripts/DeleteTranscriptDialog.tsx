import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { deleteTranscript } from '../../lib/transcripts'
import { toast } from '../../lib/toast'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'

type Props = {
  guildId: string
  ticketId: number
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Confirmación para borrar a mano el contenido de un ticket antes de que caduque.
export function DeleteTranscriptDialog({ guildId, ticketId, open, onOpenChange }: Props) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const remove = useMutation({
    mutationFn: () => deleteTranscript(guildId, ticketId),
    onSuccess: async () => {
      // Primero se sale de la página del ticket; luego se limpia su caché.
      await navigate({ to: '/guilds/$guildId/transcripts', params: { guildId } })
      queryClient.removeQueries({ queryKey: ['guilds', guildId, 'transcript', String(ticketId)] })
      await queryClient.invalidateQueries({ queryKey: ['guilds', guildId, 'transcripts'] })
      toast('Transcripción eliminada')
    },
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Eliminar transcripción">
      <p>
        Se borrarán los mensajes y las respuestas del formulario de este ticket. No se puede
        deshacer.
      </p>
      {remove.isError && (
        <small className="error">No se pudo eliminar la transcripción. Inténtalo de nuevo.</small>
      )}
      <div className="tour-actions">
        <Button variant="outline" onClick={() => onOpenChange(false)} disabled={remove.isPending}>
          Cancelar
        </Button>
        <Button variant="danger" onClick={() => remove.mutate()} loading={remove.isPending}>
          {remove.isPending ? 'Eliminando' : 'Eliminar transcripción'}
        </Button>
      </div>
    </Dialog>
  )
}
