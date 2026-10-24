import { queryOptions } from '@tanstack/react-query'

export type GuildOverview = {
  name: string
  icon: string
  memberCount: number
  onlineCount: number
  textChannels: number
  voiceChannels: number
  roles: number
  createdAt: string
}

export type TicketStats = {
  total: number
  open: number
  closed: number
  avgResolutionSeconds: number
}

export type SeriesPoint = { date: string; opened: number }

export type Overview = {
  // Null si Discord no respondió; las estadísticas de tickets se ven igualmente.
  guild: GuildOverview | null
  tickets: TicketStats
  series: SeriesPoint[]
}

export type Range = 7 | 30 | 90

export const overviewQuery = (guildId: string, days: Range) =>
  queryOptions({
    queryKey: ['guilds', guildId, 'overview', days],
    queryFn: async (): Promise<Overview> => {
      const res = await fetch(`/api/guilds/${guildId}/overview?days=${days}`)
      if (!res.ok) throw new Error(`GET overview falló: ${res.status}`)
      return res.json()
    },
    // El dashboard no cambia por segundos: al volver a él se ve al instante lo ya cargado.
    staleTime: 30_000,
  })
