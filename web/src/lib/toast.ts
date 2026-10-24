import { useSyncExternalStore } from 'react'

export type Toast = { id: number; message: string; tone: 'success' | 'error' }

// Notificaciones breves que se retiran solas. Un almacén mínimo, sin dependencias.
const VISIBLE_MS = 3500

let toasts: Toast[] = []
let nextId = 1
const listeners = new Set<() => void>()

function emit() {
  listeners.forEach((listener) => listener())
}

export function toast(message: string, tone: Toast['tone'] = 'success') {
  const id = nextId++
  toasts = [...toasts, { id, message, tone }]
  emit()
  setTimeout(() => dismissToast(id), VISIBLE_MS)
}

export function dismissToast(id: number) {
  toasts = toasts.filter((t) => t.id !== id)
  emit()
}

export function useToasts(): Toast[] {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    () => toasts,
  )
}
