import { Link, useRouterState } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { guildIconUrl, type Guild } from '../lib/guilds'

type Props = {
  guildId: string
  guild?: Guild
}

// Sub-barra lateral de un servidor: nombre y accesos a sus secciones.
export function GuildSubnav({ guildId, guild }: Props) {
  const icon = guild ? guildIconUrl(guild) : null

  // El desplegable de Configuración empieza abierto si ya estamos dentro de ella.
  const inSettings = useRouterState({
    select: (state) => state.location.pathname.startsWith(`/guilds/${guildId}/settings`),
  })
  const [open, setOpen] = useState(inSettings)

  return (
    <nav className="subnav" aria-label="Secciones del servidor">
      <div className="subnav-title">
        {icon ? (
          <img src={icon} alt="" width={24} height={24} />
        ) : (
          <span className="subnav-fallback">{guild?.name.charAt(0).toUpperCase()}</span>
        )}
        <span>{guild?.name ?? 'Servidor'}</span>
      </div>

      <Link
        to="/guilds/$guildId"
        params={{ guildId }}
        activeOptions={{ exact: true }}
        className="subnav-link"
        activeProps={{ className: 'subnav-link active' }}
      >
        <Icon>
          <path d="M3 3h7v9H3zM14 3h7v5h-7zM14 12h7v9h-7zM3 16h7v5H3z" />
        </Icon>
        Dashboard
      </Link>
      <Link
        to="/guilds/$guildId/transcripts"
        params={{ guildId }}
        className="subnav-link"
        activeProps={{ className: 'subnav-link active' }}
      >
        <Icon>
          <path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9zM14 3v6h6M8 13h8M8 17h8" />
        </Icon>
        Transcripciones
      </Link>
      <div className="subnav-group">
        <button
          type="button"
          className={inSettings ? 'subnav-link subnav-toggle current' : 'subnav-link subnav-toggle'}
          aria-expanded={open}
          aria-controls={`${guildId}-settings-menu`}
          onClick={() => setOpen((o) => !o)}
        >
          <Icon>
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
          </Icon>
          <span className="subnav-toggle-label">Configuración</span>
          <svg
            className={open ? 'subnav-chevron open' : 'subnav-chevron'}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="m9 6 6 6-6 6" />
          </svg>
        </button>

        {open && (
          <div id={`${guildId}-settings-menu`} className="subnav-sub">
            <Link
              to="/guilds/$guildId/settings"
              params={{ guildId }}
              activeOptions={{ exact: true }}
              className="subnav-link"
              activeProps={{ className: 'subnav-link active' }}
            >
              General
            </Link>
            <Link
              to="/guilds/$guildId/settings/tickets"
              params={{ guildId }}
              className="subnav-link"
              activeProps={{ className: 'subnav-link active' }}
            >
              Tickets
            </Link>
            <Link
              to="/guilds/$guildId/settings/ai"
              params={{ guildId }}
              className="subnav-link"
              activeProps={{ className: 'subnav-link active' }}
            >
              Asistente de IA
            </Link>
          </div>
        )}
      </div>
    </nav>
  )
}

function Icon({ children }: { children: ReactNode }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  )
}
