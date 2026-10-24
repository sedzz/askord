import type { ReactNode } from 'react'
import './discord.css'

// Piezas que imitan la interfaz de Discord para las vistas previas. Son maquetas:
// solo se dibujan, no hacen nada.

type FrameProps = {
  // "chat" es el fondo de los mensajes; "sidebar", el de la lista de canales.
  tone?: 'chat' | 'sidebar'
  // Cabecera con el nombre del canal, como en la parte alta de un chat.
  // badge: etiqueta a la derecha del nombre, p. ej. el tipo de ticket.
  channel?: { name: string; locked?: boolean; badge?: string }
  children: ReactNode
}

export function DiscordFrame({ tone = 'chat', channel, children }: FrameProps) {
  return (
    <div className={`dc dc-${tone}`} aria-hidden="true">
      {channel && (
        <div className="dc-header">
          <ChannelIcon locked={channel.locked} />
          <span>{channel.name}</span>
          {channel.badge && <span className="dc-badge">{channel.badge}</span>}
        </div>
      )}
      <div className="dc-body">{children}</div>
    </div>
  )
}

type MessageProps = {
  author: string
  time?: string
  // Muestra la etiqueta APP, propia de los bots.
  app?: boolean
  // Imagen del avatar; sin ella se usa la inicial del autor.
  avatarUrl?: string
  children: ReactNode
}

export function DiscordMessage({
  author,
  time = 'Hoy a las 19:22',
  app,
  avatarUrl,
  children,
}: MessageProps) {
  return (
    <div className="dc-message">
      {avatarUrl ? (
        <img className="dc-avatar" src={avatarUrl} alt="" />
      ) : (
        <div className="dc-avatar">{author.charAt(0).toUpperCase()}</div>
      )}
      <div className="dc-message-main">
        <div className="dc-meta">
          <span className="dc-author">{author}</span>
          {app && (
            <span className="dc-app">
              <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path
                  d="m3.5 8.5 3 3 6-6.5"
                  stroke="currentColor"
                  strokeWidth="3"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              APP
            </span>
          )}
          <span className="dc-time">{time}</span>
        </div>
        <div className="dc-content">{children}</div>
      </div>
    </div>
  )
}

export function DiscordEmbed({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="dc-embed">
      <div className="dc-embed-title">{title}</div>
      <div className="dc-embed-text">{children}</div>
    </div>
  )
}

type ButtonProps = {
  variant?: 'primary' | 'secondary'
  emoji?: string
  children: ReactNode
}

export function DiscordButton({ variant = 'primary', emoji, children }: ButtonProps) {
  return (
    <span className={`dc-button dc-button-${variant}`}>
      {emoji && <span aria-hidden="true">{emoji}</span>}
      {children}
    </span>
  )
}

export function DiscordMention({ children }: { children: ReactNode }) {
  return <span className="dc-mention">{children}</span>
}

export function DiscordCategory({ name, children }: { name: string; children: ReactNode }) {
  return (
    <div className="dc-category">
      <div className="dc-category-name">
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path
            d="m6 9 6 6 6-6"
            stroke="currentColor"
            strokeWidth="3"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        <span>{name}</span>
      </div>
      {children}
    </div>
  )
}

export function DiscordChannel({ name, locked }: { name: string; locked?: boolean }) {
  return (
    <div className="dc-channel">
      <ChannelIcon locked={locked} />
      <span>{name}</span>
    </div>
  )
}

// Almohadilla de canal de texto, con candado si es privado.
function ChannelIcon({ locked }: { locked?: boolean }) {
  return (
    <svg className="dc-channel-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d={locked ? 'M3 8h11M2 13h11M7 3 5 18M12 3l-2 15' : 'M4 9h16M3 15h16M9 3 7 21M15 3l-2 18'}
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
      {locked && (
        <>
          <rect x="14" y="13" width="8" height="7" rx="1.5" fill="currentColor" />
          <path
            d="M16 13v-1.5a2 2 0 0 1 4 0V13"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
          />
        </>
      )}
    </svg>
  )
}
