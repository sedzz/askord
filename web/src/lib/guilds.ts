import { queryOptions } from '@tanstack/react-query'

export type Guild = {
  id: string
  name: string
  icon: string | null
  owner: boolean
  botInstalled: boolean
  inviteUrl: string
}

async function fetchGuilds(): Promise<Guild[]> {
  const res = await fetch('/api/guilds')
  if (!res.ok) throw new Error(`GET /api/guilds falló: ${res.status}`)
  return res.json()
}

// Se vuelve a pedir al volver a la pestaña, para detectar el bot recién añadido.
export const guildsQuery = queryOptions({
  queryKey: ['guilds'],
  queryFn: fetchGuilds,
  refetchOnWindowFocus: true,
  staleTime: 0,
})

export function guildIconUrl(guild: Guild): string | null {
  if (!guild.icon) return null
  return `https://cdn.discordapp.com/icons/${guild.id}/${guild.icon}.png?size=64`
}

// Quita el bot del servidor y borra todos sus datos (configuración, tickets y transcripciones).
export async function removeGuild(guildId: string): Promise<void> {
  const res = await fetch(`/api/guilds/${guildId}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(`DELETE guild falló: ${res.status}`)
}
