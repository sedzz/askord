import { queryOptions } from '@tanstack/react-query'
import { failure, jsonHeaders } from './api'

// Conexión del servidor con su LLM (cualquier API compatible con OpenAI). La API key no
// llega nunca al navegador: solo se sabe si hay una guardada.
export type AiConnection = {
  // false si este despliegue de Askord no tiene configurado el cifrado de claves.
  available: boolean
  baseUrl: string
  model: string
  hasKey: boolean
  // Interruptor general del asistente en el servidor y sus pautas (markdown o texto plano).
  enabled: boolean
  guidelines: string
}

export type AiInput = {
  baseUrl: string
  model: string
  // Vacía: se conserva la que ya había guardada.
  apiKey: string
}

export type AiBehaviorInput = { enabled: boolean; guidelines: string }

// Límites de caracteres de los textos de IA (iguales a los del servidor).
export const MAX_GUIDELINES = 4000
export const MAX_AI_INSTRUCTIONS = 2000
export const MAX_AI_ESCALATION = 1000

export type AiTestResult = { ok: boolean; message?: string }

export const aiQuery = (guildId: string) =>
  queryOptions({
    queryKey: ['guilds', guildId, 'ai'],
    queryFn: async (): Promise<AiConnection> => {
      const res = await fetch(`/api/guilds/${guildId}/ai`)
      if (!res.ok) throw new Error(`GET ai falló: ${res.status}`)
      return res.json()
    },
  })

export async function saveAi(guildId: string, input: AiInput): Promise<AiConnection> {
  const res = await fetch(`/api/guilds/${guildId}/ai`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(input),
  })
  if (!res.ok) throw await failure(res)
  return res.json()
}

export async function saveAiBehavior(
  guildId: string,
  input: AiBehaviorInput,
): Promise<AiConnection> {
  const res = await fetch(`/api/guilds/${guildId}/ai/behavior`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(input),
  })
  if (!res.ok) throw await failure(res)
  return res.json()
}

// Prueba la conexión sin guardarla. Un fallo de la conexión no es un error de la API:
// llega como { ok: false, message }.
export async function testAi(guildId: string, input: AiInput): Promise<AiTestResult> {
  const res = await fetch(`/api/guilds/${guildId}/ai/test`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(input),
  })
  if (!res.ok) throw await failure(res)
  return res.json()
}

export async function deleteAi(guildId: string): Promise<void> {
  const res = await fetch(`/api/guilds/${guildId}/ai`, { method: 'DELETE' })
  if (!res.ok) throw await failure(res)
}
