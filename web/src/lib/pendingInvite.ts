import { useSyncExternalStore } from 'react'

// Servidor al que el usuario acaba de pulsar "Añadir bot" y que estamos esperando que
// aparezca como instalado. Mientras exista, la app consulta con frecuencia y, cuando el
// bot entra, lleva al usuario a ese servidor.
let pending: string | null = null
let timer: ReturnType<typeof setTimeout> | undefined
const listeners = new Set<() => void>()

// Si el usuario no llega a autorizar, se deja de esperar pasado un rato.
const GIVE_UP_AFTER_MS = 5 * 60 * 1000

export function setPendingInvite(guildId: string | null) {
  pending = guildId
  clearTimeout(timer)
  if (guildId) timer = setTimeout(() => setPendingInvite(null), GIVE_UP_AFTER_MS)
  listeners.forEach((listener) => listener())
}

export function usePendingInvite(): string | null {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    () => pending,
  )
}
