const date = new Intl.DateTimeFormat('es', { dateStyle: 'medium' })
const dateTime = new Intl.DateTimeFormat('es', { dateStyle: 'short', timeStyle: 'short' })

export const formatDate = (iso: string) => date.format(new Date(iso))
export const formatDateTime = (iso: string) => dateTime.format(new Date(iso))

// Número de ticket con ceros: 1 -> "0001".
export const ticketNumber = (n: number) => String(n).padStart(4, '0')

// Avatar de un usuario de Discord. Sin foto propia, Discord reparte 6 avatares por defecto
// según el ID (los 22 bits bajos no cuentan).
export function discordAvatarUrl(userId: string, avatarHash: string): string {
  if (avatarHash) return `https://cdn.discordapp.com/avatars/${userId}/${avatarHash}.png?size=64`
  const index = Number((BigInt(userId) >> 22n) % 6n)
  return `https://cdn.discordapp.com/embed/avatars/${index}.png`
}

// Duración legible: 4500 -> "1 h 15 min"; menos de un minuto -> "< 1 min".
export function formatDuration(seconds: number): string {
  const minutes = Math.round(seconds / 60)
  if (minutes < 1) return '< 1 min'
  if (minutes < 60) return `${minutes} min`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return minutes % 60 ? `${hours} h ${minutes % 60} min` : `${hours} h`
  const days = Math.floor(hours / 24)
  return hours % 24 ? `${days} d ${hours % 24} h` : `${days} d`
}

const number = new Intl.NumberFormat('es')
export const formatNumber = (n: number) => number.format(n)

// Fecha corta de un día "YYYY-MM-DD" (tal cual, sin desplazarla por la zona horaria).
const day = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short', timeZone: 'UTC' })
export const formatDay = (isoDay: string) => day.format(new Date(`${isoDay}T00:00:00Z`))
