import { useQuery } from '@tanstack/react-query'
import { guildIconUrl, guildsQuery } from '../lib/guilds'
import { buttonClassName } from './ui/button-styles'
import './ui/button.css'
import { Dialog } from './ui/Dialog'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Servidores que administras y donde el bot aún no está, con su enlace de invitación.
export function AddServerDialog({ open, onOpenChange }: Props) {
  const { data: guilds, isPending, isError } = useQuery(guildsQuery)
  const available = guilds?.filter((g) => !g.botInstalled) ?? []

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Añadir servidor"
      description="Elige un servidor que administres. Se abrirá Discord para autorizar al bot."
    >
      {isPending ? (
        <p>Cargando servidores…</p>
      ) : isError ? (
        <p className="error">No se pudieron cargar los servidores.</p>
      ) : available.length === 0 ? (
        <p>
          No hay más servidores para añadir. Si acabas de crear uno, cierra sesión y vuelve a entrar
          para actualizar la lista.
        </p>
      ) : (
        <ul className="guilds">
          {available.map((g) => {
            const icon = guildIconUrl(g)
            return (
              <li key={g.id} className="guild">
                {icon ? (
                  <img src={icon} alt="" width={40} height={40} />
                ) : (
                  <span className="guild-fallback">{g.name.charAt(0).toUpperCase()}</span>
                )}
                <span className="guild-name">{g.name}</span>
                <a
                  className={buttonClassName()}
                  href={g.inviteUrl}
                  target="_blank"
                  rel="noreferrer"
                >
                  Añadir bot
                </a>
              </li>
            )
          })}
        </ul>
      )}
    </Dialog>
  )
}
