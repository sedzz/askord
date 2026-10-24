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
