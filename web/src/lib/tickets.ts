import { queryOptions } from '@tanstack/react-query'

export type Option = { id: string; name: string }

export type TicketSettings = { categoryId: string; staffRoleId: string }

export type TicketConfig = {
  settings: (TicketSettings & { panelChannelId: string }) | null
  categories: Option[]
  roles: Option[]
  textChannels: Option[]
}

export const ticketConfigQuery = (guildId: string) =>
  queryOptions({
    queryKey: ['guilds', guildId, 'tickets'],
    queryFn: async (): Promise<TicketConfig> => {
      const res = await fetch(`/api/guilds/${guildId}/tickets`)
      if (!res.ok) throw new Error(`GET tickets falló: ${res.status}`)
      return res.json()
    },
    staleTime: 0,
  })

export async function saveTicketSettings(
  guildId: string,
  settings: TicketSettings,
): Promise<TicketSettings> {
  const res = await fetch(`/api/guilds/${guildId}/tickets`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  if (!res.ok) throw new Error(`PUT tickets falló: ${res.status}`)
  return res.json()
}

export const MISSING_PERMISSIONS = 'missing-permissions'

async function createOption(
  guildId: string,
  kind: 'category' | 'role' | 'channel',
): Promise<Option> {
  const res = await fetch(`/api/guilds/${guildId}/tickets/${kind}`, { method: 'POST' })
  if (res.status === 403) throw new Error(MISSING_PERMISSIONS)
  if (!res.ok) throw new Error(`POST ${kind} falló: ${res.status}`)
  return res.json()
}

export const createCategory = (guildId: string) => createOption(guildId, 'category')
export const createRole = (guildId: string) => createOption(guildId, 'role')

export async function publishPanel(guildId: string, channelId: string): Promise<void> {
  const res = await fetch(`/api/guilds/${guildId}/tickets/panel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ channelId }),
  })
  if (res.status === 403) throw new Error(MISSING_PERMISSIONS)
  if (!res.ok) throw new Error(`POST panel falló: ${res.status}`)
}
export const createChannel = (guildId: string) => createOption(guildId, 'channel')
