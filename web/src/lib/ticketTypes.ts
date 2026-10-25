import { queryOptions } from '@tanstack/react-query'
import { failure, jsonHeaders as json } from './api'

// Pregunta del formulario que se muestra al abrir un ticket de un tipo.
export type Question = { label: string; required: boolean; long: boolean }

export const MAX_QUESTIONS = 5
export const MAX_QUESTION_LABEL = 45

export type TicketType = {
  id: number
  name: string
  emoji: string
  description: string
  // Vacíos: se usa la categoría y el rol de staff por defecto del servidor.
  categoryId: string
  staffRoleId: string
  position: number
  questions: Question[]
  // Asistente de IA: si atiende los tickets de este tipo, cómo, y cuándo los pasa a una persona.
  aiEnabled: boolean
  aiInstructions: string
  aiEscalation: string
}

export type TicketTypeInput = Omit<TicketType, 'id' | 'position'>

export const MAX_TICKET_TYPES = 25

export const ticketTypesQuery = (guildId: string) =>
  queryOptions({
    queryKey: ['guilds', guildId, 'ticket-types'],
    queryFn: async (): Promise<TicketType[]> => {
      const res = await fetch(`/api/guilds/${guildId}/ticket-types`)
      if (!res.ok) throw new Error(`GET ticket-types falló: ${res.status}`)
      return res.json()
    },
  })

// Los errores 4xx traen un texto pensado para mostrarse; los demás, un mensaje genérico.
export async function createTicketType(
  guildId: string,
  input: TicketTypeInput,
): Promise<TicketType> {
  const res = await fetch(`/api/guilds/${guildId}/ticket-types`, {
    method: 'POST',
    headers: json,
    body: JSON.stringify(input),
  })
  if (!res.ok) throw await failure(res)
  return res.json()
}

export async function updateTicketType(
  guildId: string,
  id: number,
  input: TicketTypeInput,
): Promise<TicketType> {
  const res = await fetch(`/api/guilds/${guildId}/ticket-types/${id}`, {
    method: 'PUT',
    headers: json,
    body: JSON.stringify(input),
  })
  if (!res.ok) throw await failure(res)
  return res.json()
}

export async function deleteTicketType(guildId: string, id: number): Promise<void> {
  const res = await fetch(`/api/guilds/${guildId}/ticket-types/${id}`, { method: 'DELETE' })
  if (!res.ok) throw await failure(res)
}

export async function reorderTicketTypes(guildId: string, ids: number[]): Promise<void> {
  const res = await fetch(`/api/guilds/${guildId}/ticket-types/order`, {
    method: 'PUT',
    headers: json,
    body: JSON.stringify({ ids }),
  })
  if (!res.ok) throw await failure(res)
}
