import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { guildsQuery, removeGuild } from '../../lib/guilds'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'

type Props = {
  guildId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Confirmación para quitar el bot de un servidor y borrar todos sus datos.
export function RemoveBotDialog({ guildId, open, onOpenChange }: Props) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const remove = useMutation({
    mutationFn: () => removeGuild(guildId),
    onSuccess: async () => {
      // Primero se sale de la página de este servidor; luego se limpia su caché.
      await navigate({ to: '/' })
      queryClient.removeQueries({ queryKey: ['guilds', guildId] })
      await queryClient.invalidateQueries({ queryKey: guildsQuery.queryKey, exact: true })
    },
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Quitar el bot de este servidor">
      <p>
        Si quitas el bot, se <strong>borrarán todos los datos</strong> de este servidor: la
        configuración, los tickets y las transcripciones. No se puede deshacer.
      </p>
      <p>
        En Discord no se borrará nada más: la categoría, el rol y los canales siguen ahí, y solo se
        eliminará el panel publicado.
      </p>
      {remove.isError && (
        <small className="error">No se pudo quitar el bot. Inténtalo de nuevo.</small>
      )}
      <div className="tour-actions">
        <Button variant="outline" onClick={() => onOpenChange(false)} disabled={remove.isPending}>
          Cancelar
        </Button>
        <Button variant="danger" onClick={() => remove.mutate()} loading={remove.isPending}>
          {remove.isPending ? 'Quitando' : 'Quitar bot y borrar datos'}
        </Button>
      </div>
    </Dialog>
  )
}
