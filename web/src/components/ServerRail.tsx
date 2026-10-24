import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { guildIconUrl, guildsQuery } from '../lib/guilds'

// Barra lateral con los servidores donde está el bot, al estilo de Discord.
// Sin ninguno, no se muestra: el inicio ofrece entonces añadir el primero.
export function ServerRail({ activeGuildId }: { activeGuildId?: string }) {
  const { data: guilds } = useQuery(guildsQuery)
  const installed = guilds?.filter((g) => g.botInstalled) ?? []
  if (installed.length === 0) return null

  return (
    <nav className="rail" aria-label="Servidores">
      {installed.map((g) => {
        const icon = guildIconUrl(g)
        return (
          <Link
            key={g.id}
            to="/guilds/$guildId"
            params={{ guildId: g.id }}
            className={g.id === activeGuildId ? 'rail-item active' : 'rail-item'}
            title={g.name}
          >
            {icon ? <img src={icon} alt={g.name} /> : g.name.charAt(0).toUpperCase()}
          </Link>
        )
      })}
    </nav>
  )
}
