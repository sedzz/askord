import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'

// Respuesta al formulario del tipo, con la pregunta tal como era.
export type Answer = { question: string; answer: string }

export type TranscriptSummary = {
  id: number
  number: number
  openerId: string
  openerName: string
  status: 'open' | 'closed'
  createdAt: string
  closedAt: string | null
  messages: number
  // Tipo del ticket (el actual o, si se borró, el que tenía al abrirse); vacío si no tiene.
  typeId: number
  typeName: string
  typeEmoji: string
  answers: Answer[]
  // Primer mensaje con texto de quien abrió el ticket, recortado.
  preview: string
}

export type TranscriptMessage = {
  id: string
  authorId: string
  authorName: string
  // Hash del avatar en Discord; vacío si usa el de por defecto.
  authorAvatar: string
  content: string
  attachments: string[]
  createdAt: string
}

export type Transcript = TranscriptSummary & { conversation: TranscriptMessage[] }

// Fechas en formato YYYY-MM-DD (ambas inclusivas) y usuario por nombre o ID.
export type TranscriptFilters = { from?: string; to?: string; user?: string; type?: string }

export type ExportFormat = 'csv' | 'json' | 'txt'

function filterParams(filters: TranscriptFilters): URLSearchParams {
  const params = new URLSearchParams()
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  if (filters.user) params.set('user', filters.user)
  if (filters.type) params.set('type', filters.type)
  return params
}

type Page = { items: TranscriptSummary[]; hasMore: boolean }

export const transcriptsQuery = (guildId: string, filters: TranscriptFilters) =>
  infiniteQueryOptions({
    queryKey: ['guilds', guildId, 'transcripts', filters],
    initialPageParam: 0,
    queryFn: async ({ pageParam }): Promise<Page> => {
      const params = filterParams(filters)
      params.set('offset', String(pageParam))
      const res = await fetch(`/api/guilds/${guildId}/transcripts?${params}`)
      if (!res.ok) throw new Error(`GET transcripts falló: ${res.status}`)
      return res.json()
    },
    // El siguiente desplazamiento es lo cargado hasta ahora.
    getNextPageParam: (last, pages) =>
      last.hasMore ? pages.reduce((n, p) => n + p.items.length, 0) : undefined,
  })

export const transcriptQuery = (guildId: string, ticketId: string) =>
  queryOptions({
    queryKey: ['guilds', guildId, 'transcript', ticketId],
    queryFn: async (): Promise<Transcript> => {
      const res = await fetch(`/api/guilds/${guildId}/transcripts/${ticketId}`)
      if (!res.ok) throw new Error(`GET transcript falló: ${res.status}`)
      return res.json()
    },
  })

// Enlaces de descarga: el navegador envía la cookie de sesión, no hace falta fetch.
export function exportUrl(guildId: string, filters: TranscriptFilters, format: ExportFormat) {
  const params = filterParams(filters)
  params.set('format', format)
  return `/api/guilds/${guildId}/transcripts/export?${params}`
}

export function ticketExportUrl(guildId: string, ticketId: number | string, format: ExportFormat) {
  return `/api/guilds/${guildId}/transcripts/${ticketId}/export?format=${format}`
}
